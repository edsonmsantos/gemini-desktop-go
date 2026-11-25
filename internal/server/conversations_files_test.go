package server

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geminidesktop-go/internal/acp"
	"geminidesktop-go/internal/store"
)

func TestServer_CreateConversation_TempWorkspace(t *testing.T) {
	agentBin := findFakeAgent(t)
	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agentBin,
		LogDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("falha ao instanciar client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no initialize: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "temp_ws_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	tempBase := filepath.Join(t.TempDir(), "custom-gemini-temp")
	srv := NewServerV2(client, "", nil, st)
	srv.tempWorkspacesDir = tempBase

	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Criar conversa SEM cwd (ausente)
	reqBody := []byte(`{}`)
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		t.Fatalf("falha no POST /api/conversations: %v", err)
	}
	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 201/200, obteve %d", resp.StatusCode)
	}

	var res struct {
		ID  string `json:"id"`
		CWD string `json:"cwd"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&res)
	resp.Body.Close()

	if res.ID == "" || res.CWD == "" {
		t.Fatalf("id ou cwd vazios na resposta: %+v", res)
	}

	// Verifica se o CWD está dentro de tempBase
	if !strings.HasPrefix(res.CWD, tempBase) {
		t.Fatalf("esperava cwd dentro de %s, obteve %s", tempBase, res.CWD)
	}

	// Polish: uuid do diretório temporário deve ser igual ao id da conversa
	if filepath.Base(res.CWD) != res.ID {
		t.Fatalf("esperava que o diretório temporário tivesse o mesmo uuid da conversa: convID=%s, dir=%s", res.ID, filepath.Base(res.CWD))
	}

	// Verifica se diretório foi criado no disco
	info, err := os.Stat(res.CWD)
	if err != nil || !info.IsDir() {
		t.Fatalf("diretório temporário não foi criado no disco em %s: %v", res.CWD, err)
	}

	// Verifica persistência no banco
	savedConv, err := st.GetConversation(res.ID)
	if err != nil || savedConv.CWD != res.CWD {
		t.Fatalf("conversa não foi persistida com cwd correto no banco: %+v", savedConv)
	}

	// 2. Com cwd inválido/inexistente deve retornar 400
	invalidBody := []byte(`{"cwd":"/caminho/com_certeza_inexistente_123456"}`)
	resp, err = http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(invalidBody))
	if err != nil {
		t.Fatalf("falha no POST invalido: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para cwd inexistente, obteve %d", resp.StatusCode)
	}
}

func TestServer_PromptWithFiles(t *testing.T) {
	agentBin := findFakeAgent(t)
	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agentBin,
		LogDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("falha ao instanciar client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no initialize: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "prompt_files_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(client, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Criar conversa
	createBody, _ := json.Marshal(map[string]string{"cwd": t.TempDir()})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	var convRes struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&convRes)
	resp.Body.Close()

	// 2. Criar arquivos de teste
	tmpDir := t.TempDir()
	pngPath := filepath.Join(tmpDir, "teste.png")
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.RGBA{R: 255, A: 255})
	var pngBuf bytes.Buffer
	if err := png.Encode(&pngBuf, img); err != nil {
		t.Fatalf("falha ao codificar png válido: %v", err)
	}
	if err := os.WriteFile(pngPath, pngBuf.Bytes(), 0644); err != nil {
		t.Fatalf("falha ao criar png de teste: %v", err)
	}

	txtPath := filepath.Join(tmpDir, "dados.txt")
	if err := os.WriteFile(txtPath, []byte("conteudo de texto"), 0644); err != nil {
		t.Fatalf("falha ao criar txt de teste: %v", err)
	}

	// 3. Teste arquivo inexistente -> 400
	missingFile := filepath.Join(tmpDir, "inexistente.txt")
	badReqBody, _ := json.Marshal(map[string]interface{}{
		"text":  "Prompt com arquivo inexistente",
		"files": []string{missingFile},
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+convRes.ID+"/prompt", "application/json", bytes.NewReader(badReqBody))
	if err != nil {
		t.Fatalf("falha no request com arquivo inexistente: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para arquivo inexistente, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 4. Teste > 10 arquivos -> 400
	manyFiles := make([]string, 11)
	for i := range manyFiles {
		manyFiles[i] = txtPath
	}
	tooManyBody, _ := json.Marshal(map[string]interface{}{
		"text":  "Prompt com muitos arquivos",
		"files": manyFiles,
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+convRes.ID+"/prompt", "application/json", bytes.NewReader(tooManyBody))
	if err != nil {
		t.Fatalf("falha no request com >10 arquivos: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para >10 arquivos, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// 5. Teste válido com png e txt -> 200 + meta persistido
	validBody, _ := json.Marshal(map[string]interface{}{
		"text":  "Analise estes arquivos",
		"files": []string{pngPath, txtPath},
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+convRes.ID+"/prompt", "application/json", bytes.NewReader(validBody))
	if err != nil {
		t.Fatalf("falha no request valido: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", resp.StatusCode)
	}
	resp.Body.Close()

	// Verifica se a mensagem do usuário no store salvou os arquivos no meta
	msgs, err := st.GetMessages(convRes.ID)
	if err != nil || len(msgs) == 0 {
		t.Fatalf("mensagens não encontradas no store: %v", err)
	}
	userMsg := msgs[0]
	if userMsg.Role != "user" {
		t.Fatalf("primeira msg esperada user, obteve %s", userMsg.Role)
	}
	if userMsg.Meta == "" {
		t.Fatalf("esperava meta preenchido com anexos no userMsg, obteve vazio")
	}

	var metaFiles []string
	if err := json.Unmarshal([]byte(userMsg.Meta), &metaFiles); err != nil {
		t.Fatalf("falha ao deserializar metaFiles: %v (meta: %s)", err, userMsg.Meta)
	}
	if len(metaFiles) != 2 {
		t.Fatalf("esperava 2 arquivos no meta, obteve %d", len(metaFiles))
	}
	if !strings.Contains(metaFiles[0], "teste.png") || !strings.Contains(metaFiles[1], "dados.txt") {
		t.Fatalf("metaFiles incorreto: %v", metaFiles)
	}
}

func TestServer_PromptWithInvalidImage(t *testing.T) {
	agentBin := findFakeAgent(t)
	client, err := acp.NewClient(acp.ClientConfig{
		BinaryPath: agentBin,
		LogDir:     t.TempDir(),
	})
	if err != nil {
		t.Fatalf("falha ao instanciar client: %v", err)
	}
	defer client.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if _, err := client.Initialize(ctx); err != nil {
		t.Fatalf("falha no initialize: %v", err)
	}

	dbPath := filepath.Join(t.TempDir(), "prompt_invalid_img_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(client, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Criar conversa
	createBody, _ := json.Marshal(map[string]string{"cwd": t.TempDir()})
	resp, err := http.Post(ts.URL+"/api/conversations", "application/json", bytes.NewReader(createBody))
	if err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}
	var convRes struct {
		ID string `json:"id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&convRes)
	resp.Body.Close()

	// 2. Criar imagem inválida (8 bytes de assinatura PNG sem IHDR)
	tmpDir := t.TempDir()
	invalidPngPath := filepath.Join(tmpDir, "corrompida.png")
	invalidPngBytes := []byte{0x89, 'P', 'N', 'G', '\r', '\n', 0x1a, '\n'}
	if err := os.WriteFile(invalidPngPath, invalidPngBytes, 0644); err != nil {
		t.Fatalf("falha ao escrever imagem inválida: %v", err)
	}

	// 3. POST prompt com imagem inválida: deve falhar com 400 antes de chamar o agente
	badBody, _ := json.Marshal(map[string]interface{}{
		"text":  "Prompt com imagem quebrada",
		"files": []string{invalidPngPath},
	})
	resp, err = http.Post(ts.URL+"/api/conversations/"+convRes.ID+"/prompt", "application/json", bytes.NewReader(badBody))
	if err != nil {
		t.Fatalf("falha no request com imagem inválida: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("esperava 400 para imagem inválida, obteve %d", resp.StatusCode)
	}

	bodyBytes, _ := io.ReadAll(resp.Body)
	bodyStr := string(bodyBytes)
	expectedSub := "arquivo de imagem inválido: " + invalidPngPath
	if !strings.Contains(bodyStr, expectedSub) {
		t.Fatalf("esperava mensagem contendo %q, obteve: %q", expectedSub, bodyStr)
	}

	// Verifica que nada foi gravado no store para a conversa (não tocou o agente)
	msgs, _ := st.GetMessages(convRes.ID)
	if len(msgs) != 0 {
		t.Fatalf("esperava 0 mensagens salvas após rejeição de imagem inválida, obteve %d", len(msgs))
	}
}
