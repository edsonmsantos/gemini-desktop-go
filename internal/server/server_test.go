package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"testing"
	"time"

	"geminidesktop-go/internal/acp"

	"github.com/gorilla/websocket"
)

func findFakeAgent(t *testing.T) string {
	t.Helper()
	p := filepath.Join("..", "..", "testdata", "fake-acp-agent", "fake-agent")
	abs, err := filepath.Abs(p)
	if err != nil {
		t.Fatalf("erro ao resolver caminho fake-agent: %v", err)
	}
	if _, err := os.Stat(abs); err != nil {
		t.Fatalf("fake-agent não encontrado em %s", abs)
	}
	return abs
}

func TestServer_HealthAndStreamPrompt(t *testing.T) {
	agentBin := findFakeAgent(t)
	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agentBin,
		LogDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("falha ao instanciar client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no initialize: %v", err)
	}

	srv := NewServer(client, t.TempDir(), nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET /api/health
	res, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("falha ao chamar /api/health: %v", err)
	}
	if res.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 em /api/health, obteve %d", res.StatusCode)
	}

	// 2. Conectar WebSocket em /api/stream
	u, _ := url.Parse(ts.URL)
	wsURL := "ws://" + u.Host + "/api/stream"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("falha ao conectar WS: %v", err)
	}
	defer wsConn.Close()

	// 3. POST /api/session
	sessReqBody := []byte(`{"cwd":"/tmp"}`)
	resp, err := http.Post(ts.URL+"/api/session", "application/json", bytes.NewReader(sessReqBody))
	if err != nil {
		t.Fatalf("falha ao chamar /api/session: %v", err)
	}
	var sessResp struct {
		SessionID string `json:"sessionId"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&sessResp)
	resp.Body.Close()

	if sessResp.SessionID == "" {
		t.Fatalf("sessionId retornado vazio em /api/session")
	}

	// 4. POST /api/prompt
	promptReqBody := []byte(`{"text":"Olá fake agent"}`)
	resp, err = http.Post(ts.URL+"/api/prompt", "application/json", bytes.NewReader(promptReqBody))
	if err != nil {
		t.Fatalf("falha ao chamar /api/prompt: %v", err)
	}
	resp.Body.Close()

	// 5. Receber frames WS
	var receivedMessageChunk, receivedTurnDone bool
	deadline := time.Now().Add(5 * time.Second)
	_ = wsConn.SetReadDeadline(deadline)

	for !receivedTurnDone {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			t.Fatalf("erro ao ler frame WS: %v", err)
		}
		var ev acp.Event
		if err := json.Unmarshal(msg, &ev); err != nil {
			t.Fatalf("frame WS inválido: %v (corpo: %s)", err, string(msg))
		}
		if ev.Type == "message_chunk" {
			receivedMessageChunk = true
		}
		if ev.Type == "turn_done" {
			receivedTurnDone = true
		}
	}

	if !receivedMessageChunk {
		t.Fatalf("esperava ao menos um message_chunk no stream WS")
	}

	// 6. Testar POST /api/permission
	permReqBody := []byte(`{"callId":"tc-test","allow":true}`)
	resp, err = http.Post(ts.URL+"/api/permission", "application/json", bytes.NewReader(permReqBody))
	if err != nil {
		t.Fatalf("falha ao chamar /api/permission: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 em /api/permission, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 7. Testar POST /api/cancel
	cancelReqBody := []byte(`{"sessionId":"` + sessResp.SessionID + `"}`)
	resp, err = http.Post(ts.URL+"/api/cancel", "application/json", bytes.NewReader(cancelReqBody))
	if err != nil {
		t.Fatalf("falha ao chamar /api/cancel: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 em /api/cancel, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()
}
