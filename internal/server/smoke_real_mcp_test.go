package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"testing"

	"geminidesktop-go/internal/mcp"
)

func TestSmoke_RealMCP(t *testing.T) {
	if testing.Short() {
		t.Skip("pulando teste smoke real com agy em modo short")
	}

	_, err := exec.LookPath("agy")
	if err != nil {
		t.Skip("binário agy não encontrado no PATH")
	}

	srv := NewServer(nil, t.TempDir(), nil)
	// Garante que usa o agy real do host
	srv.SetMCPClient(mcp.NewClient("agy"))

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET /api/mcp retorna os 6 servidores reais
	res, err := http.Get(ts.URL + "/api/mcp")
	if err != nil {
		t.Fatalf("GET /api/mcp failed: %v", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.StatusCode)
	}

	var data struct {
		Servers   []mcp.Server `json:"servers"`
		Available bool         `json:"available"`
	}
	if err := json.NewDecoder(res.Body).Decode(&data); err != nil {
		t.Fatalf("failed to decode json: %v", err)
	}

	if !data.Available {
		t.Fatalf("expected available true for real agy")
	}

	expectedNames := []string{"blender", "browser-bridge", "gsc", "llm-wiki", "reddit", "sicap"}
	serversMap := make(map[string]mcp.Server)
	for _, s := range data.Servers {
		serversMap[s.Name] = s
	}

	for _, name := range expectedNames {
		s, found := serversMap[name]
		if !found {
			t.Errorf("server %q not found in list %+v", name, data.Servers)
			continue
		}
		if s.Type == "" {
			t.Errorf("server %q has empty type", name)
		}
		if s.Status == "" {
			t.Errorf("server %q has empty status", name)
		}
		if s.Command == "" {
			t.Errorf("server %q has empty command", name)
		}
	}

	// 2. POST /api/mcp/gsc/toggle disable -> enable (restaurando original)
	originalGSC, ok := serversMap["gsc"]
	if !ok {
		t.Fatalf("gsc not found for toggle test")
	}

	// Toggle para disable
	toggleBody, _ := json.Marshal(map[string]bool{"enable": false})
	reqDis, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/mcp/gsc/toggle", bytes.NewReader(toggleBody))
	reqDis.Header.Set("Content-Type", "application/json")
	resDis, err := http.DefaultClient.Do(reqDis)
	if err != nil {
		t.Fatalf("POST toggle disable failed: %v", err)
	}
	resDis.Body.Close()
	if resDis.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on toggle disable, got %d", resDis.StatusCode)
	}

	// Valida no GET subsequente
	resAfterDis, err := http.Get(ts.URL + "/api/mcp")
	if err != nil {
		t.Fatalf("GET after disable failed: %v", err)
	}
	var dataAfterDis struct {
		Servers []mcp.Server `json:"servers"`
	}
	_ = json.NewDecoder(resAfterDis.Body).Decode(&dataAfterDis)
	resAfterDis.Body.Close()

	for _, s := range dataAfterDis.Servers {
		if s.Name == "gsc" {
			if s.Status != "disabled" {
				t.Fatalf("expected gsc status disabled after toggle, got %s", s.Status)
			}
			break
		}
	}

	// Restaura para o estado original (enable)
	origEnable := originalGSC.Status == "enabled"
	restoreBody, _ := json.Marshal(map[string]bool{"enable": origEnable})
	reqRes, _ := http.NewRequest(http.MethodPost, ts.URL+"/api/mcp/gsc/toggle", bytes.NewReader(restoreBody))
	reqRes.Header.Set("Content-Type", "application/json")
	resRes, err := http.DefaultClient.Do(reqRes)
	if err != nil {
		t.Fatalf("POST toggle restore failed: %v", err)
	}
	resRes.Body.Close()
	if resRes.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 on toggle restore, got %d", resRes.StatusCode)
	}

	// Valida restauração no GET subsequente
	resFinal, err := http.Get(ts.URL + "/api/mcp")
	if err != nil {
		t.Fatalf("GET final failed: %v", err)
	}
	var dataFinal struct {
		Servers []mcp.Server `json:"servers"`
	}
	_ = json.NewDecoder(resFinal.Body).Decode(&dataFinal)
	resFinal.Body.Close()

	for _, s := range dataFinal.Servers {
		if s.Name == "gsc" {
			if s.Status != originalGSC.Status {
				t.Fatalf("expected gsc restored to %s, got %s", originalGSC.Status, s.Status)
			}
			break
		}
	}
}
