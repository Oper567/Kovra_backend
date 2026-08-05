package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/lucepay-dev/lucepay/backend/ai-service/internal/domain"
)

type ChatRepository interface {
	CreateSession(ctx context.Context, userID, title string) (*domain.ChatSession, error)
	GetSession(ctx context.Context, sessionID string) (*domain.ChatSession, error)
	ListSessions(ctx context.Context, userID string) ([]*domain.ChatSession, error)
	AddMessage(ctx context.Context, msg *domain.ChatMessage) (*domain.ChatMessage, error)
	GetMessages(ctx context.Context, sessionID string, limit int) ([]*domain.ChatMessage, error)
}

type InsightRepository interface {
	Save(ctx context.Context, insight *domain.SpendingInsight) error
	ListByUser(ctx context.Context, userID string, limit int) ([]*domain.SpendingInsight, error)
	MarkRead(ctx context.Context, insightID string) error
}

type RecommendationRepository interface {
	SaveBatch(ctx context.Context, recs []*domain.Recommendation) error
	ListByUser(ctx context.Context, userID, recType string, limit int) ([]*domain.Recommendation, error)
	Dismiss(ctx context.Context, recID string) error
}

type AIUsecase struct {
	chatRepo    ChatRepository
	insightRepo InsightRepository
	recRepo     RecommendationRepository
	provider    domain.AIProvider
	logger      *slog.Logger
}

func NewAIUsecase(chatRepo ChatRepository, insightRepo InsightRepository, recRepo RecommendationRepository, provider domain.AIProvider, logger *slog.Logger) *AIUsecase {
	return &AIUsecase{
		chatRepo:    chatRepo,
		insightRepo: insightRepo,
		recRepo:     recRepo,
		provider:    provider,
		logger:      logger,
	}
}

// GenerateLuciInsight orchestrates data fetching, prompt building, and LLM generation.
// It guarantees a fallback string on error so the client UI does not break.
func (u *AIUsecase) GenerateLuciInsight(ctx context.Context, userID string, viewContext string) (string, error) {
	// 1. Safe default fallback to prevent UI breakage
	fallbackMsg := "I'm always here to help you navigate Luce Pay! 🐍"

	if u.provider == nil {
		return fallbackMsg, nil
	}

	// 2. Bound the entire operation to avoid hanging the UI
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	var userData string

	// Fetching recent insights
	insights, err := u.insightRepo.ListByUser(ctx, userID, 3)
	if err == nil && len(insights) > 0 {
		userData = fmt.Sprintf("Recent insight: %s", insights[0].Title)
	} else {
		userData = "User is exploring the app."
	}

	// 4. Strict Persona Injection (Required by System Instructions)
	persona := `You are Luci, the friendly, highly intelligent green cobra mascot for the Luce Pay Super App. Your job is to provide short, helpful, and encouraging insights (max 2 sentences). You must occasionally use emojis like 🐍, ✨, 📈, or 🛡️. Do not use markdown. Speak directly to the user.`

	// 5. Prompt Construction
	prompt := fmt.Sprintf("%s\n\nCurrent Context: %s\nUser Data: %s\n\nGenerate the insight message:", persona, viewContext, userData)

	// 6. Gemini LLM Generation with Retry Logic
	var message string
	var err error
	maxRetries := 3

	for attempt := 0; attempt < maxRetries; attempt++ {
		message, _, err = u.provider.ChatCompletion(persona, []domain.ChatMessage{{Role: "user", Content: prompt}})
		if err == nil {
			break
		}
		
		if u.logger != nil {
			u.logger.Warn("[AI Usecase] Retry generating content from Gemini", "attempt", attempt+1, "error", err)
		}
		
		// Wait before retrying (exponential backoff: 500ms, 1s, 2s)
		select {
		case <-ctx.Done():
			err = ctx.Err()
			goto ErrorHandling
		case <-time.After(time.Duration(500*(1<<attempt)) * time.Millisecond):
		}
	}

ErrorHandling:
	if err != nil {
		if u.logger != nil {
			u.logger.Error("[AI Usecase] Error generating content from Gemini after retries", "error", err)
		}
		// Return safe fallback silently
		return fallbackMsg, nil
	}

	if message == "" {
		return fallbackMsg, nil
	}

	return message, nil
}
