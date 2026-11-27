package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandleFileContent(t *testing.T) {
	s := &Server{
		mux:               http.NewServeMux(),
		tempWorkspacesDir: os.TempDir(),
	}
	s.setupRoutes()
	ts := httptest.NewServer(s.mux)
	defer ts.Close()

	client := ts.Client()

	postFileContent := func(path string) (*http.Response, map[string]interface{}) {
		body, _ := json.Marshal(map[string]string{"path": path})
		resp, err := client.Post(ts.URL+"/api/files/content", "application/json", bytes.NewReader(body))
		if err != nil {
			t.Fatalf("erro no POST /api/files/content: %v", err)
		}
		var parsed map[string]interface{}
		_ = json.NewDecoder(resp.Body).Decode(&parsed)
		resp.Body.Close()
		return resp, parsed
	}

	t.Run("texto retorna conteúdo", func(t *testing.T) {
		dir := t.TempDir()
		txtPath := filepath.Join(dir, "teste.txt")
		content := "Olá, este é um teste de artefato de texto."
		if err := os.WriteFile(txtPath, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}

		resp, data := postFileContent(txtPath)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperava 200, obteve %d", resp.StatusCode)
		}
		if data["kind"] != "text" {
			t.Errorf("esperava kind text, obteve %v", data["kind"])
		}
		if data["content"] != content {
			t.Errorf("esperava content %q, obteve %q", content, data["content"])
		}
	})

	t.Run("imagem retorna base64 válido", func(t *testing.T) {
		dir := t.TempDir()
		pngPath := filepath.Join(dir, "imagem.png")
		rawImg := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n', 0x00, 0x01, 0x02}
		if err := os.WriteFile(pngPath, rawImg, 0644); err != nil {
			t.Fatal(err)
		}

		resp, data := postFileContent(pngPath)
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("esperava 200, obteve %d", resp.StatusCode)
		}
		if data["kind"] != "image" {
			t.Errorf("esperava kind image, obteve %v", data["kind"])
		}
		if data["mime"] != "image/png" {
			t.Errorf("esperava mime image/png, obteve %v", data["mime"])
		}
		// Deve suportar content e/ou contentBase64
		var b64 string
		if c, ok := data["content"].(string); ok && c != "" {
			b64 = c
		} else if cb, ok := data["contentBase64"].(string); ok && cb != "" {
			b64 = cb
		} else {
			t.Fatalf("esperava base64 em content ou contentBase64")
		}
		decoded, err := base64.StdEncoding.DecodeString(b64)
		if err != nil {
			t.Fatalf("base64 inválido: %v", err)
		}
		if !bytes.Equal(decoded, rawImg) {
			t.Fatalf("bytes decodificados não coincidem")
		}
	})

	t.Run("cap de tamanho texto 5MB", func(t *testing.T) {
		dir := t.TempDir()
		bigTxt := filepath.Join(dir, "grande.txt")
		// 5MB + 1 byte
		size := 5*1024*1024 + 1
		f, err := os.Create(bigTxt)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(int64(size)); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()

		resp, _ := postFileContent(bigTxt)
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("esperava 413 para texto > 5MB, obteve %d", resp.StatusCode)
		}
	})

	t.Run("cap de tamanho imagem 8MB", func(t *testing.T) {
		dir := t.TempDir()
		bigImg := filepath.Join(dir, "grande.png")
		// 8MB + 1 byte
		size := 8*1024*1024 + 1
		f, err := os.Create(bigImg)
		if err != nil {
			t.Fatal(err)
		}
		if err := f.Truncate(int64(size)); err != nil {
			f.Close()
			t.Fatal(err)
		}
		f.Close()

		resp, _ := postFileContent(bigImg)
		if resp.StatusCode != http.StatusRequestEntityTooLarge {
			t.Fatalf("esperava 413 para imagem > 8MB, obteve %d", resp.StatusCode)
		}
	})

	t.Run("inexistente 404", func(t *testing.T) {
		dir := t.TempDir()
		resp, _ := postFileContent(filepath.Join(dir, "arquivo_inexistente.txt"))
		if resp.StatusCode != http.StatusNotFound {
			t.Fatalf("esperava 404, obteve %d", resp.StatusCode)
		}
	})

	t.Run("arquivo fora da allowlist 403", func(t *testing.T) {
		resp, _ := postFileContent("/etc/passwd")
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("esperava 403 para arquivo fora da allowlist, obteve %d", resp.StatusCode)
		}
	})

	t.Run("diretório 400", func(t *testing.T) {
		dir := t.TempDir()
		resp, _ := postFileContent(dir)
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("esperava 400 para diretório, obteve %d", resp.StatusCode)
		}
	})

	t.Run("caminho relativo 400", func(t *testing.T) {
		resp, _ := postFileContent("relativo/arquivo.txt")
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("esperava 400 para caminho relativo, obteve %d", resp.StatusCode)
		}
	})

	t.Run("binário com 0x00 nas primeiras 8KB retorna 415", func(t *testing.T) {
		dir := t.TempDir()
		binPath := filepath.Join(dir, "falso_texto.txt")
		binData := append([]byte("Começo de texto"), 0x00, 0x01, 0x02)
		if err := os.WriteFile(binPath, binData, 0644); err != nil {
			t.Fatal(err)
		}

		resp, _ := postFileContent(binPath)
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("esperava 415 para binário, obteve %d", resp.StatusCode)
		}
	})

	t.Run("tipo não previewável retorna 415", func(t *testing.T) {
		dir := t.TempDir()
		pdfPath := filepath.Join(dir, "documento.pdf")
		if err := os.WriteFile(pdfPath, []byte("%PDF-1.4..."), 0644); err != nil {
			t.Fatal(err)
		}

		resp, _ := postFileContent(pdfPath)
		if resp.StatusCode != http.StatusUnsupportedMediaType {
			t.Fatalf("esperava 415 para tipo não previewável, obteve %d", resp.StatusCode)
		}
	})
}
