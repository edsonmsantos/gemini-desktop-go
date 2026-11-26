package bridge

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geminidesktop-go/internal/config"
)

func getFreePort(t *testing.T) int {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen on ephemeral port: %v", err)
	}
	defer ln.Close()
	return ln.Addr().(*net.TCPAddr).Port
}

func TestStartInProcess_Disabled(t *testing.T) {
	disabled := false
	cfg := &config.Config{
		BrowserBridgeInProcess: &disabled,
	}

	stop, err := StartInProcess(cfg)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if stop != nil {
		t.Fatalf("expected nil stop func when disabled")
	}
}

func TestStartInProcess_PortBusy(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("BROWSER_BRIDGE_DIR", tmpDir)

	port := getFreePort(t)

	// Ocupa a porta com um listener TCP fake
	ln, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		t.Fatalf("falha ao ocupar porta %d: %v", port, err)
	}
	defer ln.Close()

	cfg := &config.Config{
		BrowserBridgePort: port,
	}

	stop, err := StartInProcess(cfg)
	if err != nil {
		t.Fatalf("expected nil err when port is busy (graceful fallback), got %v", err)
	}
	if stop != nil {
		defer stop()
		t.Fatalf("expected nil stop func when port is busy")
	}
}

func TestStartInProcess_FreePort(t *testing.T) {
	tmpDir := t.TempDir()
	t.Setenv("BROWSER_BRIDGE_DIR", tmpDir)

	port := getFreePort(t)
	cfg := &config.Config{
		BrowserBridgePort: port,
	}

	stop, err := StartInProcess(cfg)
	if err != nil {
		t.Fatalf("expected nil err, got %v", err)
	}
	if stop == nil {
		t.Fatalf("expected non-nil stop func on free port")
	}
	defer stop()

	// Lê o token gerado em tmpDir
	tokenBytes, err := os.ReadFile(filepath.Join(tmpDir, "token.txt"))
	if err != nil {
		t.Fatalf("falha ao ler token.txt gerado: %v", err)
	}
	token := strings.TrimSpace(string(tokenBytes))

	// Faz requisição para GET /api/status no daemon in-process
	req, err := http.NewRequest("GET", fmt.Sprintf("http://127.0.0.1:%d/api/status", port), nil)
	if err != nil {
		t.Fatalf("falha ao criar request: %v", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("falha ao consultar /api/status: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperado status 200, obteve %d", resp.StatusCode)
	}

	var statusResp map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		t.Fatalf("falha ao decodificar JSON do /api/status: %v", err)
	}

	if statusResp["version"] == nil {
		t.Errorf("esperado campo 'version' no status, obteve %v", statusResp)
	}
}
