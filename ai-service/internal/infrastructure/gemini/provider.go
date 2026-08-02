package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/google/generative-ai-go/genai"
	"github.com/kovra-dev/kovra/backend/ai-service/internal/domain"
	"google.golang.org/api/option"
)

type geminiProvider struct {
	client *genai.Client
	logger *slog.Logger
}

func NewGeminiProvider(ctx context.Context, apiKey string, logger *slog.Logger) (domain.AIProvider, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("gemini api key is required")
	}

	client, err := genai.NewClient(ctx, option.WithAPIKey(apiKey))
	if err != nil {
		return nil, err
	}

	return &geminiProvider{
		client: client,
		logger: logger,
	}, nil
}

func (p *geminiProvider) ChatCompletion(systemPrompt string, messages []domain.ChatMessage) (string, int, error) {
	ctx := context.Background()
	model := p.client.GenerativeModel("gemini-1.5-flash")
	model.SystemInstruction = genai.NewUserContent(genai.Text(systemPrompt))
	
	cs := model.StartChat()
	
	// Preload history
	for _, msg := range messages {
		// Skip the very last message as that's the one we'll actually send to get the response
		if msg.Role == "user" {
			cs.History = append(cs.History, &genai.Content{
				Parts: []genai.Part{genai.Text(msg.Content)},
				Role:  "user",
			})
		} else if msg.Role == "assistant" {
			cs.History = append(cs.History, &genai.Content{
				Parts: []genai.Part{genai.Text(msg.Content)},
				Role:  "model",
			})
		}
	}

	// The actual prompt is the last message in history if we just built it, 
	// wait, `messages` contains the NEW user message at the end.
	if len(messages) == 0 {
		return "", 0, fmt.Errorf("no messages to send")
	}

	// Remove the last user message from history and send it as the prompt
	var lastMsg string
	if len(cs.History) > 0 && cs.History[len(cs.History)-1].Role == "user" {
		lastMsg = fmt.Sprintf("%v", cs.History[len(cs.History)-1].Parts[0])
		cs.History = cs.History[:len(cs.History)-1]
	} else {
		lastMsg = messages[len(messages)-1].Content
	}

	res, err := cs.SendMessage(ctx, genai.Text(lastMsg))
	if err != nil {
		return "", 0, fmt.Errorf("send message failed: %w", err)
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		reply := fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0])
		
		// Optional: approximate tokens if UsageMetadata is missing in older SDK versions
		tokens := 0
		if res.UsageMetadata != nil {
			tokens = int(res.UsageMetadata.TotalTokenCount)
		}
		return reply, tokens, nil
	}

	return "", 0, fmt.Errorf("no valid response from gemini")
}

func (p *geminiProvider) GenerateInsights(transactions []domain.TransactionSummary) ([]domain.InsightResult, error) {
	ctx := context.Background()
	model := p.client.GenerativeModel("gemini-1.5-flash")
	model.ResponseMIMEType = "application/json"

	data, _ := json.Marshal(transactions)
	prompt := fmt.Sprintf(`Analyze the following transactions and generate 2 financial insights. 
	Return ONLY a JSON array of objects with keys: "type" (string), "title" (string), "body" (string), "data" (object).
	Transactions: %s`, string(data))

	res, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		jsonStr := fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0])
		var insights []domain.InsightResult
		err = json.Unmarshal([]byte(jsonStr), &insights)
		if err != nil {
			return nil, fmt.Errorf("failed to parse json insights: %w", err)
		}
		return insights, nil
	}

	return nil, fmt.Errorf("empty response for insights")
}

func (p *geminiProvider) GenerateRecommendations(profile domain.UserProfile) ([]domain.RecommendationResult, error) {
	ctx := context.Background()
	model := p.client.GenerativeModel("gemini-1.5-flash")
	model.ResponseMIMEType = "application/json"

	data, _ := json.Marshal(profile)
	prompt := fmt.Sprintf(`Based on the user profile, recommend 3 relevant products, courses, or plans.
	Return ONLY a JSON array of objects with keys: "rec_type" (string: product, course, vtu_plan), "item_id" (string), "score" (float 0-1), "reason" (string).
	Profile: %s`, string(data))

	res, err := model.GenerateContent(ctx, genai.Text(prompt))
	if err != nil {
		return nil, err
	}

	if len(res.Candidates) > 0 && len(res.Candidates[0].Content.Parts) > 0 {
		jsonStr := fmt.Sprintf("%v", res.Candidates[0].Content.Parts[0])
		var recs []domain.RecommendationResult
		err = json.Unmarshal([]byte(jsonStr), &recs)
		if err != nil {
			return nil, fmt.Errorf("failed to parse json recommendations: %w", err)
		}
		return recs, nil
	}

	return nil, fmt.Errorf("empty response for recommendations")
}
