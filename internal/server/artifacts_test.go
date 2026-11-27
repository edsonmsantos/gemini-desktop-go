package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"geminidesktop-go/internal/store"
)

func TestHandleArtifacts_ListaCriadosPosConversa(t *testing.T) {
	cwd := t.TempDir()
	dbPath := filepath.Join(t.TempDir(), "art.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	// Arquivo PRÉ-existente (input do usuário): não deve aparecer
	os.WriteFile(filepath.Join(cwd, "input.txt"), []byte("entrada"), 0644)
	time.Sleep(20 * time.Millisecond)

	conv := &store.Conversation{
		ID: "conv-art", Title: "Art", Agent: "agy", ModelID: "m",
		CWD: cwd, ACPSessionID: "s", CreatedAt: time.Now().UnixMilli(), UpdatedAt: time.Now().UnixMilli(),
	}
	if err := st.CreateConversation(conv); err != nil {
		t.Fatal(err)
	}

	// Agente "cria" arquivos: raiz, subdiretório e oculto (deve ignorar)
	time.Sleep(20 * time.Millisecond)
	os.WriteFile(filepath.Join(cwd, "relatorio.pdf"), []byte("pdf"), 0644)
	os.MkdirAll(filepath.Join(cwd, "saida"), 0755)
	os.WriteFile(filepath.Join(cwd, "saida", "dados.json"), []byte("{}"), 0644)
	os.WriteFile(filepath.Join(cwd, ".oculto"), []byte("x"), 0644)
	os.MkdirAll(filepath.Join(cwd, "node_modules", "x"), 0755)
	os.WriteFile(filepath.Join(cwd, "node_modules", "x", "lib.js"), []byte("y"), 0644)

	s := &Server{store: st, mux: http.NewServeMux()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/conversations/{id}/artifacts", s.handleArtifacts)
	ts := httptest.NewServer(mux)
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/conversations/conv-art/artifacts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", resp.StatusCode)
	}
	var out struct {
		Artifacts []Artifact `json:"artifacts"`
		Total     int        `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}

	names := map[string]bool{}
	for _, a := range out.Artifacts {
		names[a.Name] = true
	}
	if !names["relatorio.pdf"] || !names[filepath.Join("saida", "dados.json")] {
		t.Fatalf("artefatos esperados ausentes: %+v", names)
	}
	if names["input.txt"] {
		t.Fatalf("input pré-existente não deveria aparecer: %+v", names)
	}
	if names[".oculto"] || names[filepath.Join("node_modules", "x", "lib.js")] {
		t.Fatalf("oculto/node_modules não deveriam aparecer: %+v", names)
	}
	// Ordenado por modificação desc: o mais recente primeiro
	if len(out.Artifacts) > 1 && out.Artifacts[0].Modified < out.Artifacts[len(out.Artifacts)-1].Modified {
		t.Fatal("lista não está ordenada desc")
	}
}

func TestHandleArtifacts_ConvSemCwd(t *testing.T) {
	st, err := store.NewStore(filepath.Join(t.TempDir(), "a.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	conv := &store.Conversation{
		ID: "conv-sem-cwd", Title: "X", Agent: "agy", ModelID: "m",
		CWD: "", ACPSessionID: "s", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.CreateConversation(conv); err != nil {
		t.Fatal(err)
	}
	s := &Server{store: st, mux: http.NewServeMux()}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/conversations/{id}/artifacts", s.handleArtifacts)
	ts := httptest.NewServer(mux)
	defer ts.Close()
	resp, err := http.Get(ts.URL + "/api/conversations/conv-sem-cwd/artifacts")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var out struct {
		Total int `json:"total"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Total != 0 {
		t.Fatalf("esperava 0 artefatos, obteve %d", out.Total)
	}
}

// TestOpenURL_RejeitaArquivo valida que /api/open-url rejeita paths locais (P0.3)
func TestOpenURL_RejeitaArquivo(t *testing.T) {
	s := &Server{}
	ts := httptest.NewServer(http.HandlerFunc(s.handleOpenURL))
	defer ts.Close()

	// arquivo inexistente ou caminho local → 400 Bad Request
	resp, _ := http.Post(ts.URL, "application/json", bytes.NewReader([]byte(`{"url":"/tmp/nao-existe-xyz.pdf"}`)))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para caminho de arquivo local, obteve %d", resp.StatusCode)
	}

	// diretório → 400 Bad Request
	dir := t.TempDir()
	resp2, _ := http.Post(ts.URL, "application/json", bytes.NewReader([]byte(`{"url":"`+dir+`"}`)))
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para diretório, obteve %d", resp2.StatusCode)
	}
}
