package server

import (
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

type searchAPIResponse struct {
	Results []store.SearchResult `json:"results"`
}

func TestServer_Search_ContentAndTitle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "search_test.db")
	st, err := store.NewStore(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	defer st.Close()

	srv := NewServerV2(nil, tempDir, nil, st)
	ts := httptest.NewServer(srv.Handler())
	defer ts.Close()

	now := time.Now().UnixMilli()

	// Conversa 1: match no content
	conv1 := &store.Conversation{
		ID:        "conv-1",
		Title:     "Projeto Alpha",
		CWD:       "/path/alpha",
		CreatedAt: now - 1000,
		UpdatedAt: now - 1000,
	}
	_ = st.CreateConversation(conv1)
	longText := strings.Repeat("palavras antes ", 10) + "CHAVE_SECRETA_ALPHA" + strings.Repeat(" palavras depois", 10)
	_ = st.AppendMessage(&store.Message{
		ConversationID: conv1.ID,
		Seq:            1,
		Role:           "user",
		Content:        longText,
		CreatedAt:      now - 1000,
	})

	// Conversa 2: match no title
	conv2 := &store.Conversation{
		ID:        "conv-2",
		Title:     "Conversa com CHAVE_SECRETA_TITLE no título",
		CWD:       "/path/beta",
		CreatedAt: now,
		UpdatedAt: now, // Mais recente
	}
	_ = st.CreateConversation(conv2)
	_ = st.AppendMessage(&store.Message{
		ConversationID: conv2.ID,
		Seq:            1,
		Role:           "user",
		Content:        "nada demais aqui",
		CreatedAt:      now,
	})

	// 1. Busca vazia -> lista vazia
	resEmpty, err := http.Get(ts.URL + "/api/search?q=")
	if err != nil {
		t.Fatalf("search empty failed: %v", err)
	}
	var emptyResp searchAPIResponse
	_ = json.NewDecoder(resEmpty.Body).Decode(&emptyResp)
	resEmpty.Body.Close()
	if len(emptyResp.Results) != 0 {
		t.Fatalf("expected 0 results for empty query, got %d", len(emptyResp.Results))
	}

	// 2. Busca por "CHAVE_SECRETA_ALPHA" -> deve achar conv-1 com snippet truncado
	resContent, err := http.Get(ts.URL + "/api/search?q=" + url.QueryEscape("chave_secreta_alpha"))
	if err != nil {
		t.Fatalf("search content failed: %v", err)
	}
	var contentResp searchAPIResponse
	_ = json.NewDecoder(resContent.Body).Decode(&contentResp)
	resContent.Body.Close()

	if len(contentResp.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(contentResp.Results))
	}
	item1 := contentResp.Results[0]
	if item1.ConversationID != "conv-1" || item1.Title != "Projeto Alpha" {
		t.Errorf("unexpected item1: %+v", item1)
	}
	if !strings.Contains(item1.Snippet, "CHAVE_SECRETA_ALPHA") {
		t.Errorf("snippet does not contain query: %q", item1.Snippet)
	}
	if !strings.HasPrefix(item1.Snippet, "...") || !strings.HasSuffix(item1.Snippet, "...") {
		t.Errorf("snippet should have ellipsis: %q", item1.Snippet)
	}

	// 3. Busca por "CHAVE_SECRETA" -> deve achar ambas ordenadas por updatedAt DESC (conv2 primeiro)
	resBoth, err := http.Get(ts.URL + "/api/search?q=" + url.QueryEscape("chave_secreta"))
	if err != nil {
		t.Fatalf("search both failed: %v", err)
	}
	var bothResp searchAPIResponse
	_ = json.NewDecoder(resBoth.Body).Decode(&bothResp)
	resBoth.Body.Close()

	if len(bothResp.Results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(bothResp.Results))
	}
	if bothResp.Results[0].ConversationID != "conv-2" {
		t.Errorf("expected conv-2 first due to updatedAt DESC, got %s", bothResp.Results[0].ConversationID)
	}
	if bothResp.Results[1].ConversationID != "conv-1" {
		t.Errorf("expected conv-1 second, got %s", bothResp.Results[1].ConversationID)
	}

	// 4. Busca sem match -> lista vazia
	resNone, err := http.Get(ts.URL + "/api/search?q=nao_existe_xyz_123")
	if err != nil {
		t.Fatalf("search none failed: %v", err)
	}
	var noneResp searchAPIResponse
	_ = json.NewDecoder(resNone.Body).Decode(&noneResp)
	resNone.Body.Close()

	if len(noneResp.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(noneResp.Results))
	}
}
