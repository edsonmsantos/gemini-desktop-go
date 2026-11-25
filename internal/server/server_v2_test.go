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
	"strings"
	"testing"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/store"

	"github.com/gorilla/websocket"
)

func setupV2TestServer(t *testing.T) (*httptest.Server, *acp.Client, *store.Store) {
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

	dbPath := filepath.Join(t.TempDir(), "v2_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}

	srv := NewServerV2(client, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	return ts, client, st
}

func TestServerV2_ConversationsLifecycle(t *testing.T) {
	ts, client, st := setupV2TestServer(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	cwd := t.TempDir()

	// 1. POST /api/conversations
	createBody, _ := json.Marshal(map[string]string{
		"cwd":     cwd,
		"modelId": "gemini-2.5-flash",
	})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("falha no POST /api/conversations: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("status inesperado em /api/conversations: %d", resp.StatusCode)
	}
	var createdConv struct {
		ID        string `json:"id"`
		Title     string `json:"title"`
		SessionID string `json:"sessionId"`
		CWD       string `json:"cwd"`
		ModelID   string `json:"modelId"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&createdConv)
	resp.Body.Close()

	if createdConv.ID == "" || createdConv.SessionID == "" {
		t.Fatalf("conversa criada inválida: %+v", createdConv)
	}

	// 2. GET /api/conversations
	resp, err = http.Get(ts.URL + "/api/conversations")
	if err != nil {
		t.Fatalf("falha no GET /api/conversations: %v", err)
	}
	var list []struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&list)
	resp.Body.Close()
	if len(list) != 1 || list[0].ID != createdConv.ID {
		t.Fatalf("lista de conversas incorreta: %+v", list)
	}

	// 3. Conectar WebSocket em /api/stream
	u, _ := url.Parse(ts.URL)
	wsURL := "ws://" + u.Host + "/api/stream"
	wsConn, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("falha ao conectar WS: %v", err)
	}
	defer wsConn.Close()

	// 4. POST /api/conversations/{id}/prompt
	promptBody, _ := json.Marshal(map[string]string{
		"text": "Olá Gemini V2",
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+createdConv.ID+"/prompt", "application/json", bytes.NewReader(promptBody))
	if err != nil {
		t.Fatalf("falha ao enviar prompt: %v", err)
	}
	resp.Body.Close()

	// 5. Receber frames WS contendo conversationId
	var receivedMessageChunk, receivedTurnDone bool
	deadline := time.Now().Add(5 * time.Second)
	_ = wsConn.SetReadDeadline(deadline)

	for !receivedTurnDone {
		_, msg, err := wsConn.ReadMessage()
		if err != nil {
			t.Fatalf("erro ao ler WS frame: %v", err)
		}
		var frame struct {
			Type           string `json:"type"`
			ConversationID string `json:"conversationId"`
		}
		_ = json.Unmarshal(msg, &frame)
		if frame.ConversationID == createdConv.ID {
			if frame.Type == "message_chunk" {
				receivedMessageChunk = true
			}
			if frame.Type == "turn_done" {
				receivedTurnDone = true
			}
		}
	}
	if !receivedMessageChunk {
		t.Fatalf("esperava message_chunk com conversationId no WS")
	}

	// 6. GET /api/conversations/{id} — histórico persistido
	resp, err = http.Get(ts.URL + "/api/conversations/" + createdConv.ID)
	if err != nil {
		t.Fatalf("falha no GET histórico: %v", err)
	}
	var convDetail struct {
		Conversation struct {
			ID    string `json:"id"`
			Title string `json:"title"`
		} `json:"conversation"`
		Messages []struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"messages"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&convDetail)
	resp.Body.Close()

	if len(convDetail.Messages) < 2 {
		t.Fatalf("esperava pelo menos 2 mensagens (user e assistant), obteve %d", len(convDetail.Messages))
	}
	// Título default derivado da 1ª mensagem do usuário
	if convDetail.Conversation.Title == "" {
		t.Fatalf("esperava título gerado automaticamente")
	}

	// 7. PATCH título
	patchTitleBody, _ := json.Marshal(map[string]string{"title": "Título Customizado"})
	req, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/conversations/"+createdConv.ID, bytes.NewReader(patchTitleBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("falha no PATCH título: %v (code %d)", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 8. GET /api/models
	resp, err = http.Get(ts.URL + "/api/models")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("falha no GET /api/models: %v", err)
	}
	var models []map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&models)
	resp.Body.Close()
	if len(models) == 0 {
		t.Fatalf("esperava modelos retornados em /api/models")
	}

	// 9. GET /api/workspaces
	resp, err = http.Get(ts.URL + "/api/workspaces")
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("falha no GET /api/workspaces: %v", err)
	}
	var wsList []map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&wsList)
	resp.Body.Close()
	if len(wsList) == 0 {
		t.Fatalf("esperava workspace registrado em /api/workspaces")
	}

	// 10. POST /api/conversations/{id}/resume
	resp, err = http.Post(ts.URL+"/api/conversations/"+createdConv.ID+"/resume", "application/json", nil)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("falha no resume da sessão: %v (code %d)", err, resp.StatusCode)
	}
	resp.Body.Close()

	// 11. DELETE /api/conversations/{id}
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/conversations/"+createdConv.ID, nil)
	resp, err = http.DefaultClient.Do(delReq)
	if err != nil || resp.StatusCode != http.StatusOK {
		t.Fatalf("falha no DELETE /api/conversations/{id}: %v", err)
	}
	resp.Body.Close()
}

// TestServerV2_AttachmentAndRegenerate cobre o upload de anexos (print colado/
// drag & drop) e o fluxo de regenerar/editar o último turno.
func TestServerV2_AttachmentAndRegenerate(t *testing.T) {
	ts, client, st := setupV2TestServer(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	cwd := t.TempDir()

	// 1. Upload de anexo (base64 válido)
	b64 := "aGVsbG8gd29ybGQ=" // "hello world"
	body, _ := json.Marshal(map[string]string{"filename": "anexo teste.txt", "dataBase64": b64})
	resp, err := http.Post(ts.URL+"/api/attachments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("falha no upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 no upload, obteve %d", resp.StatusCode)
	}
	var up struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&up); err != nil {
		t.Fatalf("falha ao decodificar resposta do upload: %v", err)
	}
	if up.Path == "" {
		t.Fatal("esperava path no resposta do upload")
	}
	data, err := os.ReadFile(up.Path)
	if err != nil || string(data) != "hello world" {
		t.Fatalf("conteúdo salvo incorreto: %v %q", err, string(data))
	}

	// 2. Upload com base64 inválido deve dar 400
	badBody, _ := json.Marshal(map[string]string{"filename": "x.png", "dataBase64": "###"})
	resp2, err := http.Post(ts.URL+"/api/attachments", "application/json", bytes.NewReader(badBody))
	if err != nil {
		t.Fatalf("falha no upload inválido: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para base64 inválido, obteve %d", resp2.StatusCode)
	}

	// 3. Conversa + prompt com o fake agent
	createBody, _ := json.Marshal(map[string]string{"cwd": cwd})
	resp3, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	defer resp3.Body.Close()
	var conv struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(resp3.Body).Decode(&conv); err != nil {
		t.Fatalf("falha ao decodificar conversa: %v", err)
	}

	// Prompt 1 com anexo do upload (exercita meta de arquivos)
	promptBody, _ := json.Marshal(map[string]interface{}{"text": "primeira pergunta", "files": []string{up.Path}})
	resp4, err := http.Post(ts.URL+"/api/conversations/"+conv.ID+"/prompt", "application/json", bytes.NewReader(promptBody))
	if err != nil {
		t.Fatalf("falha no prompt: %v", err)
	}
	resp4.Body.Close()

	// Aguarda o turno do fake agent terminar
	deadline := time.Now().Add(5 * time.Second)
	var msgs []*store.Message
	for time.Now().Before(deadline) {
		got, err := st.GetMessages(conv.ID)
		if err != nil {
			t.Fatalf("falha ao ler mensagens: %v", err)
		}
		done := false
		for _, m := range got {
			if m.Role == "assistant" && strings.Contains(m.Content, "fake") {
				done = true
			}
		}
		if done {
			msgs = got
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if len(msgs) == 0 {
		t.Fatal("turno do fake agent não produziu resposta a tempo")
	}
	if len(msgs) < 2 {
		t.Fatalf("esperava >=2 mensagens (user+assistant), obteve %d", len(msgs))
	}

	// 4. Regenerar: trunca o último turno e reenvia
	resp5, err := http.Post(ts.URL+"/api/conversations/"+conv.ID+"/regenerate", "application/json", bytes.NewReader([]byte("{}")))
	if err != nil {
		t.Fatalf("falha no regenerate: %v", err)
	}
	resp5.Body.Close()
	if resp5.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 no regenerate, obteve %d", resp5.StatusCode)
	}

	// Aguarda o novo turno terminar e valida a estrutura
	deadline = time.Now().Add(5 * time.Second)
	var finalMsgs []*store.Message
	for time.Now().Before(deadline) {
		got, err := st.GetMessages(conv.ID)
		if err != nil {
			t.Fatalf("falha ao ler mensagens pós-regenerate: %v", err)
		}
		assistantCount := 0
		for _, m := range got {
			if m.Role == "assistant" {
				assistantCount++
			}
		}
		if assistantCount >= 1 && len(got) >= 2 && got[0].Role == "user" {
			time.Sleep(800 * time.Millisecond) // assenta o restante do turno (plan/tool_call)
			finalMsgs, _ = st.GetMessages(conv.ID)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if finalMsgs == nil {
		got, _ := st.GetMessages(conv.ID)
		for _, m := range got {
			t.Logf("pós-regenerate: seq=%d role=%s content=%.60q meta=%.60q", m.Seq, m.Role, m.Content, m.Meta)
		}
		t.Fatal("regenerate não produziu novo turno a tempo")
	}
	// A primeira mensagem deve ser o prompt do usuário reenviado
	if finalMsgs[0].Role != "user" || finalMsgs[0].Content != "primeira pergunta" {
		t.Fatalf("prompt do usuário não foi reposto corretamente: %+v", finalMsgs[0])
	}

	// 5. Regenerar com edição de texto
	resp6, err := http.Post(ts.URL+"/api/conversations/"+conv.ID+"/regenerate", "application/json", bytes.NewReader([]byte(`{"text":"pergunta editada"}`)))
	if err != nil {
		t.Fatalf("falha no regenerate com edição: %v", err)
	}
	resp6.Body.Close()
	if resp6.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 no regenerate com edição, obteve %d", resp6.StatusCode)
	}

	deadline = time.Now().Add(5 * time.Second)
	var editedMsgs []*store.Message
	for time.Now().Before(deadline) {
		got, err := st.GetMessages(conv.ID)
		if err != nil {
			t.Fatalf("falha ao ler mensagens pós-edição: %v", err)
		}
		if len(got) >= 2 && got[0].Role == "user" && got[0].Content == "pergunta editada" {
			time.Sleep(800 * time.Millisecond)
			editedMsgs, _ = st.GetMessages(conv.ID)
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
	if editedMsgs == nil {
		t.Fatal("edição não produziu novo turno a tempo")
	}
	if editedMsgs[0].Content != "pergunta editada" {
		t.Fatalf("esperava texto editado, obteve %q", editedMsgs[0].Content)
	}
}
