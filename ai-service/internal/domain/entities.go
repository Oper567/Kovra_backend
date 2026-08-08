package domain

import (
	"context"
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// ─── AI Chat ─────────────────────────────────────────────────

type ChatSession struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Title     string    `json:"title"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
}

type ChatMessage struct {
	ID         string    `json:"id"`
	SessionID  string    `json:"session_id"`
	Role       string    `json:"role"` // user, assistant, system
	Content    string    `json:"content"`
	TokensUsed int       `json:"tokens_used"`
	CreatedAt  time.Time `json:"created_at"`
}

// ─── Spending Insights ──────────────────────────────────────

type SpendingInsight struct {
	ID          string         `json:"id"`
	UserID      string         `json:"user_id"`
	Period      string         `json:"period"`
	InsightType string         `json:"insight_type"`
	Title       string         `json:"title"`
	Body        string         `json:"body"`
	Data        map[string]any `json:"data"`
	IsRead      bool           `json:"is_read"`
	GeneratedAt time.Time      `json:"generated_at"`
	ExpiresAt   time.Time      `json:"expires_at"`
}

// ─── Recommendations ────────────────────────────────────────

type Recommendation struct {
	ID         string          `json:"id"`
	UserID     string          `json:"user_id"`
	RecType    string          `json:"rec_type"` // product, course, vtu_plan
	ItemID     string          `json:"item_id"`
	Score      decimal.Decimal `json:"score"`
	Reason     string          `json:"reason"`
	IsDismissed bool           `json:"is_dismissed"`
	CreatedAt  time.Time       `json:"created_at"`
}

// ─── Errors ──────────────────────────────────────────────────

var (
	ErrSessionNotFound    = errors.New("chat session not found")
	ErrSessionInactive    = errors.New("chat session is closed")
	ErrAIProviderFailed   = errors.New("AI provider API call failed")
	ErrNoInsightsYet      = errors.New("not enough data to generate insights")
	ErrRateLimitedAI      = errors.New("AI request rate limited, try again later")
)

// ─── AI Provider Interface ──────────────────────────────────

type AIProvider interface {
	// ChatCompletion sends a chat message and returns the AI response.
	ChatCompletion(systemPrompt string, messages []ChatMessage) (response string, tokensUsed int, err error)

	// GenerateInsights analyzes transaction data and produces spending insights.
	GenerateInsights(transactions []TransactionSummary) ([]InsightResult, error)

	// GenerateRecommendations produces personalized recommendations.
	GenerateRecommendations(userProfile UserProfile) ([]RecommendationResult, error)

	EvaluateMerchant(ctx context.Context, storeName, description string) (string, error)
	EvaluateTutor(ctx context.Context, displayName, bio string) (string, error)
}

// TransactionSummary is a lightweight view of transactions for AI analysis.
type TransactionSummary struct {
	Channel string          `json:"channel"`
	Amount  decimal.Decimal `json:"amount"`
	Date    time.Time       `json:"date"`
}

type UserProfile struct {
	UserID           string   `json:"user_id"`
	RecentCategories []string `json:"recent_categories"`
	SpendingTotal    string   `json:"spending_total"`
	TopProducts      []string `json:"top_products"`
	Interests        []string `json:"interests"`
}

type InsightResult struct {
	Type  string         `json:"type"`
	Title string         `json:"title"`
	Body  string         `json:"body"`
	Data  map[string]any `json:"data"`
}

type RecommendationResult struct {
	RecType string  `json:"rec_type"`
	ItemID  string  `json:"item_id"`
	Score   float64 `json:"score"`
	Reason  string  `json:"reason"`
}
