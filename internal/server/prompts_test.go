package server

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"geminidesktop-go/internal/store"
)

func TestPrompts_NilSafeList(t *testing.T) {
	// Server sem store (nil-safe)
	srv := NewServerV2(nil, t.TempDir(), nil, nil)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/api/prompts")
	if err != nil {
		t.Fatalf("falha ao requisitar /api/prompts sem store: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 OK, obteve %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); !strings.Contains(ct, "application/json") {
		t.Fatalf("esperava Content-Type application/json, obteve %s", ct)
	}

	var list []store.SavedPrompt
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		t.Fatalf("falha ao decodificar JSON: %v", err)
	}
	if list == nil || len(list) != 0 {
		t.Fatalf("esperava slice vazio não nulo, obteve %+v", list)
	}
}

func TestPrompts_CRUDCycleAndValidation(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "prompts_server_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(nil, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. GET inicial -> lista vazia
	resp, err := http.Get(ts.URL + "/api/prompts")
	if err != nil {
		t.Fatalf("falha no GET /api/prompts: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200, obteve %d", resp.StatusCode)
	}
	var items []*store.SavedPrompt
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("falha decode: %v", err)
	}
	if len(items) != 0 {
		t.Fatalf("esperava 0 prompts, obteve %d", len(items))
	}

	// 2. Validações 400 no POST
	badPayloads := []string{
		`{"title": "", "content": "algo"}`,
		`{"title": "   ", "content": "algo"}`,
		`{"title": "algo", "content": ""}`,
		`{"title": "algo", "content": "   \n\t"}`,
		`invalid json`,
	}
	for _, bp := range badPayloads {
		res, err := http.Post(ts.URL+"/api/prompts", "application/json", strings.NewReader(bp))
		if err != nil {
			t.Fatalf("falha no POST inválido: %v", err)
		}
		res.Body.Close()
		if res.StatusCode != http.StatusBadRequest {
			t.Fatalf("esperava status 400 para payload %q, obteve %d", bp, res.StatusCode)
		}
	}

	// 3. POST válido (Create)
	createBody := map[string]string{
		"title":   "Prompt de Teste",
		"content": "Instruções do prompt de teste",
	}
	payloadBytes, _ := json.Marshal(createBody)
	postResp, err := http.Post(ts.URL+"/api/prompts", "application/json", bytes.NewReader(payloadBytes))
	if err != nil {
		t.Fatalf("falha no POST /api/prompts: %v", err)
	}
	defer postResp.Body.Close()
	if postResp.StatusCode != http.StatusOK && postResp.StatusCode != http.StatusCreated {
		t.Fatalf("esperava 200/201, obteve %d", postResp.StatusCode)
	}
	var created store.SavedPrompt
	if err := json.NewDecoder(postResp.Body).Decode(&created); err != nil {
		t.Fatalf("falha ao decodificar prompt criado: %v", err)
	}
	if created.ID == "" || created.Title != "Prompt de Teste" || created.Content != "Instruções do prompt de teste" {
		t.Fatalf("prompt criado inválido: %+v", created)
	}

	// 4. GET /api/prompts lista contendo 1 prompt
	listResp, err := http.Get(ts.URL + "/api/prompts")
	if err != nil {
		t.Fatalf("falha no GET após criação: %v", err)
	}
	defer listResp.Body.Close()
	var listAfter []*store.SavedPrompt
	if err := json.NewDecoder(listResp.Body).Decode(&listAfter); err != nil {
		t.Fatalf("falha decode listAfter: %v", err)
	}
	if len(listAfter) != 1 || listAfter[0].ID != created.ID {
		t.Fatalf("esperava 1 prompt com id %s, obteve %+v", created.ID, listAfter)
	}

	// 5. PUT /api/prompts/{id} (Update)
	updateBody := map[string]string{
		"title":   "Título Atualizado",
		"content": "Conteúdo Atualizado",
	}
	upBytes, _ := json.Marshal(updateBody)
	req, _ := http.NewRequest(http.MethodPut, ts.URL+"/api/prompts/"+created.ID, bytes.NewReader(upBytes))
	req.Header.Set("Content-Type", "application/json")
	putResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("falha no PUT: %v", err)
	}
	defer putResp.Body.Close()
	if putResp.StatusCode != http.StatusOK {
		t.Fatalf("esperava 200 no PUT, obteve %d", putResp.StatusCode)
	}
	var updated store.SavedPrompt
	if err := json.NewDecoder(putResp.Body).Decode(&updated); err != nil {
		t.Fatalf("falha ao decodificar prompt atualizado: %v", err)
	}
	if updated.Title != "Título Atualizado" || updated.Content != "Conteúdo Atualizado" {
		t.Fatalf("dados não atualizados: %+v", updated)
	}

	// 6. DELETE /api/prompts/{id}
	delReq, _ := http.NewRequest(http.MethodDelete, ts.URL+"/api/prompts/"+created.ID, nil)
	delResp, err := http.DefaultClient.Do(delReq)
	if err != nil {
		t.Fatalf("falha no DELETE: %v", err)
	}
	defer delResp.Body.Close()
	if delResp.StatusCode != http.StatusOK && delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("esperava 200/204 no DELETE, obteve %d", delResp.StatusCode)
	}

	// 7. GET final deve estar vazio
	finalResp, err := http.Get(ts.URL + "/api/prompts")
	if err != nil {
		t.Fatalf("falha no GET final: %v", err)
	}
	defer finalResp.Body.Close()
	var finalList []*store.SavedPrompt
	if err := json.NewDecoder(finalResp.Body).Decode(&finalList); err != nil {
		t.Fatalf("falha decode finalList: %v", err)
	}
	if len(finalList) != 0 {
		t.Fatalf("esperava lista vazia após delete, obteve %d", len(finalList))
	}
}

