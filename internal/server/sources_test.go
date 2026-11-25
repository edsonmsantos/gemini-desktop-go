package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestSourcesResolve(t *testing.T) {
	// 1. Servidor de destino e redirecionamento
	var targetHits int32
	destMux := http.NewServeMux()
	destMux.HandleFunc("/dest", func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&targetHits, 1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("destino"))
	})
	destMux.Handle("/redirect", http.RedirectHandler("/dest", http.StatusFound))

	targetServer := httptest.NewServer(destMux)
	defer targetServer.Close()

	// 2. Servidor da aplicação
	srv := &Server{
		mux: http.NewServeMux(),
	}
	srv.setupRoutes()
	apiServer := httptest.NewServer(srv.Handler())
	defer apiServer.Close()

	client := apiServer.Client()

	// Caso A: URL válida com redirecionamento e URL inválida (sem scheme)
	redirectURL := targetServer.URL + "/redirect"
	expectedDestURL := targetServer.URL + "/dest"
	invalidURL := "invalid-url-sem-scheme"

	payload := map[string][]string{
		"urls": {redirectURL, invalidURL},
	}
	body, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("falha ao codificar json: %v", err)
	}

	resp, err := client.Post(apiServer.URL+"/api/sources/resolve", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("POST /api/sources/resolve falhou: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava status 200, obteve %d", resp.StatusCode)
	}

	var result struct {
		Resolved []SourceResolution `json:"resolved"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("falha ao decodificar resposta: %v", err)
	}

	if len(result.Resolved) != 2 {
		t.Fatalf("esperava 2 itens resolvidos, obteve %d", len(result.Resolved))
	}

	// Verifica o redirectURL
	res1 := result.Resolved[0]
	if res1.From != redirectURL {
		t.Errorf("res1.From: esperava %s, obteve %s", redirectURL, res1.From)
	}
	if res1.To != expectedDestURL {
		t.Errorf("res1.To: esperava %s, obteve %s", expectedDestURL, res1.To)
	}
	if res1.Status != http.StatusOK {
		t.Errorf("res1.Status: esperava 200, obteve %d", res1.Status)
	}

	// Verifica a URL inválida
	res2 := result.Resolved[1]
	if res2.From != invalidURL {
		t.Errorf("res2.From: esperava %s, obteve %s", invalidURL, res2.From)
	}
	if res2.To != invalidURL {
		t.Errorf("res2.To: esperava %s (from=to para inválido), obteve %s", invalidURL, res2.To)
	}
	if res2.Status != 0 {
		t.Errorf("res2.Status: esperava 0, obteve %d", res2.Status)
	}

	// Verifica se houve exatamente 1 hit no endpoint de destino
	initialHits := atomic.LoadInt32(&targetHits)
	if initialHits != 1 {
		t.Fatalf("esperava targetHits == 1, obteve %d", initialHits)
	}

	// Caso B: Cache hit - segunda chamada idêntica não deve gerar nova requisição HTTP para o destino
	body2, _ := json.Marshal(map[string][]string{
		"urls": {redirectURL},
	})
	resp2, err := client.Post(apiServer.URL+"/api/sources/resolve", "application/json", bytes.NewReader(body2))
	if err != nil {
		t.Fatalf("segunda chamada POST falhou: %v", err)
	}
	defer resp2.Body.Close()

	var result2 struct {
		Resolved []SourceResolution `json:"resolved"`
	}
	if err := json.NewDecoder(resp2.Body).Decode(&result2); err != nil {
		t.Fatalf("falha ao decodificar resposta da segunda chamada: %v", err)
	}
	if len(result2.Resolved) != 1 || result2.Resolved[0].To != expectedDestURL {
		t.Fatalf("esperava retorno em cache com to=%s, obteve %+v", expectedDestURL, result2.Resolved)
	}

	// Contador de hits deve permanecer inalterado
	newHits := atomic.LoadInt32(&targetHits)
	if newHits != initialHits {
		t.Errorf("cache falhou: targetHits mudou de %d para %d", initialHits, newHits)
	}
}
