package server

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/store"
)

func setupCapsTestServer(t *testing.T) (*Server, *httptest.Server, *acp.Client, *store.Store) {
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

	dbPath := filepath.Join(t.TempDir(), "caps_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}

	srv := NewServerV2(client, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	return srv, ts, client, st
}

func TestServer_CapabilitiesEndpoint(t *testing.T) {
	srv, ts, client, st := setupCapsTestServer(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	// 1. 404 para conversa inexistente
	resp, err := http.Get(ts.URL + "/api/conversations/conv-inexistente/capabilities")
	if err != nil {
		t.Fatalf("falha na requisicao: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("esperava 404 para conversa inexistente, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 2. Cria conversa com fake agent (anuncia modes default e accept-edits, e tier Google AI Pro)
	body, _ := json.Marshal(map[string]string{
		"cwd": t.TempDir(),
	})
	resp, err = http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperava 201 ao criar conversa, obteve %d", resp.StatusCode)
	}
	var created struct {
		ID        string `json:"id"`
		SessionID string `json:"sessionId"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// 3. GET /api/conversations/{id}/capabilities
	resp, err = http.Get(ts.URL + "/api/conversations/" + created.ID + "/capabilities")
	if err != nil {
		t.Fatalf("falha ao consultar capabilities: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 em capabilities, obteve %d", resp.StatusCode)
	}
	var caps struct {
		Modes *struct {
			CurrentModeID  string `json:"currentModeId"`
			Current        string `json:"current"`
			AvailableModes []struct {
				ID          string `json:"id"`
				Name        string `json:"name"`
				Description string `json:"description"`
			} `json:"availableModes"`
		} `json:"modes"`
		Tier *struct {
			Name string `json:"name"`
			ID   string `json:"id"`
		} `json:"tier"`
		ConfigOptions []map[string]interface{} `json:"configOptions"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&caps); err != nil {
		t.Fatalf("falha ao decodificar capabilities: %v", err)
	}
	resp.Body.Close()

	if caps.Modes == nil || caps.Modes.CurrentModeID != "default" {
		t.Fatalf("modes incorretos: %+v", caps.Modes)
	}
	if len(caps.Modes.AvailableModes) < 2 {
		t.Fatalf("esperava pelo menos 2 modos disponíveis, obteve %d", len(caps.Modes.AvailableModes))
	}
	if caps.Tier == nil || caps.Tier.Name != "Google AI Pro" {
		t.Fatalf("esperava tier 'Google AI Pro', obteve %+v", caps.Tier)
	}

	// 4. Marca sessão como órfã -> deve retornar 409
	srv.convMu.Lock()
	srv.orphanSessions[created.SessionID] = true
	srv.convMu.Unlock()

	resp, err = http.Get(ts.URL + "/api/conversations/" + created.ID + "/capabilities")
	if err != nil {
		t.Fatalf("falha ao consultar capabilities de sessao orfa: %v", err)
	}
	if resp.StatusCode != http.StatusConflict {
		t.Fatalf("esperava 409 para sessao orfa, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()
}

func TestServer_ModeEndpoint_SuccessAndNotSupported(t *testing.T) {
	_, ts, client, st := setupCapsTestServer(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	// 1. Cria conversa
	body, _ := json.Marshal(map[string]string{
		"cwd": t.TempDir(),
	})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	var created struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	// 2. Altera para modo 'accept-edits'
	modeBody, _ := json.Marshal(map[string]string{
		"modeId": "accept-edits",
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+created.ID+"/mode", "application/json", bytes.NewReader(modeBody))
	if err != nil {
		t.Fatalf("falha ao trocar modo: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 ao alterar modo, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verifica persistência no store
	convInStore, err := st.GetConversation(created.ID)
	if err != nil {
		t.Fatalf("falha ao buscar conversa: %v", err)
	}
	if convInStore.ModeID != "accept-edits" {
		t.Fatalf("esperava modeId 'accept-edits' no store, obteve %q", convInStore.ModeID)
	}

	// Verifica se GET capabilities agora reflete o novo modo
	resp, err = http.Get(ts.URL + "/api/conversations/" + created.ID + "/capabilities")
	if err != nil {
		t.Fatalf("falha ao buscar capabilities: %v", err)
	}
	var caps struct {
		Modes struct {
			CurrentModeID string `json:"currentModeId"`
		} `json:"modes"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&caps)
	resp.Body.Close()
	if caps.Modes.CurrentModeID != "accept-edits" {
		t.Fatalf("esperava modo atual 'accept-edits' em capabilities, obteve %q", caps.Modes.CurrentModeID)
	}

	// 3. Testa modo não suportado (-32601) via fake agent trigger
	mode32601, _ := json.Marshal(map[string]string{
		"modeId": "trigger-32601",
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+created.ID+"/mode", "application/json", bytes.NewReader(mode32601))
	if err != nil {
		t.Fatalf("falha ao trocar para modo que dispara 32601: %v", err)
	}
	if resp.StatusCode != http.StatusNotImplemented {
		t.Fatalf("esperava 501 Not Implemented, obteve %d", resp.StatusCode)
	}
	var errResp map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&errResp)
	resp.Body.Close()
	if errResp["error"] != "agente não suporta troca de modo" {
		t.Fatalf("mensagem de erro inesperada: %+v", errResp)
	}
}

func TestServer_CreateConversation_WithModeId(t *testing.T) {
	_, ts, client, st := setupCapsTestServer(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	body, _ := json.Marshal(map[string]string{
		"cwd":    t.TempDir(),
		"modeId": "accept-edits",
	})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("esperava 201, obteve %d", resp.StatusCode)
	}
	var created struct {
		ID     string `json:"id"`
		ModeID string `json:"modeId"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&created)
	resp.Body.Close()

	if created.ModeID != "accept-edits" {
		t.Fatalf("esperava modeId 'accept-edits' na resposta de criação, obteve %q", created.ModeID)
	}

	conv, err := st.GetConversation(created.ID)
	if err != nil {
		t.Fatalf("falha ao obter conversa: %v", err)
	}
	if conv.ModeID != "accept-edits" {
		t.Fatalf("esperava modeId 'accept-edits' salvo no banco, obteve %q", conv.ModeID)
	}
}

func TestServer_AgentInfo_IncludesTier(t *testing.T) {
	_, ts, client, st := setupCapsTestServer(t)
	defer ts.Close()
	defer client.Close()
	defer st.Close()

	// 1. Antes de criar sessão, tier é null
	resp, err := http.Get(ts.URL + "/api/agent-info")
	if err != nil {
		t.Fatalf("falha ao buscar agent-info: %v", err)
	}
	var infoBefore struct {
		Email *string       `json:"email"`
		Tier  *acp.TierInfo `json:"tier"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&infoBefore)
	resp.Body.Close()
	if infoBefore.Tier != nil {
		t.Fatalf("esperava tier nulo antes de qualquer sessão, obteve %+v", infoBefore.Tier)
	}

	// 2. Cria conversa/sessão com agente
	body, _ := json.Marshal(map[string]string{
		"cwd": t.TempDir(),
	})
	resp, err = http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	resp.Body.Close()

	// 3. Agora agent-info deve conter o tier retornado pelo agente na sessão
	resp, err = http.Get(ts.URL + "/api/agent-info")
	if err != nil {
		t.Fatalf("falha ao buscar agent-info pós sessão: %v", err)
	}
	var infoAfter struct {
		Email *string       `json:"email"`
		Tier  *acp.TierInfo `json:"tier"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&infoAfter)
	resp.Body.Close()

	if infoAfter.Tier == nil || infoAfter.Tier.Name != "Google AI Pro" {
		t.Fatalf("esperava tier 'Google AI Pro', obteve %+v", infoAfter.Tier)
	}
}
