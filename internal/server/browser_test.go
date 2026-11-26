package server

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"geminidesktop-go/internal/store"
)

func TestBrowser_Status_OnlineWithToken(t *testing.T) {
	expectedToken := "secret-auth-token-123"

	// Ponte simulada (daemon Go com token Bearer)
	bridgeStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/status" {
			auth := r.Header.Get("Authorization")
			if auth != "Bearer "+expectedToken {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"connected": true,
				"mode": "ask",
				"extensionConnected": true,
				"approvalsPending": 2
			}`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bridgeStub.Close()

	u, _ := url.Parse(bridgeStub.URL)
	port, _ := strconv.Atoi(u.Port())

	// Arquivo temporário de token
	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.txt")
	if err := os.WriteFile(tokenFile, []byte(expectedToken), 0600); err != nil {
		t.Fatalf("falha ao criar token temporário: %v", err)
	}

	st, err := store.NewStore(filepath.Join(tmpDir, "test.db"))
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(nil, tmpDir, nil, st)
	srv.SetBrowserBridge(port, tokenFile)

	req := httptest.NewRequest(http.MethodGet, "/api/browser/status", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperava 200 OK, obteve %d", rr.Code)
	}

	var status BrowserStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("erro ao decodificar resposta: %v", err)
	}

	if !status.Connected {
		t.Fatalf("esperava connected=true, obteve false")
	}
	if status.Daemon != "go" {
		t.Fatalf("esperava daemon=go, obteve %q", status.Daemon)
	}
	if status.Mode != "ask" {
		t.Fatalf("esperava mode=ask, obteve %q", status.Mode)
	}
	if !status.ExtensionConnected {
		t.Fatalf("esperava extensionConnected=true")
	}
	if status.ApprovalsPending != 2 {
		t.Fatalf("esperava approvalsPending=2, obteve %d", status.ApprovalsPending)
	}
}

func TestBrowser_Status_FallbackNoToken_NodeDaemon(t *testing.T) {
	// Daemon Node: /api/status retorna 404, mas /api/query responde
	bridgeStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Se vier com token inválido, simula 401
		if r.Header.Get("Authorization") != "" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		if r.URL.Path == "/api/status" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		if r.URL.Path == "/api/query" {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`[]`))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bridgeStub.Close()

	u, _ := url.Parse(bridgeStub.URL)
	port, _ := strconv.Atoi(u.Port())

	tmpDir := t.TempDir()
	tokenFile := filepath.Join(tmpDir, "token.txt")
	_ = os.WriteFile(tokenFile, []byte("bad-token"), 0600)

	st, _ := store.NewStore(filepath.Join(tmpDir, "test.db"))
	defer st.Close()

	srv := NewServerV2(nil, tmpDir, nil, st)
	srv.SetBrowserBridge(port, tokenFile)

	req := httptest.NewRequest(http.MethodGet, "/api/browser/status", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperava 200 OK, obteve %d", rr.Code)
	}

	var status BrowserStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("erro ao decodificar resposta: %v", err)
	}

	if !status.Connected {
		t.Fatalf("esperava connected=true para daemon Node")
	}
	if status.Daemon != "node" {
		t.Fatalf("esperava daemon=node, obteve %q", status.Daemon)
	}
}

func TestBrowser_Status_Offline(t *testing.T) {
	tmpDir := t.TempDir()
	st, _ := store.NewStore(filepath.Join(tmpDir, "test.db"))
	defer st.Close()

	srv := NewServerV2(nil, tmpDir, nil, st)
	srv.SetBrowserBridge(59999, "") // Porta fechada

	req := httptest.NewRequest(http.MethodGet, "/api/browser/status", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("esperava 200 OK com status disconnected, obteve %d", rr.Code)
	}

	var status BrowserStatusResponse
	if err := json.NewDecoder(rr.Body).Decode(&status); err != nil {
		t.Fatalf("erro ao decodificar resposta: %v", err)
	}

	if status.Connected {
		t.Fatalf("esperava connected=false, obteve true")
	}
	if !strings.Contains(status.Reason, "não está rodando") {
		t.Fatalf("esperava reason com 'não está rodando', obteve %q", status.Reason)
	}
}

func TestBrowser_Approvals_404And200AndPost(t *testing.T) {
	approvalsEnabled := false
	approvedID := ""

	bridgeStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/approvals") {
			if !approvalsEnabled {
				w.WriteHeader(http.StatusNotFound)
				return
			}

			if r.Method == http.MethodGet && r.URL.Path == "/api/approvals" {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`[{"id":"appr-abc","tool":"browser_click","args":{"selector":"#submit"}}]`))
				return
			}

			if r.Method == http.MethodPost && r.URL.Path == "/api/approvals/appr-abc" {
				var body map[string]bool
				_ = json.NewDecoder(r.Body).Decode(&body)
				if body["approve"] {
					approvedID = "appr-abc"
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte(`{"status":"resolved"}`))
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bridgeStub.Close()

	u, _ := url.Parse(bridgeStub.URL)
	port, _ := strconv.Atoi(u.Port())

	tmpDir := t.TempDir()
	st, _ := store.NewStore(filepath.Join(tmpDir, "test.db"))
	defer st.Close()

	srv := NewServerV2(nil, tmpDir, nil, st)
	srv.SetBrowserBridge(port, "")

	// 1. approvals 404 (feature off)
	req := httptest.NewRequest(http.MethodGet, "/api/browser/approvals", nil)
	rr := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr, req)
	if rr.Code != http.StatusNotFound {
		t.Fatalf("esperava 404 para approvals desabilitado, obteve %d", rr.Code)
	}

	// 2. approvals 200
	approvalsEnabled = true
	req2 := httptest.NewRequest(http.MethodGet, "/api/browser/approvals", nil)
	rr2 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rr2, req2)
	if rr2.Code != http.StatusOK {
		t.Fatalf("esperava 200 para approvals ativo, obteve %d", rr2.Code)
	}
	if !strings.Contains(rr2.Body.String(), "appr-abc") {
		t.Fatalf("resposta não contém aprovação: %s", rr2.Body.String())
	}

	// 3. POST approval
	reqPost := httptest.NewRequest(http.MethodPost, "/api/browser/approvals/appr-abc", strings.NewReader(`{"approve":true}`))
	reqPost.Header.Set("Content-Type", "application/json")
	rrPost := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rrPost, reqPost)
	if rrPost.Code != http.StatusOK {
		t.Fatalf("esperava 200 no POST approval, obteve %d", rrPost.Code)
	}
	if approvedID != "appr-abc" {
		t.Fatalf("esperava approvedID=appr-abc, obteve %q", approvedID)
	}
}

func TestBrowser_ResolveLatestScreenshot_AndPruning(t *testing.T) {
	now := time.Now().UnixMilli()

	bridgeStub := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/query") {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			// Retorna entradas de screenshot dentro do intervalo de tolerância
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"id":"shot-old","contentType":"screenshot","created_at":%d,"hasImage":true},
				{"id":"shot-new","contentType":"screenshot","created_at":%d,"hasImage":true}
			]`, now-5000, now-500)))
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/image") {
			w.Header().Set("Content-Type", "image/png")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("\x89PNG\r\n\x1a\nfake-image-bytes"))
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer bridgeStub.Close()

	u, _ := url.Parse(bridgeStub.URL)
	port, _ := strconv.Atoi(u.Port())

	tmpDir := t.TempDir()
	screenshotsBase := filepath.Join(tmpDir, "screenshots")

	st, _ := store.NewStore(filepath.Join(tmpDir, "test.db"))
	defer st.Close()

	srv := NewServerV2(nil, tmpDir, nil, st)
	srv.SetBrowserBridge(port, "")
	srv.SetScreenshotsBaseDir(screenshotsBase)

	convID := "conv-test-pruning"
	convDir := filepath.Join(screenshotsBase, convID)
	_ = os.MkdirAll(convDir, 0755)

	// Cria 52 arquivos antigos fakes no diretório para testar a poda (>50)
	for i := 1; i <= 52; i++ {
		fakeFile := filepath.Join(convDir, fmt.Sprintf("%013d.png", int64(i)*1000))
		_ = os.WriteFile(fakeFile, []byte("fake"), 0644)
	}

	// Resolve screenshot
	shotPath, err := srv.resolveLatestScreenshot(context.Background(), convID, now-1000)
	if err != nil {
		t.Fatalf("falha ao resolver screenshot: %v", err)
	}

	if shotPath == "" {
		t.Fatalf("esperava shotPath não vazio")
	}

	if !strings.HasSuffix(shotPath, ".png") {
		t.Fatalf("esperava extensão .png no caminho, obteve %s", shotPath)
	}

	// Verifica se o arquivo foi realmente escrito no disco
	data, err := os.ReadFile(shotPath)
	if err != nil {
		t.Fatalf("falha ao ler arquivo salvo: %v", err)
	}
	if !strings.Contains(string(data), "fake-image-bytes") {
		t.Fatalf("conteúdo do arquivo salvo inesperado: %q", string(data))
	}

	// Verifica se a poda manteve no máximo 50 arquivos no diretório da conversa
	entries, err := os.ReadDir(convDir)
	if err != nil {
		t.Fatalf("falha ao ler diretório de screenshots: %v", err)
	}
	if len(entries) > 50 {
		t.Fatalf("esperava <= 50 arquivos após poda, encontrou %d", len(entries))
	}
}
