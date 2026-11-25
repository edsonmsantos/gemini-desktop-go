package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"geminidesktop-go/internal/store"
)

func TestDiagnostics_NoClient(t *testing.T) {
	tmpDir := t.TempDir()
	accountsFile := filepath.Join(tmpDir, "google_accounts.json")
	if err := os.WriteFile(accountsFile, []byte(`{"active":"test@example.com"}`), 0644); err != nil {
		t.Fatalf("falha ao preparar google_accounts.json: %v", err)
	}

	st, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := &Server{
		mux:                http.NewServeMux(),
		store:              st,
		googleAccountsPath: accountsFile,
	}
	srv.SetPort(8399)
	srv.SetAgentBinary("ls") // comando que certamente existe
	srv.setupRoutes()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/diagnostics")
	if err != nil {
		t.Fatalf("erro ao chamar GET /api/diagnostics: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava status 200, recebeu %d", resp.StatusCode)
	}

	var diag DiagnosticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&diag); err != nil {
		t.Fatalf("erro ao decodificar JSON de diagnóstico: %v", err)
	}

	if diag.AgentBinary != "ls" {
		t.Errorf("esperava agentBinary 'ls', recebeu %q", diag.AgentBinary)
	}
	if !diag.AgentFound {
		t.Errorf("esperava agentFound true para 'ls'")
	}
	if diag.AgentAlive {
		t.Errorf("esperava agentAlive false quando s.client == nil")
	}
	if diag.GoogleEmail == nil || *diag.GoogleEmail != "test@example.com" {
		t.Errorf("esperava googleEmail 'test@example.com', recebeu %v", diag.GoogleEmail)
	}
	if !diag.DbOk {
		t.Errorf("esperava dbOk true")
	}
	if diag.Port != 8399 {
		t.Errorf("esperava port 8399, recebeu %d", diag.Port)
	}
}

func TestDiagnostics_AgentNotFound(t *testing.T) {
	srv := &Server{
		mux: http.NewServeMux(),
	}
	srv.SetAgentBinary("binario_que_certamente_nao_existe_xyz_123")
	srv.setupRoutes()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/diagnostics")
	if err != nil {
		t.Fatalf("erro na requisição: %v", err)
	}
	defer resp.Body.Close()

	var diag DiagnosticsResponse
	if err := json.NewDecoder(resp.Body).Decode(&diag); err != nil {
		t.Fatalf("erro ao decodificar JSON: %v", err)
	}

	if diag.AgentFound {
		t.Errorf("esperava agentFound false")
	}
	if diag.AgentAlive {
		t.Errorf("esperava agentAlive false")
	}
	if diag.DbOk {
		t.Errorf("esperava dbOk false quando s.store == nil")
	}
	if diag.Port != 8399 {
		t.Errorf("esperava port default 8399, recebeu %d", diag.Port)
	}
}

func TestOnboarding_Flow(t *testing.T) {
	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	srv := &Server{
		mux: http.NewServeMux(),
	}
	srv.setupRoutes()

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Status inicial: deve ser false
	resp, err := http.Get(ts.URL + "/api/onboarding/status")
	if err != nil {
		t.Fatalf("erro ao verificar status: %v", err)
	}
	defer resp.Body.Close()

	var statusResp struct {
		Done bool `json:"done"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&statusResp); err != nil {
		t.Fatalf("erro ao decodificar status inicial: %v", err)
	}
	if statusResp.Done {
		t.Fatalf("esperava status inicial done=false, recebeu true")
	}

	// 2. Marcar como done via POST
	postResp, err := http.Post(ts.URL+"/api/onboarding/done", "application/json", bytes.NewBuffer([]byte("{}")))
	if err != nil {
		t.Fatalf("erro no POST /api/onboarding/done: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		t.Fatalf("esperava status 200 no POST done, recebeu %d", postResp.StatusCode)
	}

	var doneResp struct {
		Done bool `json:"done"`
	}
	if err := json.NewDecoder(postResp.Body).Decode(&doneResp); err != nil {
		t.Fatalf("erro ao decodificar resposta de done: %v", err)
	}
	if !doneResp.Done {
		t.Fatalf("esperava done=true na resposta do POST")
	}

	// Verificar se arquivo marker existe com perms 0600
	markerPath := filepath.Join(tmpHome, ".geminidesktop", "onboarded")
	fi, err := os.Stat(markerPath)
	if err != nil {
		t.Fatalf("arquivo marker %s não foi criado: %v", markerPath, err)
	}
	if fi.Mode().Perm() != 0600 {
		t.Errorf("esperava permissões 0600 no marker, obteve %o", fi.Mode().Perm())
	}

	// 3. Status subsequente: deve ser true
	resp2, err := http.Get(ts.URL + "/api/onboarding/status")
	if err != nil {
		t.Fatalf("erro ao verificar status 2: %v", err)
	}
	defer resp2.Body.Close()

	var statusResp2 struct {
		Done bool `json:"done"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&statusResp2); err != nil {
		t.Fatalf("erro ao decodificar status final: %v", err)
	}
	if !statusResp2.Done {
		t.Fatalf("esperava status final done=true, recebeu false")
	}
}
