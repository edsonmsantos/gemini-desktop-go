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

func setupV2TestServerWithInstance(t *testing.T) (*httptest.Server, *acp.Client, *store.Store, *Server) {
	t.Helper()
	agentBin := findFakeAgent(t)
	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agentBin,
		LogDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("falha ao instanciar client: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no initialize: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "v2_pipeline_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}

	srv := NewServerV2(client, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	return ts, client, st, srv
}

func TestServer_ToolsCamel_TwoDistinctCalls(t *testing.T) {
	ts, client, st, _ := setupV2TestServerWithInstance(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	// 1. Criar conversa
	createBody, _ := json.Marshal(map[string]string{
		"cwd":     t.TempDir(),
		"modelId": "gemini-2.5-flash",
	})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	var createdConv struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&createdConv)
	resp.Body.Close()

	// 2. Conectar WebSocket
	u, _ := url.Parse(ts.URL)
	wsURL := "ws://" + u.Host + "/api/stream"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("falha ao conectar WS: %v", err)
	}
	defer wsConn.Close()

	// 3. Enviar prompt com "test-tools-camel"
	promptBody, _ := json.Marshal(map[string]string{
		"text": "Executar teste com test-tools-camel",
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+createdConv.ID+"/prompt", "application/json", bytes.NewReader(promptBody))
	if err != nil {
		t.Fatalf("falha ao enviar prompt: %v", err)
	}
	resp.Body.Close()

	// 4. Aguardar turn_done no WebSocket
	_ = wsConn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			t.Fatalf("erro ao ler WS frame: %v", err)
		}
		var frame struct {
			Type string `json:"type"`
		}
		_ = json.Unmarshal(msg, &frame)
		if frame.Type == "turn_done" {
			break
		}
	}

	// 5. Verificar mensagens no store
	msgs, err := st.GetMessages(createdConv.ID)
	if err != nil {
		t.Fatalf("falha ao buscar mensagens: %v", err)
	}

	var toolCalls []*store.Message
	for _, m := range msgs {
		if m.Role == "tool_call" {
			toolCalls = append(toolCalls, m)
		}
	}

	// Duas chamadas com toolCallId distintos produzem DUAS mensagens
	if len(toolCalls) != 2 {
		t.Fatalf("esperava 2 mensagens role=tool_call, obteve %d", len(toolCalls))
	}

	var meta1, meta2 map[string]interface{}
	_ = json.Unmarshal([]byte(toolCalls[0].Meta), &meta1)
	_ = json.Unmarshal([]byte(toolCalls[1].Meta), &meta2)

	if meta1["toolCallId"] != "call_7530034839743884041" {
		t.Fatalf("tool 1 esperava toolCallId 'call_7530034839743884041', obteve %v", meta1["toolCallId"])
	}
	if meta1["status"] != "completed" {
		t.Fatalf("tool 1 esperava status 'completed', obteve %v", meta1["status"])
	}
	if meta1["name"] != "call_mcp_tool" {
		t.Fatalf("tool 1 esperava name 'call_mcp_tool', obteve %v", meta1["name"])
	}
	if meta1["rawOutput"] != "modo: agente (porta 9223)" {
		t.Fatalf("tool 1 esperava rawOutput 'modo: agente (porta 9223)', obteve %v", meta1["rawOutput"])
	}

	if meta2["toolCallId"] != "call_7530034839743884042" {
		t.Fatalf("tool 2 esperava toolCallId 'call_7530034839743884042', obteve %v", meta2["toolCallId"])
	}
	if meta2["status"] != "failed" {
		t.Fatalf("tool 2 esperava status 'failed', obteve %v", meta2["status"])
	}
	if meta2["name"] != "call_mcp_tool" {
		t.Fatalf("tool 2 esperava name 'call_mcp_tool', obteve %v", meta2["name"])
	}
	if meta2["rawOutput"] != "elemento #btn não encontrado" {
		t.Fatalf("tool 2 esperava rawOutput 'elemento #btn não encontrado', obteve %v", meta2["rawOutput"])
	}
}

func TestServer_TurnTimeout_Configurable(t *testing.T) {
	ts, client, st, srv := setupV2TestServerWithInstance(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	// Configura timeout curto de 300ms
	srv.SetTurnTimeout(300 * time.Millisecond)

	// 1. Criar conversa
	createBody, _ := json.Marshal(map[string]string{
		"cwd":     t.TempDir(),
		"modelId": "gemini-2.5-flash",
	})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	var createdConv struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&createdConv)
	resp.Body.Close()

	// 2. Conectar WebSocket
	u, _ := url.Parse(ts.URL)
	wsURL := "ws://" + u.Host + "/api/stream"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("falha ao conectar WS: %v", err)
	}
	defer wsConn.Close()

	// 3. Enviar prompt com "test-slow" (que demoraria 10s no fake agent)
	promptBody, _ := json.Marshal(map[string]string{
		"text": "Executar tarefa com test-slow",
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+createdConv.ID+"/prompt", "application/json", bytes.NewReader(promptBody))
	if err != nil {
		t.Fatalf("falha ao enviar prompt: %v", err)
	}
	resp.Body.Close()

	// 4. Receber evento de erro com "tempo expirou" no WebSocket
	_ = wsConn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var errorEventReceived bool
	var errorMsg string
	for {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			break
		}
		var frame struct {
			Type  string `json:"type"`
			Error *struct {
				Msg string `json:"msg"`
			} `json:"error"`
		}
		_ = json.Unmarshal(msg, &frame)
		if frame.Type == "error" && frame.Error != nil {
			errorEventReceived = true
			errorMsg = frame.Error.Msg
			break
		}
	}

	if !errorEventReceived {
		t.Fatalf("esperava evento de erro via WS após expiração do timeout")
	}
	if !strings.Contains(errorMsg, "tempo expirou") {
		t.Fatalf("esperava que erro contivesse 'tempo expirou', obteve %q", errorMsg)
	}

	// 5. O cliente continua vivo
	if !client.IsAlive() {
		t.Fatalf("esperava que cliente continuasse vivo após timeout de turno")
	}

	// 6. Mensagem de erro foi persistida no store
	msgs, err := st.GetMessages(createdConv.ID)
	if err != nil {
		t.Fatalf("falha ao buscar mensagens: %v", err)
	}
	var foundErrorInStore bool
	for _, m := range msgs {
		if m.Role == "error" && strings.Contains(m.Content, "tempo expirou") {
			foundErrorInStore = true
			break
		}
	}
	if !foundErrorInStore {
		t.Fatalf("mensagem de erro 'tempo expirou' não foi encontrada no store")
	}

	// 7. Teste de SetTurnTimeout(0) (sem limite): campo reflete 0
	srv.SetTurnTimeout(0)
	srv.mu.Lock()
	timeoutVal := srv.turnTimeout
	srv.mu.Unlock()
	if timeoutVal != 0 {
		t.Fatalf("esperava turnTimeout == 0 após SetTurnTimeout(0), obteve %v", timeoutVal)
	}
}
