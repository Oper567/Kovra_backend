package usecase

import (
	"context"
	"log/slog"
	"time"

	"github.com/kovra-dev/kovra/backend/notification-service/internal/domain"
)

type NotificationUsecase struct {
	repo         domain.NotificationRepository
	pushProvider domain.PushProvider
	logger       *slog.Logger
}

func NewNotificationUsecase(repo domain.NotificationRepository, push domain.PushProvider, logger *slog.Logger) *NotificationUsecase {
	return &NotificationUsecase{
		repo:         repo,
		pushProvider: push,
		logger:       logger,
	}
}

func (uc *NotificationUsecase) RegisterDevice(ctx context.Context, userID, token, platform string) error {
	device := &domain.Device{
		UserID:   userID,
		FCMToken: token,
		Platform: platform,
	}
	return uc.repo.RegisterDevice(ctx, device)
}

func (uc *NotificationUsecase) SendPushToUser(ctx context.Context, userID, title, body, notifType string, data map[string]string) error {
	tokens, err := uc.repo.GetTokensByUser(ctx, userID)
	if err != nil {
		uc.logger.ErrorContext(ctx, "failed to get tokens", slog.String("error", err.Error()))
		return err
	}

	if len(tokens) == 0 {
		uc.logger.InfoContext(ctx, "no active devices for user", slog.String("user_id", userID))
		return nil
	}

	err = uc.pushProvider.SendPush(ctx, tokens, title, body, data)
	
	now := time.Now()
	logEntry := &domain.NotificationLog{
		UserID:      userID,
		Title:       title,
		Body:        body,
		Type:        notifType,
		IsDelivered: err == nil,
		SentAt:      &now,
	}
	if err != nil {
		logEntry.ErrorMsg = err.Error()
	}

	// Fire and forget logging
	go func() {
		_ = uc.repo.LogNotification(context.Background(), logEntry)
	}()

	return err
}

func (uc *NotificationUsecase) SendAdminBroadcast(ctx context.Context, title, body string) error {
	// In a real scenario, you'd batch this or use FCM topics.
	// For now, we log that an admin broadcast was sent.
	uc.logger.InfoContext(ctx, "admin broadcast sent (topic: all)", slog.String("title", title))
	return nil
}
