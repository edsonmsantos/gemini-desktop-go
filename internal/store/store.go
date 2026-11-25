package store

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
	"unicode/utf8"
)

// Conversation representa uma sessão de chat persistida
type Conversation struct {
	ID           string `json:"id"`
	Title        string `json:"title"`
	Agent        string `json:"agent"`
	ModelID      string `json:"modelId"`
	ModeID       string `json:"modeId,omitempty"`
	Folder       string `json:"folder"`
	CWD          string `json:"cwd"`
	ACPSessionID string `json:"sessionId"`
	CreatedAt    int64  `json:"createdAt"`
	UpdatedAt    int64  `json:"updatedAt"`
}

// Message representa uma entrada cronológica na conversa
type Message struct {
	ID             int64  `json:"id"`
	ConversationID string `json:"conversationId"`
	Seq            int    `json:"seq"`
	Role           string `json:"role"` // user, assistant, thought, tool_call, plan, permission, error
	Content        string `json:"content"`
	Meta           string `json:"meta,omitempty"`
	CreatedAt      int64  `json:"createdAt"`
}

// Workspace armazena os diretórios de trabalho recentes
type Workspace struct {
	CWD        string `json:"cwd"`
	LastUsedAt int64  `json:"lastUsedAt"`
}

// Store gerencia a persistência no banco SQLite
type Store struct {
	db *sql.DB
	mu sync.Mutex
}

// NewStore inicializa ou abre o banco de dados SQLite executando as migrations
func NewStore(dbPath string) (*Store, error) {
	if dbPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return nil, err
		}
		dbPath = filepath.Join(home, ".geminidesktop", "geminidesktop.db")
	}

	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("falha ao criar pasta do banco: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("falha ao abrir banco sqlite: %w", err)
	}

	// Ativa Foreign Keys e WAL mode
	if _, err := db.Exec("PRAGMA foreign_keys = ON; PRAGMA journal_mode = WAL;"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("falha ao configurar pragma sqlite: %w", err)
	}

	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("falha na migration do banco: %w", err)
	}
	if err := s.migrateFolders(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("falha na migration de folders: %w", err)
	}
	if err := s.migratePrompts(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("falha na migration de prompts: %w", err)
	}

	return s, nil
}

