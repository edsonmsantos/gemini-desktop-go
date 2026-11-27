package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSecurity_OriginAndSessionToken(t *testing.T) {
	srv := &Server{
		mux:  http.NewServeMux(),
		port: 8399,
	}
	srv.setupRoutes()
	srv.SetSessionToken("test-secret-token")

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// 1. Origin não permitido em produção deve retornar 403 Forbidden
	req, err := http.NewRequest(http.MethodGet, ts.URL+"/api/agent-info", nil)
	if err != nil {
		t.Fatalf("falha ao criar request: %v", err)
	}
	req.Header.Set("Origin", "http://malicious-website.com")
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("falha no request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("esperava 403 Forbidden para origin malicioso, obteve %d", resp.StatusCode)
	}

	// 2. Método mutativo sem token deve retornar 401 Unauthorized
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/api/conversations", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("falha ao criar request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("falha no request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("esperava 401 Unauthorized para POST sem token, obteve %d", resp.StatusCode)
	}

	// 3. Método mutativo com X-Gemini-Token válido é autorizado (passa pelo middleware)
	req, err = http.NewRequest(http.MethodPost, ts.URL+"/api/conversations", strings.NewReader(`{}`))
	if err != nil {
		t.Fatalf("falha ao criar request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Gemini-Token", "test-secret-token")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("falha no request: %v", err)
	}
	resp.Body.Close()
	// Passou do middleware de auth (mesmo que dê 503 por clientOrError / store nil, não é 401)
	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		t.Fatalf("requisição com token válido foi barrada com status %d", resp.StatusCode)
	}
}

