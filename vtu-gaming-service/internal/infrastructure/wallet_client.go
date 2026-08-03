package infrastructure

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/domain"
)

type WalletHTTPClient struct {
	baseURL    string
	httpClient *http.Client
}

func NewWalletHTTPClient(baseURL string) *WalletHTTPClient {
	return &WalletHTTPClient{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

func (c *WalletHTTPClient) DebitForSaga(sagaID, userID, amount, channel, description, idempotencyKey string) (string, string, error) {
	url := fmt.Sprintf("%s/internal/wallet/saga/debit", c.baseURL)
	
	payload := map[string]interface{}{
		"saga_id":         sagaID,
		"user_id":         userID,
		"amount":          amount,
		"channel":         channel,
		"description":     description,
		"idempotency_key": idempotencyKey,
	}

	body, _ := json.Marshal(payload)
	
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return "", "", fmt.Errorf("wallet debit request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", "", fmt.Errorf("wallet debit returned status %d", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			TransactionID string `json:"transaction_id"`
			NewBalance    string `json:"new_balance"`
		} `json:"data"`
		Error string `json:"error"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", err
	}

	if !result.Success {
		return "", "", domain.ErrProviderAPIFailed // Or a specific wallet error
	}

	return result.Data.TransactionID, result.Data.NewBalance, nil
}

func (c *WalletHTTPClient) CompensateSaga(sagaID, reason string) error {
	url := fmt.Sprintf("%s/internal/wallet/saga/compensate", c.baseURL)
	
	payload := map[string]interface{}{
		"saga_id": sagaID,
		"reason":  reason,
	}

	body, _ := json.Marshal(payload)
	
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("wallet compensate request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wallet compensate returned status %d", resp.StatusCode)
	}
	return nil
}

func (c *WalletHTTPClient) CompleteSaga(sagaID string) error {
	url := fmt.Sprintf("%s/internal/wallet/saga/complete", c.baseURL)
	
	payload := map[string]interface{}{
		"saga_id": sagaID,
	}

	body, _ := json.Marshal(payload)
	
	resp, err := c.httpClient.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		return fmt.Errorf("wallet complete request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("wallet complete returned status %d", resp.StatusCode)
	}
	return nil
}
