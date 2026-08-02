package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/kovra-dev/kovra/backend/ai-service/internal/domain"
)

// AIUsecase orchestrates all AI features: chatbot, insights, recommendations.
type AIUsecase struct {
	chatRepo    ChatRepository
	insightRepo InsightRepository
	recRepo     RecommendationRepository
	aiProvider  domain.AIProvider
	logger      *slog.Logger
}

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

func NewAIUsecase(
	chatRepo ChatRepository,
	insightRepo InsightRepository,
	recRepo RecommendationRepository,
	aiProvider domain.AIProvider,
	logger *slog.Logger,
) *AIUsecase {
	return &AIUsecase{
		chatRepo:    chatRepo,
		insightRepo: insightRepo,
		recRepo:     recRepo,
		aiProvider:  aiProvider,
		logger:      logger,
	}
}

// ─── Chat (AI Customer Support) ─────────────────────────────

const systemPrompt = `You are Kovra AI, a helpful assistant for the Kovra super app.
You help users with:
- Wallet questions (balance, transactions, funding)
- VTU purchases (airtime, data, gaming credits)
- E-commerce orders and tracking
- Course enrollment and quiz help
- Referral program and rewards

Be friendly, concise, and always guide users toward taking action in the app.
If you don't know something, suggest they contact human support.
Never reveal internal system details or API endpoints.`

type ChatRequest struct {
	UserID    string `json:"user_id"`
	SessionID string `json:"session_id,omitempty"`
	Message   string `json:"message"`
}

type ChatResponse struct {
	SessionID string `json:"session_id"`
	Reply     string `json:"reply"`
	TokensUsed int   `json:"tokens_used"`
}

func (uc *AIUsecase) Chat(ctx context.Context, req ChatRequest) (*ChatResponse, error) {
	var session *domain.ChatSession
	var err error

	// Get or create session
	if req.SessionID != "" {
		session, err = uc.chatRepo.GetSession(ctx, req.SessionID)
		if err != nil {
			return nil, err
		}
	} else {
		title := truncate(req.Message, 64)
		session, err = uc.chatRepo.CreateSession(ctx, req.UserID, title)
		if err != nil {
			return nil, fmt.Errorf("create chat session: %w", err)
		}
	}

	// Save user message
	userMsg := &domain.ChatMessage{
		SessionID: session.ID,
		Role:      "user",
		Content:   req.Message,
	}
	if _, err := uc.chatRepo.AddMessage(ctx, userMsg); err != nil {
		return nil, fmt.Errorf("save user message: %w", err)
	}

	// Get conversation history
	history, err := uc.chatRepo.GetMessages(ctx, session.ID, 20)
	if err != nil {
		return nil, fmt.Errorf("get history: %w", err)
	}

	// Call AI provider
	var historyValues []domain.ChatMessage
	for _, m := range history {
		historyValues = append(historyValues, *m)
	}
	reply, tokens, err := uc.aiProvider.ChatCompletion(systemPrompt, historyValues)
	if err != nil {
		uc.logger.ErrorContext(ctx, "AI provider failed", slog.String("error", err.Error()))
		reply = "I'm having trouble connecting right now. Please try again in a moment, or contact our support team for immediate help."
		tokens = 0
	}

	// Save assistant reply
	assistantMsg := &domain.ChatMessage{
		SessionID:  session.ID,
		Role:       "assistant",
		Content:    reply,
		TokensUsed: tokens,
	}
	uc.chatRepo.AddMessage(ctx, assistantMsg)

	return &ChatResponse{
		SessionID:  session.ID,
		Reply:      reply,
		TokensUsed: tokens,
	}, nil
}

func (uc *AIUsecase) ListChatSessions(ctx context.Context, userID string) ([]*domain.ChatSession, error) {
	return uc.chatRepo.ListSessions(ctx, userID)
}

func (uc *AIUsecase) GetChatHistory(ctx context.Context, sessionID string, limit int) ([]*domain.ChatMessage, error) {
	return uc.chatRepo.GetMessages(ctx, sessionID, limit)
}

// ─── Spending Insights ──────────────────────────────────────

func (uc *AIUsecase) GenerateInsights(ctx context.Context, userID string, transactions []domain.TransactionSummary) ([]*domain.SpendingInsight, error) {
	if len(transactions) < 5 {
		return nil, domain.ErrNoInsightsYet
	}

	results, err := uc.aiProvider.GenerateInsights(transactions)
	if err != nil {
		return nil, fmt.Errorf("generate insights: %w", err)
	}

	var insights []*domain.SpendingInsight
	for _, r := range results {
		insight := &domain.SpendingInsight{
			UserID:      userID,
			Period:      "weekly",
			InsightType: r.Type,
			Title:       r.Title,
			Body:        r.Body,
			Data:        r.Data,
			GeneratedAt: time.Now(),
			ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
		}
		if err := uc.insightRepo.Save(ctx, insight); err != nil {
			uc.logger.ErrorContext(ctx, "save insight failed", slog.String("error", err.Error()))
			continue
		}
		insights = append(insights, insight)
	}

	return insights, nil
}

func (uc *AIUsecase) GetInsights(ctx context.Context, userID string, limit int) ([]*domain.SpendingInsight, error) {
	return uc.insightRepo.ListByUser(ctx, userID, limit)
}

// ─── Recommendations ────────────────────────────────────────

func (uc *AIUsecase) GenerateRecommendations(ctx context.Context, profile domain.UserProfile) ([]*domain.Recommendation, error) {
	results, err := uc.aiProvider.GenerateRecommendations(profile)
	if err != nil {
		return nil, fmt.Errorf("generate recommendations: %w", err)
	}

	var recs []*domain.Recommendation
	for _, r := range results {
		rec := &domain.Recommendation{
			UserID:    profile.UserID,
			RecType:   r.RecType,
			ItemID:    r.ItemID,
			Reason:    r.Reason,
			CreatedAt: time.Now(),
		}
		recs = append(recs, rec)
	}

	if err := uc.recRepo.SaveBatch(ctx, recs); err != nil {
		return nil, fmt.Errorf("save recommendations: %w", err)
	}

	return recs, nil
}

func (uc *AIUsecase) GetRecommendations(ctx context.Context, userID, recType string, limit int) ([]*domain.Recommendation, error) {
	return uc.recRepo.ListByUser(ctx, userID, recType, limit)
}

func (uc *AIUsecase) DismissRecommendation(ctx context.Context, recID string) error {
	return uc.recRepo.Dismiss(ctx, recID)
}

// ─── Helpers ─────────────────────────────────────────────────

func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}
