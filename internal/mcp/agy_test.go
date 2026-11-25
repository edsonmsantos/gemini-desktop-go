package mcp_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"geminidesktop-go/internal/mcp"
)

const realHostOutputFixture = `NAME            TYPE   STATUS   COMMAND/URL
blender         stdio  enabled  /home/edson/.local/bin/blender-mcp
browser-bridge  stdio  enabled  node /home/edson/Documentos/MiniProjetos/browser-bridge/dist/index.js
gsc             stdio  enabled  npx -y suganthan-gsc-mcp
llm-wiki        stdio  enabled  /home/edson/Documentos/GitHub/llm-wik-2/.venv/bin/python /home/edson/Documentos/GitHub/llm-wik-2/mcp_server.py
reddit          stdio  enabled  npx -y reddit-mcp-server
sicap           stdio  enabled  python3 /home/edson/.gemini/config/sicap_mcp_bridge.py
`

func TestParseListOutput_RealFixture(t *testing.T) {
	servers := mcp.ParseListOutput(realHostOutputFixture)
	if len(servers) != 6 {
		t.Fatalf("expected 6 servers, got %d", len(servers))
	}

	expected := []struct {
		name    string
		mcpType string
		status  string
		command string
	}{
		{"blender", "stdio", "enabled", "/home/edson/.local/bin/blender-mcp"},
		{"browser-bridge", "stdio", "enabled", "node /home/edson/Documentos/MiniProjetos/browser-bridge/dist/index.js"},
		{"gsc", "stdio", "enabled", "npx -y suganthan-gsc-mcp"},
		{"llm-wiki", "stdio", "enabled", "/home/edson/Documentos/GitHub/llm-wik-2/.venv/bin/python /home/edson/Documentos/GitHub/llm-wik-2/mcp_server.py"},
		{"reddit", "stdio", "enabled", "npx -y reddit-mcp-server"},
		{"sicap", "stdio", "enabled", "python3 /home/edson/.gemini/config/sicap_mcp_bridge.py"},
	}

	for i, exp := range expected {
		s := servers[i]
		if s.Name != exp.name {
			t.Errorf("[%d] expected name %q, got %q", i, exp.name, s.Name)
		}
		if s.Type != exp.mcpType {
			t.Errorf("[%d] expected type %q, got %q", i, exp.mcpType, s.Type)
		}
		if s.Status != exp.status {
			t.Errorf("[%d] expected status %q, got %q", i, exp.status, s.Status)
		}
		if s.Command != exp.command {
			t.Errorf("[%d] expected command %q, got %q", i, exp.command, s.Command)
		}
	}
}

func TestParseListOutput_OtherAndMalformed(t *testing.T) {
	raw := `
NAME            TYPE   STATUS   COMMAND/URL
alpha           stdio  disabled /bin/alpha-cmd
beta            sse    paused   http://localhost:8080/sse
malformed-line-too-few
gamma           stdio  unknown  /bin/gamma-tool with multiple args

`
	servers := mcp.ParseListOutput(raw)
	if len(servers) != 3 {
		t.Fatalf("expected 3 valid servers, got %d", len(servers))
	}

	// alpha: disabled
	if servers[0].Name != "alpha" || servers[0].Status != "disabled" || servers[0].Type != "stdio" {
		t.Errorf("unexpected alpha: %+v", servers[0])
	}
	// beta: other
	if servers[1].Name != "beta" || servers[1].Status != "other" || servers[1].Type != "sse" {
		t.Errorf("unexpected beta: %+v", servers[1])
	}
	// gamma: other
	if servers[2].Name != "gamma" || servers[2].Status != "other" || servers[2].Command != "/bin/gamma-tool with multiple args" {
		t.Errorf("unexpected gamma: %+v", servers[2])
	}
}

func TestClient_WithFakeBinary(t *testing.T) {
	tmpDir := t.TempDir()
	fakeBin := filepath.Join(tmpDir, "fake-agy")

	fakeScript := `#!/bin/sh
if [ "$1" = "mcp" ] && [ "$2" = "list" ]; then
  cat << 'EOF'
NAME            TYPE   STATUS   COMMAND/URL
fake-tool       stdio  enabled  /bin/fake-run
EOF
  exit 0
fi

if [ "$1" = "mcp" ] && [ "$2" = "enable" ] && [ -n "$3" ]; then
  exit 0
fi

if [ "$1" = "mcp" ] && [ "$2" = "disable" ] && [ -n "$3" ]; then
  exit 0
fi

exit 1
`
	if err := os.WriteFile(fakeBin, []byte(fakeScript), 0755); err != nil {
		t.Fatalf("failed to write fake-agy: %v", err)
	}

	client := mcp.NewClient(fakeBin)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// 1. List
	list, err := client.List(ctx)
	if err != nil {
		t.Fatalf("List failed: %v", err)
	}
	if len(list) != 1 || list[0].Name != "fake-tool" || list[0].Status != "enabled" {
		t.Fatalf("unexpected list: %+v", list)
	}

	// 2. Enable
	if err := client.Enable(ctx, "fake-tool"); err != nil {
		t.Fatalf("Enable failed: %v", err)
	}

	// 3. Disable
	if err := client.Disable(ctx, "fake-tool"); err != nil {
		t.Fatalf("Disable failed: %v", err)
	}
}

func TestClient_UnavailableBinary(t *testing.T) {
	client := mcp.NewClient("/non/existent/agy/path/binary")
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	_, err := client.List(ctx)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !errors.Is(err, mcp.ErrMCPUnavailable) {
		t.Fatalf("expected ErrMCPUnavailable, got: %v", err)
	}

	err = client.Enable(ctx, "something")
	if !errors.Is(err, mcp.ErrMCPUnavailable) {
		t.Fatalf("expected ErrMCPUnavailable on Enable, got: %v", err)
	}

	fmt.Println("Unavailable tested successfully")
}
