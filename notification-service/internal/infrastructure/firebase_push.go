package infrastructure

import (
	"context"
	"log/slog"
)

// MockFirebaseProvider logs notifications to stdout instead of actually sending them.
// In a real prod setup, this would initialize the Firebase Admin SDK.
type MockFirebaseProvider struct {
	logger *slog.Logger
}

func NewMockFirebaseProvider(logger *slog.Logger) *MockFirebaseProvider {
	return &MockFirebaseProvider{logger: logger}
}

func (p *MockFirebaseProvider) SendPush(ctx context.Context, tokens []string, title, body string, data map[string]string) error {
	p.logger.InfoContext(ctx, "[MOCK FIREBASE] Sending Push Notification",
		slog.Any("tokens", tokens),
		slog.String("title", title),
		slog.String("body", body),
	)
	// Here you would use firebase.google.com/go/v4/messaging
	// client.SendMulticast(...)
	return nil
}
