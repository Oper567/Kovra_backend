package repository

import (
	"context"
	"database/sql"
	"time"

	"github.com/lucepay-dev/lucepay/backend/notification-service/internal/domain"
)

type PostgresNotificationRepo struct {
	db *sql.DB
}

func NewPostgresNotificationRepo(db *sql.DB) *PostgresNotificationRepo {
	return &PostgresNotificationRepo{db: db}
}

func (r *PostgresNotificationRepo) RegisterDevice(ctx context.Context, device *domain.Device) error {
	query := `
		INSERT INTO user_devices (user_id, fcm_token, platform, is_active, updated_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (fcm_token) 
		DO UPDATE SET user_id = EXCLUDED.user_id, is_active = EXCLUDED.is_active, updated_at = EXCLUDED.updated_at
	`
	_, err := r.db.ExecContext(ctx, query,
		device.UserID, device.FCMToken, device.Platform, true, time.Now(),
	)
	return err
}

func (r *PostgresNotificationRepo) GetTokensByUser(ctx context.Context, userID string) ([]string, error) {
	query := `SELECT fcm_token FROM user_devices WHERE user_id = $1 AND is_active = true`
	rows, err := r.db.QueryContext(ctx, query, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tokens []string
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err != nil {
			return nil, err
		}
		tokens = append(tokens, t)
	}
	return tokens, nil
}

func (r *PostgresNotificationRepo) DeactivateToken(ctx context.Context, token string) error {
	_, err := r.db.ExecContext(ctx, `UPDATE user_devices SET is_active = false, updated_at = NOW() WHERE fcm_token = $1`, token)
	return err
}

func (r *PostgresNotificationRepo) LogNotification(ctx context.Context, log *domain.NotificationLog) error {
	query := `
		INSERT INTO notification_logs (user_id, title, body, type, is_delivered, error_msg, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
	`
	_, err := r.db.ExecContext(ctx, query,
		log.UserID, log.Title, log.Body, log.Type, log.IsDelivered, log.ErrorMsg, log.SentAt,
	)
	return err
}