func TestPrompts_CurlCycle(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "curl_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao criar store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(nil, t.TempDir(), nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	// 1. POST via curl (Create)
	createCmd := exec.Command("curl", "-s", "-X", "POST",
		"-H", "Content-Type: application/json",
		"-d", `{"title":"Curl Title","content":"Curl Content"}`,
		ts.URL+"/api/prompts")
	out, err := createCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("falha no curl POST: %v, out: %s", err, string(out))
	}
	var created store.SavedPrompt
	if err := json.Unmarshal(out, &created); err != nil {
		t.Fatalf("falha ao parsear json criado via curl: %v, out: %s", err, string(out))
	}
	if created.ID == "" || created.Title != "Curl Title" {
		t.Fatalf("curl post criou incorretamente: %+v", created)
	}

	// 2. GET via curl (List)
	listCmd := exec.Command("curl", "-s", ts.URL+"/api/prompts")
	out, err = listCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("falha no curl GET: %v, out: %s", err, string(out))
	}
	var list []*store.SavedPrompt
	if err := json.Unmarshal(out, &list); err != nil || len(list) != 1 {
		t.Fatalf("falha na lista via curl: %v, len: %d, out: %s", err, len(list), string(out))
	}

	// 3. PUT via curl (Update)
	putCmd := exec.Command("curl", "-s", "-X", "PUT",
		"-H", "Content-Type: application/json",
		"-d", `{"title":"Curl Updated","content":"Curl Content Updated"}`,
		ts.URL+"/api/prompts/"+created.ID)
	out, err = putCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("falha no curl PUT: %v, out: %s", err, string(out))
	}
	var updated store.SavedPrompt
	if err := json.Unmarshal(out, &updated); err != nil || updated.Title != "Curl Updated" {
		t.Fatalf("falha no update via curl: %v, out: %s", err, string(out))
	}

	// 4. DELETE via curl (Delete)
	delCmd := exec.Command("curl", "-s", "-X", "DELETE", ts.URL+"/api/prompts/"+created.ID)
	out, err = delCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("falha no curl DELETE: %v, out: %s", err, string(out))
	}

	// 5. GET final
	finalCmd := exec.Command("curl", "-s", ts.URL+"/api/prompts")
	out, err = finalCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("falha no curl GET final: %v, out: %s", err, string(out))
	}
	var finalList []*store.SavedPrompt
	if err := json.Unmarshal(out, &finalList); err != nil || len(finalList) != 0 {
		t.Fatalf("esperava lista vazia após delete no curl, out: %s", string(out))
	}
}

