package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestStore_Folders(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_folders.db")

	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao abrir store: %v", err)
	}
	defer st.Close()

	// 1. Criar conversas iniciais (sem pasta)
	conv1 := &Conversation{
		ID:        "c1",
		Title:     "Conversa 1",
		Agent:     "agy",
		ModelID:   "gemini-2.5-flash",
		CWD:       tmpDir,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	conv2 := &Conversation{
		ID:        "c2",
		Title:     "Conversa 2",
		Agent:     "agy",
		ModelID:   "gemini-2.5-flash",
		CWD:       tmpDir,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}
	conv3 := &Conversation{
		ID:        "c3",
		Title:     "Conversa 3",
		Agent:     "agy",
		ModelID:   "gemini-2.5-flash",
		CWD:       tmpDir,
		CreatedAt: time.Now().UnixMilli(),
		UpdatedAt: time.Now().UnixMilli(),
	}

	if err := st.CreateConversation(conv1); err != nil {
		t.Fatalf("falha ao criar conv1: %v", err)
	}
	if err := st.CreateConversation(conv2); err != nil {
		t.Fatalf("falha ao criar conv2: %v", err)
	}
	if err := st.CreateConversation(conv3); err != nil {
		t.Fatalf("falha ao criar conv3: %v", err)
	}

	// Conversas recém criadas devem ter folder = ""
	c1Get, err := st.GetConversation("c1")
	if err != nil {
		t.Fatalf("falha ao buscar c1: %v", err)
	}
	if c1Get.Folder != "" {
		t.Fatalf("esperado folder vazio, obteve %q", c1Get.Folder)
	}

	// ListFolders inicial deve ser vazio (conversas sem pasta não aparecem)
	folders, err := st.ListFolders()
	if err != nil {
		t.Fatalf("falha ao listar pastas: %v", err)
	}
	if len(folders) != 0 {
		t.Fatalf("esperava 0 pastas, obteve %d", len(folders))
	}

	// 2. Mover conversas para pastas (com trim de espaços)
	if err := st.SetConversationFolder("c1", "  Projetos  "); err != nil {
		t.Fatalf("falha ao definir pasta de c1: %v", err)
	}
	if err := st.SetConversationFolder("c2", "Projetos"); err != nil {
		t.Fatalf("falha ao definir pasta de c2: %v", err)
	}
	if err := st.SetConversationFolder("c3", "Estudos"); err != nil {
		t.Fatalf("falha ao definir pasta de c3: %v", err)
	}

	// Verificar se c1 foi salvo com folder trimado "Projetos"
	c1Get, err = st.GetConversation("c1")
	if err != nil {
		t.Fatalf("falha ao buscar c1: %v", err)
	}
	if c1Get.Folder != "Projetos" {
		t.Fatalf("esperava folder 'Projetos', obteve %q", c1Get.Folder)
	}

	// 3. ListFolders com contagens e ordenação por contagem DESC
	folders, err = st.ListFolders()
	if err != nil {
		t.Fatalf("falha ao listar pastas: %v", err)
	}
	if len(folders) != 2 {
		t.Fatalf("esperava 2 pastas, obteve %d", len(folders))
	}
	if folders[0].Name != "Projetos" || folders[0].Count != 2 {
		t.Fatalf("esperava Projetos com count 2, obteve %+v", folders[0])
	}
	if folders[1].Name != "Estudos" || folders[1].Count != 1 {
		t.Fatalf("esperava Estudos com count 1, obteve %+v", folders[1])
	}

	// Também checar em ListConversations se o folder é retornado
	list, err := st.ListConversations()
	if err != nil {
		t.Fatalf("falha ao listar conversas: %v", err)
	}
	for _, conv := range list {
		if conv.ID == "c1" && conv.Folder != "Projetos" {
			t.Fatalf("c1 na listagem deveria ter folder 'Projetos', obteve %q", conv.Folder)
		}
	}

	// 4. ClearFolder: "excluir pasta" zera o folder das conversas associadas
	if err := st.ClearFolder("Projetos"); err != nil {
		t.Fatalf("falha ao limpar pasta Projetos: %v", err)
	}

	// ListFolders agora deve ter apenas "Estudos"
	folders, err = st.ListFolders()
	if err != nil {
		t.Fatalf("falha ao listar pastas: %v", err)
	}
	if len(folders) != 1 || folders[0].Name != "Estudos" {
		t.Fatalf("esperava apenas pasta Estudos, obteve %+v", folders)
	}

	// c1 e c2 devem agora ter folder = ""
	c1Get, err = st.GetConversation("c1")
	if err != nil {
		t.Fatalf("falha ao buscar c1: %v", err)
	}
	if c1Get.Folder != "" {
		t.Fatalf("esperava folder vazio para c1 após ClearFolder, obteve %q", c1Get.Folder)
	}

	// 5. Testar migração idempotente (reabrir store)
	st.Close()
	st2, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao reabrir store: %v", err)
	}
	defer st2.Close()

	folders, err = st2.ListFolders()
	if err != nil {
		t.Fatalf("falha ao listar pastas após reabertura: %v", err)
	}
	if len(folders) != 1 || folders[0].Name != "Estudos" {
		t.Fatalf("esperava 1 pasta Estudos após reabertura, obteve %+v", folders)
	}
}