func TestSecurity_ValidateArtifactPath(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "gemini-artifacts-test-*")
	if err != nil {
		t.Fatalf("falha ao criar tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	validFile := filepath.Join(tempDir, "valid-output.png")
	if err := os.WriteFile(validFile, []byte("fake-png-content"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo: %v", err)
	}

	srv := &Server{
		tempWorkspacesDir: tempDir,
	}

	// Arquivo dentro do temp workspace autorizado -> OK
	if _, err := srv.ValidateArtifactPath(validFile); err != nil {
		t.Fatalf("esperava sucesso para arquivo em temp workspace, obteve erro: %v", err)
	}

	// Arquivo fora dos diretórios autorizados -> Rejeitado
	outsideFile := filepath.Join(os.TempDir(), "gemini-outside-file.txt")
	_ = os.WriteFile(outsideFile, []byte("outside"), 0644)
	defer os.Remove(outsideFile)

	if _, err := srv.ValidateArtifactPath(outsideFile); err == nil {
		t.Fatal("esperava erro ao validar arquivo fora dos diretórios autorizados, mas passou")
	}

	// Directory traversal attempt -> Rejeitado
	traversalPath := filepath.Join(tempDir, "..", "..", "etc", "passwd")
	if _, err := srv.ValidateArtifactPath(traversalPath); err == nil {
		t.Fatal("esperava erro para tentativa de directory traversal, mas passou")
	}
}

func TestSecurity_GetEndpointsRequireToken(t *testing.T) {
	srv := &Server{
		mux:  http.NewServeMux(),
		port: 8399,
	}
	srv.setupRoutes()
	srv.SetSessionToken("test-secret-token")

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	client := ts.Client()

	// G3: GET /api/conversations sem token deve retornar 401
	resp, err := client.Get(ts.URL + "/api/conversations")
	if err != nil {
		t.Fatalf("falha no GET: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("esperava 401 Unauthorized para GET /api/conversations sem token, obteve %d", resp.StatusCode)
	}

	// GET /api/conversations com X-Gemini-Token válido é autorizado (passa pelo middleware)
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/conversations", nil)
	req.Header.Set("X-Gemini-Token", "test-secret-token")
	resp, err = client.Do(req)
	if err != nil {
		t.Fatalf("falha no GET com token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("esperava autorização com token válido, mas obteve %d", resp.StatusCode)
	}

	// Endpoints públicos: GET /api/health e GET / não exigem token
	respHealth, err := client.Get(ts.URL + "/api/health")
	if err != nil {
		t.Fatalf("falha no GET /api/health: %v", err)
	}
	respHealth.Body.Close()
	if respHealth.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK para GET /api/health sem token, obteve %d", respHealth.StatusCode)
	}
}

func TestSecurity_ServeModeTokenRequired(t *testing.T) {
	// G1: Em modo serve, com token configurado no boot, rejeita requests desprotegidos e aceita com token
	srv := &Server{
		mux:  http.NewServeMux(),
		port: 8399,
	}
	srv.setupRoutes()
	token := GenerateSessionToken()
	srv.SetSessionToken(token)

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Sem token -> 401
	resp, err := ts.Client().Get(ts.URL + "/api/conversations")
	if err != nil {
		t.Fatalf("falha no request: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("esperava 401 Unauthorized para chamada sem token, obteve %d", resp.StatusCode)
	}

	// 2. Com token -> Autorizado
	req, _ := http.NewRequest(http.MethodGet, ts.URL+"/api/conversations", nil)
	req.Header.Set("X-Gemini-Token", token)
	resp, err = ts.Client().Do(req)
	if err != nil {
		t.Fatalf("falha no request com token: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode == http.StatusUnauthorized {
		t.Fatalf("esperava autorização com token válido, obteve %d", resp.StatusCode)
	}
}

func TestSecurity_APIStatusRootEndpoint(t *testing.T) {
	// Quando staticFS == nil, GET / deve retornar status JSON
	srv := NewServerV2(nil, "", nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := ts.Client().Get(ts.URL + "/")
	if err != nil {
		t.Fatalf("falha ao chamar GET /: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK, obteve %d", resp.StatusCode)
	}
	buf := new(bytes.Buffer)
	buf.ReadFrom(resp.Body)
	if !strings.Contains(buf.String(), "Gemini Desktop API") {
		t.Fatalf("esperava resposta da API, obteve %s", buf.String())
	}
}

func TestSecurity_SymlinkBypassBlocked_R1(t *testing.T) {
	// R1: Symlink dentro de diretório permitido apontando para fora da allowlist deve ser rejeitado
	tempDir, err := os.MkdirTemp("", "gemini-symlink-test-*")
	if err != nil {
		t.Fatalf("falha ao criar tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	secretFile := filepath.Join(os.TempDir(), "gemini-secret-outside.txt")
	if err := os.WriteFile(secretFile, []byte("CHAVE_SECRETA_SENSIVEL"), 0644); err != nil {
		t.Fatalf("falha ao criar arquivo secreto: %v", err)
	}
	defer os.Remove(secretFile)

	symlinkFile := filepath.Join(tempDir, "innocent.txt")
	if err := os.Symlink(secretFile, symlinkFile); err != nil {
		t.Fatalf("falha ao criar symlink: %v", err)
	}

	srv := &Server{
		mux:               http.NewServeMux(),
		tempWorkspacesDir: tempDir,
	}
	srv.setupRoutes()

	// 1. ValidateArtifactPath deve rejeitar o symlink
	if _, err := srv.ValidateArtifactPath(symlinkFile); err == nil {
		t.Fatal("esperava erro de segurança ao validar symlink apontando para fora da allowlist, mas passou")
	}

	// 2. /api/files/content deve retornar 403 Forbidden
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{"path": symlinkFile})
	resp, err := ts.Client().Post(ts.URL+"/api/files/content", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("falha no POST /api/files/content: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("esperava 403 Forbidden para leitura de symlink malicioso, obteve %d", resp.StatusCode)
	}
}

func TestSecurity_OpenArtifactExtensionAllowlist_R2(t *testing.T) {
	// R2: Bloqueia launchers e executáveis (.desktop, .sh, etc.) mesmo que estejam em diretórios permitidos
	tempDir, err := os.MkdirTemp("", "gemini-open-test-*")
	if err != nil {
		t.Fatalf("falha ao criar tempdir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	srv := &Server{
		mux:               http.NewServeMux(),
		tempWorkspacesDir: tempDir,
	}
	srv.setupRoutes()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Cria arquivos maliciosos de teste dentro do temp workspace autorizado
	desktopFile := filepath.Join(tempDir, "evil.desktop")
	_ = os.WriteFile(desktopFile, []byte("[Desktop Entry]\nExec=xcalc\n"), 0644)

	shFile := filepath.Join(tempDir, "evil.sh")
	_ = os.WriteFile(shFile, []byte("#!/bin/sh\nexit 0\n"), 0755)

	postOpen := func(path string) int {
		body, _ := json.Marshal(map[string]string{"path": path})
		resp, err := ts.Client().Post(ts.URL+"/api/artifacts/open", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("falha no POST /api/artifacts/open: %v", err)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}

	if code := postOpen(desktopFile); code != http.StatusBadRequest {
		t.Fatalf("esperava 400 Bad Request para .desktop, obteve %d", code)
	}

	if code := postOpen(shFile); code != http.StatusBadRequest {
		t.Fatalf("esperava 400 Bad Request para .sh, obteve %d", code)
	}
}

func TestSecurity_AttachmentLargeAllowed_R3(t *testing.T) {
	// R3: Anexo legítimo de até 24.8 MiB decodificado (~33 MiB em base64) deve passar pelo MaxBytesReader (40MB)
	srv := &Server{
		mux:  http.NewServeMux(),
		port: 8399,
	}
	srv.setupRoutes()
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// Cria payload de ~24.5 MiB (dentro do teto de 25MB do handler, mas > 32MB em base64)
	rawSize := 24*1024*1024 + 500*1024 // ~24.5 MiB
	rawBytes := make([]byte, rawSize)
	for i := range rawBytes {
		rawBytes[i] = byte(i % 256)
	}
	b64Data := base64.StdEncoding.EncodeToString(rawBytes)

	reqPayload := map[string]string{
		"filename":   "large_test.bin",
		"dataBase64": b64Data,
	}
	body, err := json.Marshal(reqPayload)
	if err != nil {
		t.Fatalf("falha ao serializar json: %v", err)
	}

	resp, err := ts.Client().Post(ts.URL+"/api/attachments", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatalf("falha no POST /api/attachments: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK para upload de anexo de 24.5MB, obteve %d", resp.StatusCode)
	}

	var res map[string]string
	_ = json.NewDecoder(resp.Body).Decode(&res)
	if path, ok := res["path"]; ok && path != "" {
		_ = os.Remove(path)
	}
}
