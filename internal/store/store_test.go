package store

import (
	"encoding/json"
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
		ModeID:       "default",
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
	if got.Title != "Nova Conversa" || got.ModelID != "gemini-2.5-flash" || got.ModeID != "default" {
		t.Fatalf("conversa recuperada incorreta: %+v", got)
	}

	// 3. Atualizar título, modelo e modo
	if err := st.UpdateConversationTitle("conv-1", "Título Atualizado"); err != nil {
		t.Fatalf("falha ao atualizar título: %v", err)
	}
	if err := st.UpdateConversationModel("conv-1", "gemini-2.5-pro"); err != nil {
		t.Fatalf("falha ao atualizar modelo: %v", err)
	}
	if err := st.UpdateConversationMode("conv-1", "accept-edits"); err != nil {
		t.Fatalf("falha ao atualizar modo: %v", err)
	}
	got, _ = st.GetConversation("conv-1")
	if got.Title != "Título Atualizado" || got.ModelID != "gemini-2.5-pro" || got.ModeID != "accept-edits" {
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
	_, err = st.UpsertToolCallMessage("conv-1", "tc-1", "pending", "Consultar Mapa", "read", "", map[string]interface{}{"loc": "DF"}, "")
	if err != nil {
		t.Fatalf("falha ao inserir tool call pending: %v", err)
	}
	_, err = st.UpsertToolCallMessage("conv-1", "tc-1", "completed", "Consultar Mapa", "read", "", map[string]interface{}{"loc": "DF"}, "")
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

func TestStore_ToolCall_UpsertAndIsolation(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test.db")
	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("erro ao abrir store: %v", err)
	}
	defer st.Close()

	if err := st.CreateConversation(&Conversation{
		ID:      "conv-1",
		Title:   "Test",
		Agent:   "agy",
		ModelID: "m1",
		CWD:     "/tmp",
	}); err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}

	// 1. Duas chamadas com IDs diferentes na mesma conversa -> 2 linhas role=tool_call
	_, err = st.UpsertToolCallMessage("conv-1", "tc-1", "in_progress", "Tool 1", "execute", "call_mcp_tool", map[string]interface{}{"cmd": "ls"}, "")
	if err != nil {
		t.Fatalf("falha tool 1: %v", err)
	}
	_, err = st.UpsertToolCallMessage("conv-1", "tc-2", "in_progress", "Tool 2", "execute", "call_mcp_tool", map[string]interface{}{"cmd": "pwd"}, "")
	if err != nil {
		t.Fatalf("falha tool 2: %v", err)
	}

	msgs, err := st.GetMessages("conv-1")
	if err != nil {
		t.Fatalf("falha ao buscar mensagens: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("esperava 2 mensagens role=tool_call, obteve %d", len(msgs))
	}

	// 2. Update de uma (tc-1) -> continuam 2 e só uma mudou, meta contém name e rawOutput, rawOutput vazio não vai no meta
	_, err = st.UpsertToolCallMessage("conv-1", "tc-1", "completed", "Tool 1", "execute", "call_mcp_tool", map[string]interface{}{"cmd": "ls"}, "output 1")
	if err != nil {
		t.Fatalf("falha update tool 1: %v", err)
	}

	msgs, err = st.GetMessages("conv-1")
	if err != nil {
		t.Fatalf("falha ao buscar mensagens: %v", err)
	}
	if len(msgs) != 2 {
		t.Fatalf("esperava 2 mensagens após update, obteve %d", len(msgs))
	}

	// Verificar mensagem tc-1 (tem rawOutput e name)
	var meta1 map[string]interface{}
	if err := json.Unmarshal([]byte(msgs[0].Meta), &meta1); err != nil {
		t.Fatalf("erro ao parsear meta 1: %v", err)
	}
	if meta1["toolCallId"] != "tc-1" || meta1["status"] != "completed" || meta1["name"] != "call_mcp_tool" || meta1["rawOutput"] != "output 1" {
		t.Fatalf("meta1 incorreto: %+v", meta1)
	}

	// Verificar mensagem tc-2 (rawOutput vazio não deve estar no meta)
	var meta2 map[string]interface{}
	if err := json.Unmarshal([]byte(msgs[1].Meta), &meta2); err != nil {
		t.Fatalf("erro ao parsear meta 2: %v", err)
	}
	if meta2["toolCallId"] != "tc-2" || meta2["status"] != "in_progress" || meta2["name"] != "call_mcp_tool" {
		t.Fatalf("meta2 incorreto: %+v", meta2)
	}
	if _, ok := meta2["rawOutput"]; ok {
		t.Fatalf("meta2 não deveria ter rawOutput quando vazio, obteve %v", meta2["rawOutput"])
	}

	// 3. Duas chamadas com ID vazio -> 2 linhas adicionais (sem colapso)
	mEmpty1, err := st.UpsertToolCallMessage("conv-1", "", "in_progress", "Empty 1", "execute", "tool_a", nil, "")
	if err != nil {
		t.Fatalf("falha empty 1: %v", err)
	}
	mEmpty2, err := st.UpsertToolCallMessage("conv-1", "", "in_progress", "Empty 2", "execute", "tool_b", nil, "")
	if err != nil {
		t.Fatalf("falha empty 2: %v", err)
	}

	msgs, err = st.GetMessages("conv-1")
	if err != nil {
		t.Fatalf("falha ao buscar mensagens: %v", err)
	}
	if len(msgs) != 4 {
		t.Fatalf("esperava 4 mensagens no total (2 vazias inseridas separadamente), obteve %d", len(msgs))
	}
	if mEmpty1.ID == mEmpty2.ID {
		t.Fatalf("mensagens com ID vazio não deveriam ter o mesmo ID de banco: %d == %d", mEmpty1.ID, mEmpty2.ID)
	}
}

func TestStore_ModeIdMigration(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "migration_test.db")

	// 1. Simula banco legado (v1) sem coluna mode_id
	db, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao inicializar store: %v", err)
	}
	db.Close()

	// 2. Reabre store (deve rodar migration se necessário)
	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao reabrir store com migration: %v", err)
	}
	defer st.Close()

	conv := &Conversation{
		ID:           "conv-migrated",
		Title:        "Conversa Migrada",
		Agent:        "agy",
		ModelID:      "gemini-2.5-flash",
		ModeID:       "accept-edits",
		CWD:          tmpDir,
		ACPSessionID: "sess-migrated",
		CreatedAt:    time.Now().UnixMilli(),
		UpdatedAt:    time.Now().UnixMilli(),
	}
	if err := st.CreateConversation(conv); err != nil {
		t.Fatalf("falha ao inserir conversa após migration: %v", err)
	}

	got, err := st.GetConversation("conv-migrated")
	if err != nil {
		t.Fatalf("falha ao obter conversa: %v", err)
	}
	if got.ModeID != "accept-edits" {
		t.Fatalf("esperava modeId 'accept-edits', obteve %q", got.ModeID)
	}
}