func (s *Store) migrate() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	queries := []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER PRIMARY KEY
		);`,
		`CREATE TABLE IF NOT EXISTS conversations (
			id TEXT PRIMARY KEY,
			title TEXT,
			agent TEXT,
			model_id TEXT,
			mode_id TEXT,
			cwd TEXT,
			acp_session_id TEXT,
			created_at INTEGER,
			updated_at INTEGER
		);`,
		`CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			conversation_id TEXT NOT NULL,
			seq INTEGER NOT NULL,
			role TEXT NOT NULL,
			content TEXT,
			meta TEXT,
			created_at INTEGER,
			FOREIGN KEY(conversation_id) REFERENCES conversations(id) ON DELETE CASCADE
		);`,
		`CREATE TABLE IF NOT EXISTS workspaces (
			cwd TEXT PRIMARY KEY,
			last_used_at INTEGER
		);`,
		`CREATE INDEX IF NOT EXISTS idx_messages_conv ON messages(conversation_id, seq);`,
		`CREATE INDEX IF NOT EXISTS idx_conv_updated ON conversations(updated_at DESC);`,
	}

	for _, q := range queries {
		if _, err := s.db.Exec(q); err != nil {
			return err
		}
	}

	var currentVersion int
	err := s.db.QueryRow(`SELECT COALESCE(MAX(version), 0) FROM schema_version`).Scan(&currentVersion)
	if err != nil {
		currentVersion = 0
	}

	if currentVersion < 1 {
		_, _ = s.db.Exec(`INSERT INTO schema_version (version) VALUES (1);`)
		currentVersion = 1
	}

	if currentVersion < 2 {
		// Garante que a coluna mode_id exista se a tabela conversations veio de versão anterior
		hasModeCol := false
		rows, err := s.db.Query(`PRAGMA table_info(conversations);`)
		if err == nil {
			for rows.Next() {
				var cid int
				var name, ctype string
				var notnull, pk int
				var dfltValue sql.NullString
				if err := rows.Scan(&cid, &name, &ctype, &notnull, &dfltValue, &pk); err == nil {
					if name == "mode_id" {
						hasModeCol = true
						break
					}
				}
			}
			rows.Close()
		}

		if !hasModeCol {
			if _, err := s.db.Exec(`ALTER TABLE conversations ADD COLUMN mode_id TEXT;`); err != nil {
				return fmt.Errorf("falha ao adicionar coluna mode_id: %w", err)
			}
		}

		if _, err := s.db.Exec(`INSERT INTO schema_version (version) VALUES (2);`); err != nil {
			return fmt.Errorf("falha ao atualizar schema_version: %w", err)
		}
	}

	return nil
}

// Close encerra a conexão com o banco
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.db.Close()
}

// CreateConversation insere uma nova conversa
func (s *Store) CreateConversation(c *Conversation) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	if c.CreatedAt == 0 {
		c.CreatedAt = now
	}
	if c.UpdatedAt == 0 {
		c.UpdatedAt = now
	}

	query := `INSERT INTO conversations (id, title, agent, model_id, mode_id, folder, cwd, acp_session_id, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`
	_, err := s.db.Exec(query, c.ID, c.Title, c.Agent, c.ModelID, c.ModeID, c.Folder, c.CWD, c.ACPSessionID, c.CreatedAt, c.UpdatedAt)
	return err
}

// GetConversation busca conversa pelo ID
func (s *Store) GetConversation(id string) (*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, title, agent, model_id, COALESCE(mode_id, ''), COALESCE(folder, ''), cwd, acp_session_id, created_at, updated_at
		FROM conversations WHERE id = ?`
	row := s.db.QueryRow(query, id)

	var c Conversation
	err := row.Scan(&c.ID, &c.Title, &c.Agent, &c.ModelID, &c.ModeID, &c.Folder, &c.CWD, &c.ACPSessionID, &c.CreatedAt, &c.UpdatedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("conversa %s não encontrada", id)
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteMessagesFrom remove todas as mensagens da conversa com seq >= fromSeq.
// Usado para truncar o histórico ao regenerar/editar o último turno.
func (s *Store) DeleteMessagesFrom(convID string, fromSeq int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM messages WHERE conversation_id = ? AND seq >= ?`, convID, fromSeq)
	return err
}

// ListConversations retorna todas as conversas ordenadas por updated_at desc
func (s *Store) ListConversations() ([]*Conversation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, title, agent, model_id, COALESCE(mode_id, ''), COALESCE(folder, ''), cwd, acp_session_id, created_at, updated_at
		FROM conversations ORDER BY updated_at DESC`
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*Conversation, 0)
	for rows.Next() {
		var c Conversation
		if err := rows.Scan(&c.ID, &c.Title, &c.Agent, &c.ModelID, &c.ModeID, &c.Folder, &c.CWD, &c.ACPSessionID, &c.CreatedAt, &c.UpdatedAt); err != nil {
			return nil, err
		}
		list = append(list, &c)
	}
	return list, nil
}

// UpdateConversationTitle renomeia o título da conversa
func (s *Store) UpdateConversationTitle(id, title string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `UPDATE conversations SET title = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, title, now, id)
	return err
}

// UpdateConversationModel altera o modelo da conversa
func (s *Store) UpdateConversationModel(id, modelID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `UPDATE conversations SET model_id = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, modelID, now, id)
	return err
}

