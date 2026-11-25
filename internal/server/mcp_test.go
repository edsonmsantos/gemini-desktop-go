package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"geminidesktop-go/internal/mcp"
)

type mcpListResponse struct {
	Servers   []mcp.Server `json:"servers"`
	Available bool         `json:"available"`
}

type mcpToggleResponse struct {
	Name   string      `json:"name"`
	Status string      `json:"status"`
	Server *mcp.Server `json:"server,omitempty"`
}

func createFakeAgyScript(t *testing.T) (string, *string) {
	t.Helper()
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "fake-agy")
	stateFile := filepath.Join(tmpDir, "status.txt")
	_ = os.WriteFile(stateFile, []byte("enabled"), 0644)

	script := `#!/bin/sh
STATE_FILE="` + stateFile + `"
STATUS=$(cat "$STATE_FILE" 2>/dev/null || echo "enabled")

if [ "$1" = "mcp" ] && [ "$2" = "list" ]; then
  cat << EOF
NAME            TYPE   STATUS   COMMAND/URL
alpha           stdio  $STATUS  /bin/alpha-run
EOF
  exit 0
fi

if [ "$1" = "mcp" ] && [ "$2" = "enable" ]; then
  echo "enabled" > "$STATE_FILE"
  exit 0
fi

if [ "$1" = "mcp" ] && [ "$2" = "disable" ]; then
  echo "disabled" > "$STATE_FILE"
  exit 0
fi

exit 1
`
	if err := os.WriteFile(fakeBin, []byte(script), 0755); err != nil {
		t.Fatalf("failed to write fake-agy script: %v", err)
	}

	return fakeBin, &stateFile
}

func TestServer_MCP_GetListAndCache(t *testing.T) {
	fakeBin, stateFile := createFakeAgyScript(t)
	srv := NewServer(nil, t.TempDir(), nil)
	srv.SetMCPClient(mcp.NewClient(fakeBin))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET inicial
	res, err := http.Get(ts.URL + "/api/mcp")
	if err != nil {
		t.Fatalf("GET /api/mcp failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var data mcpListResponse
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if !data.Available {
		t.Fatal("expected available true")
	}
	if len(data.Servers) != 1 || data.Servers[0].Name != "alpha" || data.Servers[0].Status != "enabled" {
		t.Fatalf("unexpected servers: %+v", data.Servers)
	}

	// 2. Modificar o arquivo de estado em disco diretamente (simulando mudança externa)
	_ = os.WriteFile(*stateFile, []byte("disabled"), 0644)

	// 3. GET imediato deve vir do cache de 5s (ainda com enabled)
	res2, err := http.Get(ts.URL + "/api/mcp")
	if err != nil {
		t.Fatalf("second GET failed: %v", err)
	}
	var data2 mcpListResponse
	_ = json.NewDecoder(res2.Body).Decode(&data2)
	res2.Body.Close()

	if len(data2.Servers) != 1 || data2.Servers[0].Status != "enabled" {
		t.Errorf("expected cached status enabled, got %s", data2.Servers[0].Status)
	}
}

func TestServer_MCP_ToggleAndCacheInvalidation(t *testing.T) {
	fakeBin, _ := createFakeAgyScript(t)
	srv := NewServer(nil, t.TempDir(), nil)
	srv.SetMCPClient(mcp.NewClient(fakeBin))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Fazer toggle para disable
	toggleBody, _ := json.Marshal(map[string]bool{"enable": false})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/mcp/alpha/toggle", bytes.NewReader(toggleBody))
	req.Header.Set("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST toggle failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on toggle, got %d", res.StatusCode)
	}

	var toggleResp mcpToggleResponse
	if err := json.NewDecoder(res.Body).Decode(&toggleResp); err != nil {
		t.Fatalf("failed to decode toggle resp: %v", err)
	}

	if toggleResp.Status != "disabled" {
		t.Errorf("expected toggle resp status disabled, got %s", toggleResp.Status)
	}

	// 2. O GET subsequente deve refletir disabled imediatamente (cache invalidado pelo toggle)
	resList, err := http.Get(ts.URL + "/api/mcp")
	if err != nil {
		t.Fatalf("GET after toggle failed: %v", err)
	}
	var listResp mcpListResponse
	_ = json.NewDecoder(resList.Body).Decode(&listResp)
	resList.Body.Close()

	if len(listResp.Servers) != 1 || listResp.Servers[0].Status != "disabled" {
		t.Errorf("expected status disabled after toggle, got: %+v", listResp.Servers)
	}
}

func TestServer_MCP_Validation(t *testing.T) {
	srv := NewServer(nil, t.TempDir(), nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Rejeita "../x"
	body, _ := json.Marshal(map[string]bool{"enable": true})
	req1, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/mcp/..%2Fx/toggle", bytes.NewReader(body))
	res1, err := http.DefaultClient.Do(req1)
	if err != nil {
		t.Fatalf("request 1 failed: %v", err)
	}
	res1.Body.Close()
	if res1.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for path traversal name, got %d", res1.StatusCode)
	}

	// 2. Rejeita "a b"
	req2, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/mcp/a%20b/toggle", bytes.NewReader(body))
	res2, err := http.DefaultClient.Do(req2)
	if err != nil {
		t.Fatalf("request 2 failed: %v", err)
	}
	res2.Body.Close()
	if res2.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400 for space in name, got %d", res2.StatusCode)
	}
}

func TestServer_MCP_Unavailable(t *testing.T) {
	srv := NewServer(nil, t.TempDir(), nil)
	srv.SetMCPClient(mcp.NewClient("/caminho/nao/existe/agy-bin-12345"))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET retorna available: false
	res, err := http.Get(ts.URL + "/api/mcp")
	if err != nil {
		t.Fatalf("GET /api/mcp failed: %v", err)
	}
	defer res.Body.Close()

	var data mcpListResponse
	_ = json.NewDecoder(res.Body).Decode(&data)
	if data.Available {
		t.Errorf("expected available false when binary is missing")
	}

	// 2. POST toggle responde 503
	body, _ := json.Marshal(map[string]bool{"enable": true})
	req, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/mcp/some-tool/toggle", bytes.NewReader(body))
	resToggle, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("toggle failed: %v", err)
	}
	resToggle.Body.Close()

	if resToggle.StatusCode != http.StatusServiceUnavailable {
		t.Errorf("expected 503 when agy is unavailable, got %d", resToggle.StatusCode)
	}
}
