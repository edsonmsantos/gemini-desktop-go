package store

import (
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// SavedPrompt representa um prompt reutilizável salvo pelo usuário
type SavedPrompt struct {
	ID        string `json:"id"`
	Title     string `json:"title"`
	Content   string `json:"content"`
	CreatedAt int64  `json:"createdAt"`
	UpdatedAt int64  `json:"updatedAt"`
}

// migratePrompts cria a tabela saved_prompts e índices necessários
func (s *Store) migratePrompts() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `CREATE TABLE IF NOT EXISTS saved_prompts (
		id TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		content TEXT NOT NULL,
		created_at INTEGER,
		updated_at INTEGER
	);
	CREATE INDEX IF NOT EXISTS idx_saved_prompts_updated ON saved_prompts(updated_at DESC);`

	_, err := s.db.Exec(query)
	return err
}

// ListSavedPrompts retorna todos os prompts salvos ordenados por updated_at DESC.
// Retorna slice vazio não-nulo caso nenhum prompt exista.
func (s *Store) ListSavedPrompts() ([]*SavedPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, title, content, created_at, updated_at FROM saved_prompts ORDER BY updated_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*SavedPrompt, 0)
	for rows.Next() {
		var p SavedPrompt
		if err := rows.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt, &p.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return list, nil
}

// GetSavedPrompt busca um prompt salvo pelo seu ID
func (s *Store) GetSavedPrompt(id string) (*SavedPrompt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, title, content, created_at, updated_at FROM saved_prompts WHERE id = ?`
	row := s.db.QueryRow(query, id)

	var p SavedPrompt
	err := row.Scan(&p.ID, &p.Title, &p.Content, &p.CreatedAt, &p.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("prompt %s não encontrado", id)
	}
	if err != nil {
		return nil, err
	}
	return &p, nil
}

// SaveSavedPrompt insere se ID for vazio ou atualiza o registro caso contrário
func (s *Store) SaveSavedPrompt(p *SavedPrompt) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if p.ID == "" {
		p.ID = uuid.New().String()
		if p.CreatedAt == 0 {
			p.CreatedAt = now
		}
		if p.UpdatedAt == 0 {
			p.UpdatedAt = now
		}
		query := `INSERT INTO saved_prompts (id, title, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
		_, err := s.db.Exec(query, p.ID, p.Title, p.Content, p.CreatedAt, p.UpdatedAt)
		return err
	}

	p.UpdatedAt = now
	query := `UPDATE saved_prompts SET title = ?, content = ?, updated_at = ? WHERE id = ?`
	res, err := s.db.Exec(query, p.Title, p.Content, p.UpdatedAt, p.ID)
	if err != nil {
		return err
	}
	rows, err := res.RowsAffected()
	if err == nil && rows == 0 {
		// Se não existia, tenta inserir com o ID fornecido
		if p.CreatedAt == 0 {
			p.CreatedAt = now
		}
		insertQuery := `INSERT INTO saved_prompts (id, title, content, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`
		_, err = s.db.Exec(insertQuery, p.ID, p.Title, p.Content, p.CreatedAt, p.UpdatedAt)
		return err
	}
	return nil
}

// DeleteSavedPrompt remove o prompt pelo ID
func (s *Store) DeleteSavedPrompt(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM saved_prompts WHERE id = ?`, id)
	return err
}
