package domain

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

// ─── Entities ────────────────────────────────────────────────

type VTUCategory string

const (
	CategoryAirtime     VTUCategory = "AIRTIME"
	CategoryData        VTUCategory = "DATA"
	CategoryTV          VTUCategory = "TV"
	CategoryElectricity VTUCategory = "ELECTRICITY"
	CategoryInternet    VTUCategory = "INTERNET"
	CategoryEducation   VTUCategory = "EDUCATION"
)

type OrderStatus string

const (
	OrderPending    OrderStatus = "PENDING"
	OrderProcessing OrderStatus = "PROCESSING"
	OrderCompleted  OrderStatus = "COMPLETED"
	OrderFailed     OrderStatus = "FAILED"
	OrderRefunded   OrderStatus = "REFUNDED"
)

type Provider struct {
	ID       string         `json:"id"`
	Name     string         `json:"name"`
	Slug     string         `json:"slug"`
	APIBase  string         `json:"-"` // Never expose
	IsActive bool           `json:"is_active"`
	Config   map[string]any `json:"-"`
}

type Product struct {
	ID           string          `json:"id"`
	ProviderID   string          `json:"provider_id"`
	Category     VTUCategory     `json:"category"`
	Name         string          `json:"name"`
	Description  *string         `json:"description,omitempty"`
	Amount       decimal.Decimal `json:"amount"`
	Currency     string          `json:"currency"`
	ProviderCode string          `json:"-"`
	IsActive     bool            `json:"is_active"`
	Metadata     map[string]any  `json:"metadata"`
}

type Order struct {
	ID               string          `json:"id"`
	UserID           string          `json:"user_id"`
	ProductID        string          `json:"product_id"`
	SagaID           string          `json:"saga_id"`
	WalletTxnID      *string         `json:"wallet_txn_id,omitempty"`
	Recipient        string          `json:"recipient"`
	Amount           decimal.Decimal `json:"amount"`
	Status           OrderStatus     `json:"status"`
	ProviderRef      *string         `json:"provider_ref,omitempty"`
	ProviderResponse map[string]any  `json:"provider_response,omitempty"`
	ErrorMessage     *string         `json:"error_message,omitempty"`
	Attempts         int             `json:"attempts"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
}

// ─── Errors ──────────────────────────────────────────────────

var (
	ErrProductNotFound    = errors.New("VTU product not found")
	ErrProductInactive    = errors.New("VTU product is not currently available")
	ErrProviderInactive   = errors.New("VTU provider is offline")
	ErrOrderNotFound      = errors.New("VTU order not found")
	ErrProviderAPIFailed  = errors.New("third-party provider API call failed")
	ErrProviderTimeout    = errors.New("third-party provider timed out")
	ErrInvalidRecipient   = errors.New("invalid recipient number or ID")
	ErrOrderAlreadyExists = errors.New("order with this saga ID already exists")
)

// ─── Interfaces ──────────────────────────────────────────────

type ProviderAPI interface {
	// Execute sends the top-up/purchase request to the third-party API.
	// Returns the provider's reference ID and response data.
	Execute(product *Product, recipient string) (providerRef string, response map[string]any, err error)

	// CheckStatus queries the provider for the status of a previous request.
	CheckStatus(providerRef string) (OrderStatus, error)

	// Name returns the provider's slug.
	Name() string
}

// WalletClient is the gRPC client interface for the wallet-service.
type WalletClient interface {
	DebitForSaga(sagaID, userID, amount, channel, description, idempotencyKey string) (transactionID string, newBalance string, err error)
	CompensateSaga(sagaID, reason string) error
	CompleteSaga(sagaID string) error
}
