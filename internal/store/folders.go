package store

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// ErrFolderNotFound é retornado quando uma pasta especificada não é encontrada
var ErrFolderNotFound = errors.New("pasta não encontrada")

// FolderInfo representa o agrupamento de uma pasta e a contagem de conversas contidas
type FolderInfo struct {
	Name  string `json:"name"`
	Count int    `json:"count"`
}

// migrateFolders garante que a coluna folder exista na tabela conversations de forma idempotente
func (s *Store) migrateFolders() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.db.Exec(`ALTER TABLE conversations ADD COLUMN folder TEXT NOT NULL DEFAULT '';`)
	return nil
}

// SetConversationFolder define a pasta de uma conversa (com trim de espaços). Se folder for "", remove da pasta.
func (s *Store) SetConversationFolder(id, folder string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	folder = strings.TrimSpace(folder)
	now := time.Now().UnixMilli()

	res, err := s.db.Exec(`UPDATE conversations SET folder = ?, updated_at = ? WHERE id = ?`, folder, now, id)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return fmt.Errorf("conversa %s não encontrada", id)
	}

	return nil
}

// ListFolders retorna a lista de pastas com conversas agrupadas e suas contagens, ordenadas por count DESC
func (s *Store) ListFolders() ([]FolderInfo, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT folder, COUNT(*) as count FROM conversations WHERE folder != '' GROUP BY folder ORDER BY count DESC, folder ASC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]FolderInfo, 0)
	for rows.Next() {
		var f FolderInfo
		if err := rows.Scan(&f.Name, &f.Count); err != nil {
			return nil, err
		}
		list = append(list, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// ClearFolder remove a atribuição de pasta para todas as conversas da pasta especificada ("excluir pasta")
func (s *Store) ClearFolder(name string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	name = strings.TrimSpace(name)
	now := time.Now().UnixMilli()

	res, err := s.db.Exec(`UPDATE conversations SET folder = '', updated_at = ? WHERE folder = ?`, now, name)
	if err != nil {
		return err
	}

	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrFolderNotFound
	}

	return nil
}
