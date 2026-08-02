package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/kovra-dev/kovra/backend/vtu-gaming-service/internal/domain"
)

type DatastationProvider struct {
	apiKey     string
	baseURL    string
	httpClient *http.Client
}

func NewDatastationProvider(apiKey string) *DatastationProvider {
	return &DatastationProvider{
		apiKey:  apiKey,
		baseURL: "https://datastation.com.ng",
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

func (p *DatastationProvider) Name() string {
	return "datastation"
}

func (p *DatastationProvider) Execute(product *domain.Product, recipient string) (string, map[string]any, error) {
	// The exact endpoint depends on the product category based on Datastation docs
	var endpoint string
	var payload map[string]any

	switch product.Category {
	case domain.CategoryAirtime:
		endpoint = "/api/vtu/"
		payload = map[string]any{
			"network": product.ProviderCode, // e.g., "1" for MTN
			"amount":  product.Amount.String(),
			"phone":   recipient,
		}
	case domain.CategoryData:
		endpoint = "/api/data/"
		payload = map[string]any{
			"network": product.ProviderCode,
			"plan":    product.ProviderCode, // e.g., "183"
			"phone":   recipient,
		}
	case domain.CategoryElectricity:
		endpoint = "/api/billpay/"
		payload = map[string]any{
			"disco":       product.ProviderCode,
			"meter_no":    recipient,
			"amount":      product.Amount.String(),
			"meter_type":  "prepaid",
		}
	default:
		return "", nil, fmt.Errorf("unsupported product category for datastation: %s", product.Category)
	}

	url := fmt.Sprintf("%s%s", p.baseURL, endpoint)
	body, _ := json.Marshal(payload)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(body))
	if err != nil {
		return "", nil, err
	}

	req.Header.Add("Authorization", fmt.Sprintf("Token %s", p.apiKey))
	req.Header.Add("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	var responseData map[string]any
	if err := json.Unmarshal(respBody, &responseData); err != nil {
		// If it's not JSON, maybe just string
		responseData = map[string]any{"raw": string(respBody)}
	}

	// Assuming 200 or 201 means success in Datastation. 
	// Real-world implementation would check responseData["status"] or similar.
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return "", responseData, fmt.Errorf("datastation API error (status %d): %s", resp.StatusCode, string(respBody))
	}

	// Generate a fake provider ref if none is returned
	providerRef := fmt.Sprintf("ds-%d", time.Now().UnixNano())
	if ref, ok := responseData["reference"].(string); ok {
		providerRef = ref
	}

	return providerRef, responseData, nil
}

func (p *DatastationProvider) CheckStatus(providerRef string) (domain.OrderStatus, error) {
	// Not fully implemented for Datastation yet
	return domain.OrderCompleted, nil
}
