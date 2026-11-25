package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore_MigrationsAndCRUD(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")

	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao abrir store: %v", err)
	}
	defer st.Close()

	// 1. Criar conversa
	conv := &Conversation{
		ID:           "conv-1",
		Title:        "Nova Conversa",
		Agent:        "agy",
		ModelID:      "gemini-2.5-flash",
		CWD:          tmpDir,
		ACPSessionID: "sess-123",
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	}
	if err := st.CreateConversation(conv); err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}

	// 2. Buscar conversa
	got, err := st.GetConversation("conv-1")
	if err != nil {
		t.Fatalf("falha ao buscar conversa: %v", err)
	}
	if got.Title != "Nova Conversa" || got.ModelID != "gemini-2.5-flash" {
		t.Fatalf("conversa recuperada incorreta: %+v", got)
	}

	// 3. Atualizar título e modelo
	if err := st.UpdateConversationTitle("conv-1", "Título Atualizado"); err != nil {
		t.Fatalf("falha ao atualizar título: %v", err)
	}
	if err := st.UpdateConversationModel("conv-1", "gemini-2.5-pro"); err != nil {
		t.Fatalf("falha ao atualizar modelo: %v", err)
	}
	got, _ = st.GetConversation("conv-1")
	if got.Title != "Título Atualizado" || got.ModelID != "gemini-2.5-pro" {
		t.Fatalf("dados não atualizados: %+v", got)
	}

	// 4. Listar conversas
	list, err := st.ListConversations()
	if err != nil {
		t.Fatalf("falha ao listar conversas: %v", err)
	}
	if len(list) != 1 || list[0].ID != "conv-1" {
		t.Fatalf("lista de conversas incorreta: %v", list)
	}

	// 5. Inserir mensagens
	// Mensagem do usuário
	userMsg := &Message{
		ConversationID: "conv-1",
		Role:           "user",
		Content:        "Qual é a capital do Brasil?",
		CreatedAt:      time.Now().UnixMilli(),
	}
	if err := st.AppendMessage(userMsg); err != nil {
		t.Fatalf("falha ao adicionar mensagem do usuário: %v", err)
	}

	// Chunks de assistant concatenados
	if _, err := st.AppendOrUpdateChunk("conv-1", "assistant", "A capital "); err != nil {
		t.Fatalf("falha chunk 1: %v", err)
	}
	if _, err := st.AppendOrUpdateChunk("conv-1", "assistant", "do Brasil "); err != nil {
		t.Fatalf("falha chunk 2: %v", err)
	}
	if _, err := st.AppendOrUpdateChunk("conv-1", "assistant", "é Brasília."); err != nil {
		t.Fatalf("falha chunk 3: %v", err)
	}

	// Finaliza turno com turn_done
	if err := st.UpdateAssistantTurnDone("conv-1", "end_turn"); err != nil {
		t.Fatalf("falha ao marcar turn_done: %v", err)
	}

	// Tool call upsert
	_, err = st.UpsertToolCallMessage("conv-1", "tc-1", "pending", "Consultar Mapa", "read", map[string]interface{}{"loc": "DF"})
	if err != nil {
		t.Fatalf("falha ao inserir tool call pending: %v", err)
	}
	_, err = st.UpsertToolCallMessage("conv-1", "tc-1", "completed", "Consultar Mapa", "read", map[string]interface{}{"loc": "DF"})
	if err != nil {
		t.Fatalf("falha ao atualizar tool call completed: %v", err)
	}

	msgs, err := st.GetMessages("conv-1")
	if err != nil {
		t.Fatalf("falha ao buscar mensagens: %v", err)
	}
	// Deve haver: 1 user, 1 assistant (com todo texto concatenado), 1 tool_call
	if len(msgs) != 3 {
		t.Fatalf("esperava 3 mensagens (user, assistant, tool_call), obteve %d", len(msgs))
	}
	if msgs[1].Content != "A capital do Brasil é Brasília." {
		t.Fatalf("texto concatenado incorreto: %q", msgs[1].Content)
	}

	// 6. Workspaces
	if err := st.TouchWorkspace(tmpDir); err != nil {
		t.Fatalf("falha ao registrar workspace: %v", err)
	}
	wsList, err := st.ListWorkspaces(10)
	if err != nil {
		t.Fatalf("falha ao listar workspaces: %v", err)
	}
	if len(wsList) != 1 || wsList[0].CWD != tmpDir {
		t.Fatalf("workspaces incorretos: %+v", wsList)
	}

	// 7. Deletar conversa
	if err := st.DeleteConversation("conv-1"); err != nil {
		t.Fatalf("falha ao deletar conversa: %v", err)
	}
	msgsAfter, _ := st.GetMessages("conv-1")
	if len(msgsAfter) != 0 {
		t.Fatalf("esperava 0 mensagens após deletar conversa, obteve %d", len(msgsAfter))
	}
}
