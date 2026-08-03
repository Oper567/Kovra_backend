package postgres

import (
	"context"
	"database/sql"

	"github.com/lucepay-dev/lucepay/backend/ai-service/internal/domain"
	"github.com/lucepay-dev/lucepay/backend/ai-service/internal/usecase"
)

type chatRepo struct {
	db *sql.DB
}

func NewChatRepository(db *sql.DB) usecase.ChatRepository {
	return &chatRepo{db: db}
}

func (r *chatRepo) CreateSession(ctx context.Context, userID, title string) (*domain.ChatSession, error) {
	query := `
		INSERT INTO ai_chat_sessions (user_id, title)
		VALUES ($1, $2)
		RETURNING id, is_active, created_at`
	
	session := &domain.ChatSession{
		UserID: userID,
		Title:  title,
	}

	err := r.db.QueryRowContext(ctx, query, userID, title).Scan(&session.ID, &session.IsActive, &session.CreatedAt)
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *chatRepo) GetSession(ctx context.Context, sessionID string) (*domain.ChatSession, error) {
	query := `
		SELECT id, user_id, title, is_active, created_at
		FROM ai_chat_sessions
		WHERE id = $1`

	session := &domain.ChatSession{}
	err := r.db.QueryRowContext(ctx, query, sessionID).Scan(
		&session.ID, &session.UserID, &session.Title, &session.IsActive, &session.CreatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, domain.ErrSessionNotFound
	}
	if err != nil {
		return nil, err
	}
	return session, nil
}

func (r *chatRepo) ListSessions(ctx context.Context, userID string) ([]*domain.ChatSession, error) {
	query := `
		SELECT id, user_id, title, is_active, created_at
		FROM ai_chat_sessions
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 20`

	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []*domain.ChatSession
	for rows.Next() {
		var s domain.ChatSession
		if err := rows.Scan(&s.ID, &s.UserID, &s.Title, &s.IsActive, &s.CreatedAt); err != nil {
			return nil, err
		}
		sessions = append(sessions, &s)
	}
	return sessions, nil
}

func (r *chatRepo) AddMessage(ctx context.Context, msg *domain.ChatMessage) (*domain.ChatMessage, error) {
	query := `
		INSERT INTO ai_chat_messages (session_id, role, content, tokens_used)
		VALUES ($1, $2, $3, $4)
		RETURNING id, created_at`

	err := r.db.QueryRowContext(ctx, query, msg.SessionID, msg.Role, msg.Content, msg.TokensUsed).
		Scan(&msg.ID, &msg.CreatedAt)
	
	if err != nil {
		return nil, err
	}
	
	// Update session updated_at
	updateQ := `UPDATE ai_chat_sessions SET updated_at = NOW() WHERE id = $1`
	r.db.ExecContext(ctx, updateQ, msg.SessionID)

	return msg, nil
}

func (r *chatRepo) GetMessages(ctx context.Context, sessionID string, limit int) ([]*domain.ChatMessage, error) {
	query := `
		SELECT id, session_id, role, content, tokens_used, created_at
		FROM ai_chat_messages
		WHERE session_id = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.QueryContext(ctx, query, sessionID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []*domain.ChatMessage
	for rows.Next() {
		var m domain.ChatMessage
		if err := rows.Scan(&m.ID, &m.SessionID, &m.Role, &m.Content, &m.TokensUsed, &m.CreatedAt); err != nil {
			return nil, err
		}
		msgs = append(msgs, &m)
	}

	// Reverse to chronological order
	for i, j := 0, len(msgs)-1; i < j; i, j = i+1, j-1 {
		msgs[i], msgs[j] = msgs[j], msgs[i]
	}

	return msgs, nil
}
