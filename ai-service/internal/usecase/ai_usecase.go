package usecase

import (
	"context"
	"fmt"
	"log"
	"time"
)

// InsightRepo handles fetching contextual data for the user from various domains.
type InsightRepo interface {
	GetWalletInsight(ctx context.Context, userID string) (string, error)
	GetRewardsInsight(ctx context.Context, userID string) (string, error)
	GetEdTechInsight(ctx context.Context, userID string) (string, error)
}

// GeminiProvider handles the interaction with the Google Gemini LLM API.
type GeminiProvider interface {
	GenerateContent(ctx context.Context, prompt string) (string, error)
}

type AIUsecase struct {
	repo     InsightRepo
	provider GeminiProvider
}

func NewAIUsecase(repo InsightRepo, provider GeminiProvider) *AIUsecase {
	return &AIUsecase{
		repo:     repo,
		provider: provider,
	}
}

// GenerateKoviInsight orchestrates data fetching, prompt building, and LLM generation.
// It guarantees a fallback string on error so the client UI does not break.
func (u *AIUsecase) GenerateKoviInsight(ctx context.Context, userID string, viewContext string) (string, error) {
	// 1. Safe default fallback to prevent UI breakage
	fallbackMsg := "I'm always here to help you navigate Kovra! 🐍"

	// 2. Bound the entire operation to avoid hanging the UI
	ctx, cancel := context.WithTimeout(ctx, 4*time.Second)
	defer cancel()

	var userData string
	var err error

	// 3. Contextual Data Fetching
	switch viewContext {
	case "wallet_view":
		userData, err = u.repo.GetWalletInsight(ctx, userID)
	case "rewards_view":
		userData, err = u.repo.GetRewardsInsight(ctx, userID)
	case "edtech_view", "quiz_start":
		userData, err = u.repo.GetEdTechInsight(ctx, userID)
	default:
		userData = "User is exploring the app."
	}

	if err != nil {
		log.Printf("[AI Usecase] Warning: Failed to fetch data for %s: %v", viewContext, err)
		userData = "User data unavailable at the moment."
	}

	// 4. Strict Persona Injection (Required by System Instructions)
	persona := `You are Kovi, the friendly, highly intelligent green cobra mascot for the Kovra Super App. Your job is to provide short, helpful, and encouraging insights (max 2 sentences). You must occasionally use emojis like 🐍, ✨, 📈, or 🛡️. Do not use markdown. Speak directly to the user.`

	// 5. Prompt Construction
	prompt := fmt.Sprintf("%s\n\nCurrent Context: %s\nUser Data: %s\n\nGenerate the insight message:", persona, viewContext, userData)

	// 6. Gemini LLM Generation
	message, err := u.provider.GenerateContent(ctx, prompt)
	if err != nil {
		log.Printf("[AI Usecase] Error generating content from Gemini: %v", err)
		// Return safe fallback silently
		return fallbackMsg, nil
	}

	if message == "" {
		return fallbackMsg, nil
	}

	return message, nil
}
