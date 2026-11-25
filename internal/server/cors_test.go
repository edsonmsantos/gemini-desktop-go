package server

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestServer_CORSMiddleware(t *testing.T) {
	srv := &Server{
		mux: http.NewServeMux(),
	}
	srv.setupRoutes()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// 1. Preflight OPTIONS com Origin wails://wails.localhost -> 204 + ACAO + ACAM + ACAH
	req, err := http.NewRequest(http.MethodOptions, ts.URL+"/api/conversations", nil)
	if err != nil {
		t.Fatalf("falha ao criar request: %v", err)
	}
	req.Header.Set("Origin", "wails://wails.localhost")
	req.Header.Set("Access-Control-Request-Method", "POST")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("falha no request OPTIONS: %v", err)
	}
	resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("esperava status 204 em preflight OPTIONS, obteve %d", resp.StatusCode)
	}
	acao := resp.Header.Get("Access-Control-Allow-Origin")
	if acao != "wails://wails.localhost" {
		t.Fatalf("esperava ACAO 'wails://wails.localhost', obteve '%s'", acao)
	}
	acam := resp.Header.Get("Access-Control-Allow-Methods")
	if !strings.Contains(acam, "POST") || !strings.Contains(acam, "GET") {
		t.Fatalf("esperava ACAM contendo methods, obteve '%s'", acam)
	}
	acah := resp.Header.Get("Access-Control-Allow-Headers")
	if !strings.Contains(acah, "Content-Type") {
		t.Fatalf("esperava ACAH contendo Content-Type, obteve '%s'", acah)
	}

	// 2. GET /api/agent-info com Origin permitido http://localhost:5173 -> ACAO presente
	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/agent-info", nil)
	if err != nil {
		t.Fatalf("falha ao criar request: %v", err)
	}
	req.Header.Set("Origin", "http://localhost:5173")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("falha no request GET: %v", err)
	}
	resp.Body.Close()

	acao = resp.Header.Get("Access-Control-Allow-Origin")
	if acao != "http://localhost:5173" {
		t.Fatalf("esperava ACAO 'http://localhost:5173', obteve '%s'", acao)
	}

	// 3. GET /api/agent-info com Origin proibido http://evil.com -> SEM ACAO
	req, err = http.NewRequest(http.MethodGet, ts.URL+"/api/agent-info", nil)
	if err != nil {
		t.Fatalf("falha ao criar request: %v", err)
	}
	req.Header.Set("Origin", "http://evil.com")

	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("falha no request GET: %v", err)
	}
	resp.Body.Close()

	acao = resp.Header.Get("Access-Control-Allow-Origin")
	if acao != "" {
		t.Fatalf("esperava ACAO vazio para origin proibido, obteve '%s'", acao)
	}
}
