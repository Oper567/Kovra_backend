package domain

import (
	"context"
	"time"
)

type Device struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	FCMToken  string    `json:"fcm_token"`
	Platform  string    `json:"platform"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

type NotificationLog struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Type        string         `json:"type"` // 'wallet', 'admin', 'system'
	Data        map[string]any `json:"data"`
	IsDelivered bool           `json:"is_delivered"`
	ErrorMsg    string         `json:"error_msg,omitempty"`
	SentAt      *time.Time     `json:"sent_at,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
}

type NotificationRepository interface {
	RegisterDevice(ctx context.Context, device *Device) error
	GetTokensByUser(ctx context.Context, userID string) ([]string, error)
	DeactivateToken(ctx context.Context, token string) error
	LogNotification(ctx context.Context, log *NotificationLog) error
}

type PushProvider interface {
	SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) error
}
