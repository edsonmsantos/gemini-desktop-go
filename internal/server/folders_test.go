package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"geminidesktop-go/internal/store"
)

func TestFolders_NilSafeGet(t *testing.T) {
	srv := NewServerV2(nil, t.TempDir(), nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/folders")
	if err != nil {
		t.Fatalf("falha ao requisitar /api/folders sem store: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK, obteve %d", resp.StatusCode)
	}

	var data struct {
		Folders []store.FolderInfo `json:"folders"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("falha ao decodificar JSON: %v", err)
	}
	if data.Folders == nil || len(data.Folders) != 0 {
		t.Fatalf("esperava slice vazio não nulo, obteve %+v", data.Folders)
	}
}

func TestFolders_CycleAndValidation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "folders_server_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(nil, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. Criar 2 conversas no store
	c1 := &store.Conversation{
		ID:        "conv-1",
		Title:     "Conversa 1",
		Agent:     "agy",
		ModelID:   "gemini-2.5-flash",
		CWD:       t.TempDir(),
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	c2 := &store.Conversation{
		ID:        "conv-2",
		Title:     "Conversa 2",
		Agent:     "agy",
		ModelID:   "gemini-2.5-flash",
		CWD:       t.TempDir(),
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	if err := st.CreateConversation(c1); err != nil {
		t.Fatalf("falha ao criar conv-1: %v", err)
	}
	if err := st.CreateConversation(c2); err != nil {
		t.Fatalf("falha ao criar conv-2: %v", err)
	}

	// 2. PATCH em conversa inexistente -> 404
	patchReq, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/conversations/nao-existe/folder", strings.NewReader(`{"folder":"Projetos"}`))
	patchReq.Header.Set("Content-Type", "application/json")
	resp, err := http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("falha no PATCH conversa inexistente: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("esperava 404 para conversa inexistente, obteve %d", resp.StatusCode)
	}

	// 3. PATCH conv-1 para "Trabalho / Projetos" (com espaços e caracteres especiais)
	patchBody, _ := json.Marshal(map[string]string{"folder": "  Trabalho / Projetos  "})
	patchReq, _ = http.NewRequest(http.MethodPatch, ts.URL+"/api/conversations/conv-1/folder", bytes.NewReader(patchBody))
	patchReq.Header.Set("Content-Type", "application/json")
	resp, err = http.DefaultClient.Do(patchReq)
	if err != nil {
		t.Fatalf("falha no PATCH conv-1: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK no PATCH, obteve %d", resp.StatusCode)
	}
	var patchResp struct {
		ID     string `json:"id"`
		Folder string `json:"folder"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&patchResp); err != nil {
		t.Fatalf("falha decode patchResp: %v", err)
	}
	if patchResp.ID != "conv-1" || patchResp.Folder != "Trabalho / Projetos" {
		t.Fatalf("retorno inesperado no PATCH: %+v", patchResp)
	}

	// PATCH conv-2 para "Trabalho / Projetos"
	patchReq2, _ := http.NewRequest(http.MethodPatch, ts.URL+"/api/conversations/conv-2/folder", strings.NewReader(`{"folder":"Trabalho / Projetos"}`))
	patchReq2.Header.Set("Content-Type", "application/json")
	resp2, err := http.DefaultClient.Do(patchReq2)
	if err != nil {
		t.Fatalf("falha no PATCH conv-2: %v", err)
	}
	resp2.Body.Close()
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK no PATCH conv-2, obteve %d", resp2.StatusCode)
	}

	// 4. GET /api/folders -> {folders: [{name: "Trabalho / Projetos", count: 2}]}
	getResp, err := http.Get(ts.URL + "/api/folders")
	if err != nil {
		t.Fatalf("falha no GET /api/folders: %v", err)
	}
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK no GET /api/folders, obteve %d", getResp.StatusCode)
	}
	var foldersData struct {
		Folders []store.FolderInfo `json:"folders"`
	}
	if err := json.NewDecoder(getResp.Body).Decode(&foldersData); err != nil {
		t.Fatalf("falha decode foldersData: %v", err)
	}
	if len(foldersData.Folders) != 1 {
		t.Fatalf("esperava 1 pasta, obteve %d", len(foldersData.Folders))
	}
	if foldersData.Folders[0].Name != "Trabalho / Projetos" || foldersData.Folders[0].Count != 2 {
		t.Fatalf("pasta inesperada: %+v", foldersData.Folders[0])
	}

	// 5. GET /api/conversations deve retornar o campo folder
	convListResp, err := http.Get(ts.URL + "/api/conversations")
	if err != nil {
		t.Fatalf("falha no GET /api/conversations: %v", err)
	}
	defer convListResp.Body.Close()
	var convs []store.Conversation
	if err := json.NewDecoder(convListResp.Body).Decode(&convs); err != nil {
		t.Fatalf("falha decode conversas: %v", err)
	}
	for _, c := range convs {
		if c.Folder != "Trabalho / Projetos" {
			t.Fatalf("esperava folder 'Trabalho / Projetos' na listagem, obteve %q", c.Folder)
		}
	}

	// 6. DELETE /api/folders/{name} com pasta inexistente -> 404
	delReq404, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/folders/Inexistente", nil)
	delResp404, err := http.DefaultClient.Do(delReq404)
	if err != nil {
		t.Fatalf("falha no DELETE pasta inexistente: %v", err)
	}
	delResp404.Body.Close()
	if delResp404.StatusCode != http.StatusNotFound {
		t.Fatalf("esperava 404 no DELETE de pasta inexistente, obteve %d", delResp404.StatusCode)
	}

	// 7. DELETE /api/folders/{name} com URL escaped path
	escapedName := url.PathEscape("Trabalho / Projetos")
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/folders/"+escapedName, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("falha no DELETE pasta: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK no DELETE pasta, obteve %d", delResp.StatusCode)
	}

	// 8. GET /api/folders agora deve estar vazia
	getResp2, err := http.Get(ts.URL + "/api/folders")
	if err != nil {
		t.Fatalf("falha no GET /api/folders: %v", err)
	}
	defer getResp2.Body.Close()
	var foldersData2 struct {
		Folders []store.FolderInfo `json:"folders"`
	}
	_ = json.NewDecoder(getResp2.Body).Decode(&foldersData2)
	if len(foldersData2.Folders) != 0 {
		t.Fatalf("esperava 0 pastas restantes, obteve %d", len(foldersData2.Folders))
	}

	// 9. Conversas devem ter voltado a ter folder vazio
	c1Get, _ := st.GetConversation("conv-1")
	if c1Get.Folder != "" {
		t.Fatalf("c1 deveria ter folder vazio após DELETE da pasta, obteve %q", c1Get.Folder)
	}
}