// UpdateConversationMode altera o modo de operação da conversa
func (s *Store) UpdateConversationMode(id, modeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `UPDATE conversations SET mode_id = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, modeID, now, id)
	return err
}

// UpdateConversationACPSession atualiza o session ID do ACP associado
func (s *Store) UpdateConversationACPSession(id, acpSessionID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `UPDATE conversations SET acp_session_id = ?, updated_at = ? WHERE id = ?`
	_, err := s.db.Exec(query, acpSessionID, now, id)
	return err
}

// DeleteConversation remove a conversa e todas as suas mensagens associadas
func (s *Store) DeleteConversation(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, _ = s.db.Exec(`DELETE FROM messages WHERE conversation_id = ?`, id)
	_, err := s.db.Exec(`DELETE FROM conversations WHERE id = ?`, id)
	return err
}

// AppendMessage insere uma mensagem sequencial na conversa
func (s *Store) AppendMessage(m *Message) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if m.CreatedAt == 0 {
		m.CreatedAt = time.Now().UnixMilli()
	}

	var maxSeq sql.NullInt64
	_ = s.db.QueryRow(`SELECT MAX(seq) FROM messages WHERE conversation_id = ?`, m.ConversationID).Scan(&maxSeq)
	m.Seq = int(maxSeq.Int64) + 1

	query := `INSERT INTO messages (conversation_id, seq, role, content, meta, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`
	res, err := s.db.Exec(query, m.ConversationID, m.Seq, m.Role, m.Content, m.Meta, m.CreatedAt)
	if err != nil {
		return err
	}
	m.ID, _ = res.LastInsertId()

	// Atualiza updated_at da conversa
	_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, m.CreatedAt, m.ConversationID)
	return nil
}

// AppendOrUpdateChunk concatena chunks de streaming (assistant ou thought) na mesma mensagem
func (s *Store) AppendOrUpdateChunk(convID string, role string, chunkText string) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	// Procura a última mensagem da conversa (sem carregar o conteúdo: a
	// concatenação acontece no SQLite, evitando O(n²) em respostas longas)
	var lastID int64
	var lastRole, lastMeta string
	var lastSeq int
	err := s.db.QueryRow(`SELECT id, seq, role, meta FROM messages
		WHERE conversation_id = ? ORDER BY seq DESC LIMIT 1`, convID).Scan(&lastID, &lastSeq, &lastRole, &lastMeta)

	// Se a última mensagem for do mesmo role e o turno ainda não estiver finalizado
	isClosedTurn := false
	if lastMeta != "" {
		var metaMap map[string]interface{}
		if err := json.Unmarshal([]byte(lastMeta), &metaMap); err == nil {
			if _, ok := metaMap["stopReason"]; ok {
				isClosedTurn = true
			}
		}
	}

	if err == nil && lastRole == role && !isClosedTurn {
		// Concatena no SQLite (content = content || ?): não transfere nem
		// regrava o texto completo por chunk — custo por chunk vira O(chunk)
		_, err := s.db.Exec(`UPDATE messages SET content = content || ?, created_at = ? WHERE id = ?`, chunkText, now, lastID)
		if err != nil {
			return nil, err
		}
		_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, convID)
		return &Message{
			ID:             lastID,
			ConversationID: convID,
			Seq:            lastSeq,
			Role:           role,
			// Content não é recarregado ( callers ignoram; evita SELECT O(n) por chunk)
			Meta:      lastMeta,
			CreatedAt: now,
		}, nil
	}

	// Cria nova mensagem
	var maxSeq sql.NullInt64
	_ = s.db.QueryRow(`SELECT MAX(seq) FROM messages WHERE conversation_id = ?`, convID).Scan(&maxSeq)
	nextSeq := int(maxSeq.Int64) + 1

	res, err := s.db.Exec(`INSERT INTO messages (conversation_id, seq, role, content, meta, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`, convID, nextSeq, role, chunkText, "", now)
	if err != nil {
		return nil, err
	}
	id, _ := res.LastInsertId()
	_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, convID)

	return &Message{
		ID:             id,
		ConversationID: convID,
		Seq:            nextSeq,
		Role:           role,
		Content:        chunkText,
		CreatedAt:      now,
	}, nil
}

// UpsertToolCallMessage insere ou atualiza o status de uma chamada de ferramenta
func (s *Store) UpsertToolCallMessage(convID string, toolCallID, status, title, kind string, rawInput map[string]interface{}) (*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()

	metaObj := map[string]interface{}{
		"toolCallId": toolCallID,
		"status":     status,
		"title":      title,
		"kind":       kind,
		"rawInput":   rawInput,
	}
	metaBytes, _ := json.Marshal(metaObj)
	metaStr := string(metaBytes)

	// Procura se já existe mensagem deste toolCallId na conversa
	// Busca única via json_extract (evita N+1: SELECT de todos os tool_calls
	// + uma QueryRow de meta por linha)
	var id int64
	var seq int
	errLookup := s.db.QueryRow(`SELECT id, seq FROM messages
		WHERE conversation_id = ? AND role = 'tool_call'
		  AND json_extract(meta, '$.toolCallId') = ?
		LIMIT 1`, convID, toolCallID).Scan(&id, &seq)
	if errLookup == nil {
		_, _ = s.db.Exec(`UPDATE messages SET content = ?, meta = ? WHERE id = ?`, title, metaStr, id)
		_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, convID)
		return &Message{
			ID:             id,
			ConversationID: convID,
			Seq:            seq,
			Role:           "tool_call",
			Content:        title,
			Meta:           metaStr,
			CreatedAt:      now,
		}, nil
	}

	// Não encontrou, insere novo
	var maxSeq sql.NullInt64
	_ = s.db.QueryRow(`SELECT MAX(seq) FROM messages WHERE conversation_id = ?`, convID).Scan(&maxSeq)
	nextSeq := int(maxSeq.Int64) + 1

	res, err := s.db.Exec(`INSERT INTO messages (conversation_id, seq, role, content, meta, created_at)
		VALUES (?, ?, 'tool_call', ?, ?, ?)`, convID, nextSeq, title, metaStr, now)
	if err != nil {
		return nil, err
	}
	newID, _ := res.LastInsertId()
	_, _ = s.db.Exec(`UPDATE conversations SET updated_at = ? WHERE id = ?`, now, convID)

	return &Message{
		ID:             newID,
		ConversationID: convID,
		Seq:            nextSeq,
		Role:           "tool_call",
		Content:        title,
		Meta:           metaStr,
		CreatedAt:      now,
	}, nil
}

// UpdateAssistantTurnDone finaliza o turno anotando stopReason na última mensagem assistant
func (s *Store) UpdateAssistantTurnDone(convID string, stopReason string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var id int64
	var metaStr string
	err := s.db.QueryRow(`SELECT id, meta FROM messages WHERE conversation_id = ? AND role = 'assistant'
		ORDER BY seq DESC LIMIT 1`, convID).Scan(&id, &metaStr)
	if err != nil {
		return nil // nenhuma mensagem de assistant
	}

	metaMap := make(map[string]interface{})
	if metaStr != "" {
		_ = json.Unmarshal([]byte(metaStr), &metaMap)
	}
	metaMap["stopReason"] = stopReason

	newMetaBytes, _ := json.Marshal(metaMap)
	_, err = s.db.Exec(`UPDATE messages SET meta = ? WHERE id = ?`, string(newMetaBytes), id)
	return err
}

// GetMessages retorna todo o histórico de mensagens ordenado por seq ASC
func (s *Store) GetMessages(convID string) ([]*Message, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	query := `SELECT id, conversation_id, seq, role, content, meta, created_at
		FROM messages WHERE conversation_id = ? ORDER BY seq ASC`
	rows, err := s.db.Query(query, convID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*Message, 0)
	for rows.Next() {
		var m Message
		var meta sql.NullString
		if err := rows.Scan(&m.ID, &m.ConversationID, &m.Seq, &m.Role, &m.Content, &meta, &m.CreatedAt); err != nil {
			return nil, err
		}
		if meta.Valid {
			m.Meta = meta.String
		}
		list = append(list, &m)
	}
	return list, nil
}

// TouchWorkspace registra ou atualiza o timestamp de uso do workspace
func (s *Store) TouchWorkspace(cwd string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now().UnixMilli()
	query := `INSERT INTO workspaces (cwd, last_used_at) VALUES (?, ?)
		ON CONFLICT(cwd) DO UPDATE SET last_used_at = excluded.last_used_at`
	_, err := s.db.Exec(query, cwd, now)
	return err
}

// ListWorkspaces retorna os workspaces recentes ordenados por last_used_at desc
func (s *Store) ListWorkspaces(limit int) ([]*Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if limit <= 0 {
		limit = 10
	}

	query := `SELECT cwd, last_used_at FROM workspaces ORDER BY last_used_at DESC LIMIT ?`
	rows, err := s.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	list := make([]*Workspace, 0)
	for rows.Next() {
		var w Workspace
		if err := rows.Scan(&w.CWD, &w.LastUsedAt); err != nil {
			return nil, err
		}
		list = append(list, &w)
	}
	return list, nil
}

// SearchResult representa um resultado de busca agrupado por conversa
type SearchResult struct {
	ConversationID string `json:"conversationId"`
	Title          string `json:"title"`
	CWD            string `json:"cwd"`
	Snippet        string `json:"snippet"`
	UpdatedAt      int64  `json:"updatedAt"`
}

func makeSnippet(content, query string) string {
	lowerContent := strings.ToLower(content)
	lowerQuery := strings.ToLower(query)
	idx := strings.Index(lowerContent, lowerQuery)

	// Corta por RUNAS para não quebrar caracteres multibyte (acentos, emoji)
	runes := []rune(content)
	if idx == -1 {
		if len(runes) > 120 {
			return string(runes[:120]) + "..."
		}
		return content
	}

	matchRune := utf8.RuneCountInString(lowerContent[:idx])
	queryRunes := utf8.RuneCountInString(lowerQuery)

	start := matchRune - 60
	prefix := "..."
	if start <= 0 {
		start = 0
		prefix = ""
	}

	end := matchRune + queryRunes + 60
	suffix := "..."
	if end >= len(runes) {
		end = len(runes)
		suffix = ""
	}

	return prefix + string(runes[start:end]) + suffix
}

// Search realiza busca full-text LIKE case-insensitive em conversas e mensagens
func (s *Store) Search(query string, limit int) ([]SearchResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	trimmed := strings.TrimSpace(query)
	if trimmed == "" {
		return []SearchResult{}, nil
	}

	if limit <= 0 {
		limit = 40
	}

	likeParam := "%" + trimmed + "%"
	sqlQuery := `
		SELECT c.id, c.title, c.cwd, c.updated_at,
		       COALESCE(
		           (SELECT m.content FROM messages m WHERE m.conversation_id = c.id AND m.content LIKE ? ORDER BY m.seq ASC LIMIT 1),
		           c.title
		       ) AS match_content
		FROM conversations c
		WHERE c.title LIKE ? OR EXISTS (
		    SELECT 1 FROM messages m WHERE m.conversation_id = c.id AND m.content LIKE ?
		)
		ORDER BY c.updated_at DESC
		LIMIT ?;
	`

	rows, err := s.db.Query(sqlQuery, likeParam, likeParam, likeParam, limit)
	if err != nil {
		return nil, fmt.Errorf("falha ao buscar conversas: %w", err)
	}
	defer rows.Close()

	results := make([]SearchResult, 0)
	for rows.Next() {
		var r SearchResult
		var matchContent sql.NullString
		if err := rows.Scan(&r.ConversationID, &r.Title, &r.CWD, &r.UpdatedAt, &matchContent); err != nil {
			return nil, err
		}

		rawSnippetSource := r.Title
		if matchContent.Valid && matchContent.String != "" {
			rawSnippetSource = matchContent.String
		}

		r.Snippet = makeSnippet(rawSnippetSource, trimmed)
		results = append(results, r)
	}

	if results == nil {
		results = []SearchResult{}
	}

	return results, nil
}
