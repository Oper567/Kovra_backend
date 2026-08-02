package worker

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// PushNotificationService represents an interface to an external push provider (FCM/APNS)
type PushNotificationService interface {
	SendPushNotification(ctx context.Context, deviceToken, title, body string, data map[string]string) error
}

// RetentionWorker scans the database for inactive users and sends re-engagement pushes.
type RetentionWorker struct {
	db       *sql.DB
	pushSvc  PushNotificationService
	interval time.Duration
}

// NewRetentionWorker initializes a new retention worker.
func NewRetentionWorker(db *sql.DB, pushSvc PushNotificationService, interval time.Duration) *RetentionWorker {
	return &RetentionWorker{
		db:       db,
		pushSvc:  pushSvc,
		interval: interval,
	}
}

// Start runs the worker in the background using a ticker.
func (w *RetentionWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.interval)
	go func() {
		for {
			select {
			case <-ctx.Done():
				ticker.Stop()
				log.Println("Retention worker shutting down...")
				return
			case <-ticker.C:
				w.processInactiveUsers(ctx)
			}
		}
	}()
}

func (w *RetentionWorker) processInactiveUsers(ctx context.Context) {
	log.Println("Running retention cron job: Scanning for inactive users (> 48 hours)")

	// Find users who haven't logged in within the last 48 hours and haven't been sent a reminder recently
	query := `
		SELECT id, device_token 
		FROM users 
		WHERE last_login < NOW() - INTERVAL '48 hours' 
		  AND (last_retention_push IS NULL OR last_retention_push < NOW() - INTERVAL '24 hours')
		  AND device_token IS NOT NULL
	`

	rows, err := w.db.QueryContext(ctx, query)
	if err != nil {
		log.Printf("Error querying inactive users: %v", err)
		return
	}
	defer rows.Close()

	var sentCount int
	for rows.Next() {
		var id, deviceToken string
		if err := rows.Scan(&id, &deviceToken); err != nil {
			log.Printf("Error scanning user row: %v", err)
			continue
		}

		// Payload specifically designed to trigger the Kovi "Miss You" UI on the frontend
		err := w.pushSvc.SendPushNotification(
			ctx,
			deviceToken,
			"Kovi misses you! 🐍",
			"You almost broke our streak! Come back and claim your daily reward.",
			map[string]string{
				"action": "trigger_kovi_danger_streak",
			},
		)

		if err != nil {
			log.Printf("Failed to send push to user %s: %v", id, err)
			continue
		}

		// Update last_retention_push so we don't spam them
		_, _ = w.db.ExecContext(ctx, "UPDATE users SET last_retention_push = NOW() WHERE id = $1", id)
		sentCount++
	}

	log.Printf("Retention cron job complete. Sent %d re-engagement pushes.", sentCount)
}
