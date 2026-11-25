package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderMarkdownPDF_GeraArquivoValido(t *testing.T) {
	dest := filepath.Join(t.TempDir(), "resp.pdf")
	md := "# Título com acentuação\n\nParágrafo com **negrito**, `código` e [link](https://exemplo.com).\n\n- item um\n- item dois\n\n```go\nfmt.Println(\"olá\")\n```\n\n> citação\n\n---\n\n1. primeiro\n2. segundo"
	if err := RenderMarkdownPDF(dest, "Teste de Exportação", md); err != nil {
		t.Fatalf("falha ao gerar PDF: %v", err)
	}
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("falha ao ler PDF: %v", err)
	}
	if len(data) < 500 || !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("PDF inválido: %d bytes", len(data))
	}
}

func TestHandleExportPDF_EndToEnd(t *testing.T) {
	s := &Server{notifications: false}
	ts := httptest.NewServer(http.HandlerFunc(s.handleExportPDF))
	defer ts.Close()

	body, _ := json.Marshal(map[string]string{
		"title":    "Resposta/Teste com acentos",
		"markdown": "## Resposta\n\nConteúdo **útil** aqui.",
	})
	resp, err := http.Post(ts.URL, "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", resp.StatusCode)
	}
	var out struct {
		Path        string `json:"path"`
		DisplayPath string `json:"displayPath"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out.Path == "" || out.DisplayPath == "" {
		t.Fatalf("resposta sem caminhos: %+v", out)
	}
	data, err := os.ReadFile(out.Path)
	if err != nil || !bytes.HasPrefix(data, []byte("%PDF")) {
		t.Fatalf("PDF gerado inválido: %v", err)
	}
	if !strings.Contains(out.DisplayPath, "~/Downloads/Gemini Desktop/") {
		t.Fatalf("displayPath inesperado: %q", out.DisplayPath)
	}
	// Limpeza do arquivo real gerado em ~/Downloads
	_ = os.Remove(out.Path)
}

func TestHandleExportPDF_Validacoes(t *testing.T) {
	s := &Server{notifications: false}
	ts := httptest.NewServer(http.HandlerFunc(s.handleExportPDF))
	defer ts.Close()

	// markdown vazio
	resp, _ := http.Post(ts.URL, "application/json", bytes.NewReader([]byte(`{"title":"x","markdown":"  "}`)))
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para markdown vazio, obteve %d", resp.StatusCode)
	}
}

func TestHandleOpenURL_Validacoes(t *testing.T) {
	s := &Server{}
	ts := httptest.NewServer(http.HandlerFunc(s.handleOpenURL))
	defer ts.Close()

	for _, bad := range []string{
		`{"url":"javascript:alert(1)"}`,
		`{"url":"file:///etc/passwd"}`,
		`{"url":"nao-e-url"}`,
		`{}`,
	} {
		resp, err := http.Post(ts.URL, "application/json", bytes.NewReader([]byte(bad)))
		if err != nil {
			t.Fatal(err)
		}
		resp.Body.Close()
		if resp.StatusCode != http.StatusBadRequest {
			t.Fatalf("esperava 400 para %s, obteve %d", bad, resp.StatusCode)
		}
	}
}
