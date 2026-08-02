package infrastructure

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/shopspring/decimal"
)

// PaystackClient interacts with the Paystack API.
type PaystackClient struct {
	secretKey  string
	httpClient *http.Client
}

func NewPaystackClient(secretKey string) *PaystackClient {
	return &PaystackClient{
		secretKey: secretKey,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// InitializePaymentRequest holds data required to start a Paystack transaction.
type InitializePaymentRequest struct {
	Email     string `json:"email"`
	Amount    string `json:"amount"` // Amount in kobo (multiply Naira by 100)
	Reference string `json:"reference"`
	Callback  string `json:"callback_url,omitempty"`
}

// InitializePaymentResponse is returned from Paystack.
type InitializePaymentResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		AuthorizationURL string `json:"authorization_url"`
		AccessCode       string `json:"access_code"`
		Reference        string `json:"reference"`
	} `json:"data"`
}

// InitializePayment calls Paystack to get a checkout URL.
func (p *PaystackClient) InitializePayment(ctx context.Context, email, reference string, amount decimal.Decimal) (*InitializePaymentResponse, error) {
	// Paystack expects amount in Kobo (Naira * 100)
	amountInKobo := amount.Mul(decimal.NewFromInt(100)).IntPart()

	reqPayload := InitializePaymentRequest{
		Email:     email,
		Amount:    fmt.Sprintf("%d", amountInKobo),
		Reference: reference,
	}

	payloadBytes, err := json.Marshal(reqPayload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal paystack request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", "https://api.paystack.co/transaction/initialize", bytes.NewReader(payloadBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create paystack request: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.secretKey)
	req.Header.Set("Content-Type", "application/json")

	res, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("paystack http request failed: %w", err)
	}
	defer res.Body.Close()

	bodyBytes, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read paystack response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("paystack returned status %d: %s", res.StatusCode, string(bodyBytes))
	}

	var response InitializePaymentResponse
	if err := json.Unmarshal(bodyBytes, &response); err != nil {
		return nil, fmt.Errorf("failed to decode paystack response: %w", err)
	}

	if !response.Status {
		return nil, fmt.Errorf("paystack error: %s", response.Message)
	}

	return &response, nil
}

// VerifyWebhookSignature checks if the payload was actually signed by Paystack.
func (p *PaystackClient) VerifyWebhookSignature(payload []byte, signature string) bool {
	mac := hmac.New(sha512.New, []byte(p.secretKey))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))

	return expectedMAC == signature
}
