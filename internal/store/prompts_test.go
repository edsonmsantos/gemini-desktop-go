package store

import (
	"path/filepath"
	"testing"
	"time"
)

func TestSavedPrompts_CRUDAndEmptySlice(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_prompts.db")

	st, err := NewStore(dbPath)
	if err != nil {
		t.Fatalf("falha ao abrir store: %v", err)
	}
	defer st.Close()

	// 1. Lista inicial deve retornar slice vazio NÃO-NULO
	initialList, err := st.ListSavedPrompts()
	if err != nil {
		t.Fatalf("falha ao listar prompts inicialmente: %v", err)
	}
	if initialList == nil {
		t.Fatalf("ListSavedPrompts deve retornar slice não-nulo, obteve nil")
	}
	if len(initialList) != 0 {
		t.Fatalf("esperava 0 prompts, obteve %d", len(initialList))
	}

	// 2. Criar novo prompt (ID vazio -> deve gerar uuid e timestamps)
	p1 := &SavedPrompt{
		Title:   "Prompt 1",
		Content: "Conteúdo do prompt 1",
	}
	if err := st.SaveSavedPrompt(p1); err != nil {
		t.Fatalf("falha ao salvar prompt 1: %v", err)
	}
	if p1.ID == "" {
		t.Fatalf("SaveSavedPrompt deveria ter gerado um ID")
	}
	if p1.CreatedAt == 0 || p1.UpdatedAt == 0 {
		t.Fatalf("SaveSavedPrompt deveria ter preenchido CreatedAt e UpdatedAt")
	}

	// 3. Buscar prompt por ID
	got, err := st.GetSavedPrompt(p1.ID)
	if err != nil {
		t.Fatalf("falha ao buscar prompt por id: %v", err)
	}
	if got.Title != "Prompt 1" || got.Content != "Conteúdo do prompt 1" {
		t.Fatalf("prompt recuperado diferente: %+v", got)
	}

	// Criar segundo prompt após pequena pausa para diferenciar updated_at
	time.Sleep(10 * time.Millisecond)
	p2 := &SavedPrompt{
		Title:   "Prompt 2",
		Content: "Conteúdo do prompt 2",
	}
	if err := st.SaveSavedPrompt(p2); err != nil {
		t.Fatalf("falha ao salvar prompt 2: %v", err)
	}

	// 4. Listar prompts ordenados por updated_at DESC (p2 deve vir antes de p1)
	list, err := st.ListSavedPrompts()
	if err != nil {
		t.Fatalf("falha ao listar prompts: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("esperava 2 prompts na lista, obteve %d", len(list))
	}
	if list[0].ID != p2.ID || list[1].ID != p1.ID {
		t.Fatalf("ordem esperada [p2, p1], obteve [%s, %s]", list[0].ID, list[1].ID)
	}

	// 5. Update do prompt 1 (atualiza title, content e updated_at)
	p1Updated := &SavedPrompt{
		ID:      p1.ID,
		Title:   "Prompt 1 Modificado",
		Content: "Conteúdo modificado",
	}
	time.Sleep(10 * time.Millisecond)
	if err := st.SaveSavedPrompt(p1Updated); err != nil {
		t.Fatalf("falha ao atualizar prompt 1: %v", err)
	}

	gotUpdated, err := st.GetSavedPrompt(p1.ID)
	if err != nil {
		t.Fatalf("falha ao buscar prompt 1 atualizado: %v", err)
	}
	if gotUpdated.Title != "Prompt 1 Modificado" || gotUpdated.Content != "Conteúdo modificado" {
		t.Fatalf("conteúdo atualizado incorreto: %+v", gotUpdated)
	}
	if gotUpdated.UpdatedAt <= p1.UpdatedAt {
		t.Fatalf("UpdatedAt deveria ter sido atualizado")
	}

	// Agora p1 foi atualizado por último, deve vir em primeiro na lista
	listAfterUpdate, err := st.ListSavedPrompts()
	if err != nil {
		t.Fatalf("falha ao listar prompts após update: %v", err)
	}
	if listAfterUpdate[0].ID != p1.ID {
		t.Fatalf("esperava p1 em primeiro após update, obteve %s", listAfterUpdate[0].ID)
	}

	// 6. Delete
	if err := st.DeleteSavedPrompt(p1.ID); err != nil {
		t.Fatalf("falha ao deletar prompt: %v", err)
	}
	_, err = st.GetSavedPrompt(p1.ID)
	if err == nil {
		t.Fatalf("esperava erro ao buscar prompt deletado, mas não ocorreu")
	}

	listAfterDelete, err := st.ListSavedPrompts()
	if err != nil {
		t.Fatalf("falha ao listar prompts após delete: %v", err)
	}
	if len(listAfterDelete) != 1 || listAfterDelete[0].ID != p2.ID {
		t.Fatalf("esperava apenas p2 na lista após delete")
	}
}
