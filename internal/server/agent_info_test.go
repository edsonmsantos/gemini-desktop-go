package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestServer_AgentInfo(t *testing.T) {
	// Caminho 1: Arquivo válido
	tmpDir := t.TempDir()
	accountsFile := filepath.Join(tmpDir, "google_accounts.json")
	if err := os.WriteFile(accountsFile, []byte(`{"active":"alice@example.com","old":[]}`), 0644); err != nil {
		t.Fatalf("falha ao escrever arquivo de teste: %v", err)
	}

	srv := &Server{
		mux:                http.NewServeMux(),
		googleAccountsPath: accountsFile,
	}
	srv.setupRoutes()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/agent-info")
	if err != nil {
		t.Fatalf("falha na requisicao: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava status 200, obteve %d", resp.StatusCode)
	}

	var resValid struct {
		Email *string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resValid); err != nil {
		t.Fatalf("falha ao decodificar json: %v", err)
	}
	resp.Body.Close()

	if resValid.Email == nil || *resValid.Email != "alice@example.com" {
		t.Fatalf("esperava email 'alice@example.com', obteve %v", resValid.Email)
	}

	// Caminho 2a: Arquivo ausente
	srvMissing := &Server{
		mux:                http.NewServeMux(),
		googleAccountsPath: filepath.Join(tmpDir, "nao_existe.json"),
	}
	srvMissing.setupRoutes()
	tsMissing := httptest.NewServer(srvMissing.Handler())
	defer tsMissing.Close()

	resp, err = http.Get(tsMissing.URL + "/api/agent-info")
	if err != nil {
		t.Fatalf("falha na requisicao: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava status 200, obteve %d", resp.StatusCode)
	}

	var resNull struct {
		Email *string `json:"email"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&resNull); err != nil {
		t.Fatalf("falha ao decodificar json: %v", err)
	}
	resp.Body.Close()

	if resNull.Email != nil {
		t.Fatalf("esperava email null para arquivo ausente, obteve %v", *resNull.Email)
	}

	// Caminho 2b: Arquivo com JSON inválido
	invalidFile := filepath.Join(tmpDir, "invalid.json")
	_ = os.WriteFile(invalidFile, []byte(`not a json`), 0644)
	srvInvalid := &Server{
		mux:                http.NewServeMux(),
		googleAccountsPath: invalidFile,
	}
	srvInvalid.setupRoutes()
	tsInvalid := httptest.NewServer(srvInvalid.Handler())
	defer tsInvalid.Close()

	resp, err = http.Get(tsInvalid.URL + "/api/agent-info")
	if err != nil {
		t.Fatalf("falha na requisicao: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava status 200, obteve %d", resp.StatusCode)
	}

	var resInvalid struct {
		Email *string `json:"email"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&resInvalid)
	resp.Body.Close()

	if resInvalid.Email != nil {
		t.Fatalf("esperava email null para json invalido, obteve %v", *resInvalid.Email)
	}
}