// TestStore_EmptyLists_NaoNulo garante que listagens em banco vazio retornam
// slices vazios (serializados como [] pelo JSON) e nunca null — causa de
// crash do frontend ("null is not an object") na primeira execução.
func TestStore_EmptyLists_NaoNulo(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "empty.db")

	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao abrir store: %v", err)
	}
	defer st.Close()

	convs, err := st.ListConversations()
	if err != nil {
		t.Fatalf("falha ao listar conversas: %v", err)
	}
	if convs == nil || len(convs) != 0 {
		t.Fatalf("ListConversations deveria retornar slice vazio não-nulo, obteve %v", convs)
	}

	ws, err := st.ListWorkspaces(10)
	if err != nil {
		t.Fatalf("falha ao listar workspaces: %v", err)
	}
	if ws == nil || len(ws) != 0 {
		t.Fatalf("ListWorkspaces deveria retornar slice vazio não-nulo, obteve %v", ws)
	}

	res, err := st.Search("qualquer-coisa", 10)
	if err != nil {
		t.Fatalf("falha ao buscar: %v", err)
	}
	if res == nil || len(res) != 0 {
		t.Fatalf("Search deveria retornar slice vazio não-nulo, obteve %v", res)
	}

	// Mensagens de conversa inexistente também devem vir vazias, não nulas
	msgs, err := st.GetMessages("conv-inexistente")
	if err != nil {
		t.Fatalf("falha ao listar mensagens: %v", err)
	}
	if msgs == nil || len(msgs) != 0 {
		t.Fatalf("GetMessages deveria retornar slice vazio não-nulo, obteve %v", msgs)
	}
}

// TestStore_DeleteMessagesFrom valida o truncamento usado por regenerar/editar
func TestStore_DeleteMessagesFrom(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "trunc.db")

	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao abrir store: %v", err)
	}
	defer st.Close()

	conv := &Conversation{
		ID: "conv-trunc", Title: "Trunc", Agent: "agy", ModelID: "m",
		CWD: tmpDir, ACPSessionID: "s", CreatedAt: 1, UpdatedAt: 1,
	}
	if err := st.CreateConversation(conv); err != nil {
		t.Fatalf("falha ao criar conversa: %v", err)
	}

	// user, assistant, user, assistant
	for _, role := range []string{"user", "assistant", "user", "assistant"} {
		if err := st.AppendMessage(&Message{ConversationID: "conv-trunc", Role: role, Content: role + "-msg", CreatedAt: 1}); err != nil {
			t.Fatalf("falha ao inserir mensagem: %v", err)
		}
	}

	msgs, _ := st.GetMessages("conv-trunc")
	if len(msgs) != 4 {
		t.Fatalf("esperava 4 mensagens, obteve %d", len(msgs))
	}

	// Trunca a partir do 3º (segundo user): restam só 2
	if err := st.DeleteMessagesFrom("conv-trunc", msgs[2].Seq); err != nil {
		t.Fatalf("falha ao truncar: %v", err)
	}
	msgs, _ = st.GetMessages("conv-trunc")
	if len(msgs) != 2 || msgs[0].Role != "user" || msgs[1].Role != "assistant" {
		t.Fatalf("esperava 2 mensagens após truncar, obteve %d: %+v", len(msgs), msgs)
	}

	// Trunca tudo
	if err := st.DeleteMessagesFrom("conv-trunc", 0); err != nil {
		t.Fatalf("falha ao truncar tudo: %v", err)
	}
	msgs, _ = st.GetMessages("conv-trunc")
	if len(msgs) != 0 {
		t.Fatalf("esperava 0 mensagens, obteve %d", len(msgs))
	}
}
