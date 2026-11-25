package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/store"

	"github.com/gorilla/websocket"
)

func TestServer_AgentDieMidTurnAndRespawn(t *testing.T) {
	agentBin := findFakeAgent(t)
	logDir := t.TempDir()

	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agentBin,
		LogDir:     logDir,
	})
	if err != nil {
		t.Fatalf("falha ao instanciar client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no initialize inicial: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "resilience_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(client, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Conecta WebSocket em /api/stream
	u, _ := url.Parse(ts.URL)
	wsURL := "ws://" + u.Host + "/api/stream"
	ws, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("falha ao conectar no websocket: %v", err)
	}
	defer ws.Close()

	evCh := make(chan acp.Event, 20)
	wsDone := make(chan struct{})
	go func() {
		defer close(wsDone)
		for {
			var ev acp.Event
			if err := ws.ReadJSON(&ev); err != nil {
				return
			}
			evCh <- ev
		}
	}()

	// 2. Criar conversa
	createBody, _ := json.Marshal(map[string]string{
		"cwd": t.TempDir(),
	})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 201/200, obteve %d", resp.StatusCode)
	}
	var convRes struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionId"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&convRes)
	resp.Body.Close()

	// 3. Primeiro prompt: simula falha fatal die-mid-turn
	promptBody, _ := json.Marshal(map[string]interface{}{
		"text": "test-die-mid-turn",
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+convRes.ID+"/prompt", "application/json", bytes.NewReader(promptBody))
	if err != nil {
		t.Fatalf("falha no 1o prompt: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 no disparo do prompt, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Aguarda evento de erro no stream
	receivedThought := false
	receivedError := false
	var errMsg string

	timeout := time.After(5 * time.Second)
	for !receivedError {
		select {
		case ev := <-evCh:
			if ev.Type == "thought_chunk" {
				receivedThought = true
			}
			if ev.Type == "error" && ev.Error != nil {
				receivedError = true
				errMsg = ev.Error.Msg
			}
		case <-timeout:
			t.Fatalf("timeout aguardando evento de erro do agente morto (thought=%v, error=%v)", receivedThought, receivedError)
		}
	}

	if !receivedThought {
		t.Logf("aviso: thought_chunk não capturado antes da morte do processo")
	}

	if !strings.Contains(errMsg, "agente encerrou inesperadamente") {
		t.Fatalf("esperava mensagem contendo 'agente encerrou inesperadamente', obteve: %q", errMsg)
	}

	// 4. O app e o servidor DEVEM continuar vivos
	healthResp, err := http.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("servidor HTTP caiu após morte do agente: %v", err)
	}
	if healthResp.StatusCode != http.StatusOK {
		t.Fatalf("servidor HTTP retornou status inesperado após morte do agente: %d", healthResp.StatusCode)
	}
	healthResp.Body.Close()

	// 5. Segundo prompt na MESMA conversa: DEVE respawnar automaticamente e completar o turno
	secondPromptBody, _ := json.Marshal(map[string]interface{}{
		"text": "segundo prompt após recuperação",
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+convRes.ID+"/prompt", "application/json", bytes.NewReader(secondPromptBody))
	if err != nil {
		t.Fatalf("falha no 2o prompt: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 no 2o prompt com respawn, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Aguarda turn_done no 2o prompt
	receivedTurnDone := false
	receivedMessage := false
	timeout2 := time.After(5 * time.Second)

	for !receivedTurnDone {
		select {
		case ev := <-evCh:
			if ev.Type == "message_chunk" {
				receivedMessage = true
			}
			if ev.Type == "turn_done" {
				receivedTurnDone = true
			}
		case <-timeout2:
			t.Fatalf("timeout aguardando turn_done no 2o prompt após respawn (msg=%v, done=%v)", receivedMessage, receivedTurnDone)
		}
	}

	if !receivedMessage {
		t.Fatalf("esperava receber message_chunk no 2o prompt após respawn")
	}

	// 6. Verifica se o servidor ainda está de pé e saudável
	healthResp2, err := http.Get(ts.URL + "/api/health")
	if err != nil || healthResp2.StatusCode != http.StatusOK {
		t.Fatalf("servidor HTTP não sobreviveu ao 2o turno")
	}
	healthResp2.Body.Close()
}
