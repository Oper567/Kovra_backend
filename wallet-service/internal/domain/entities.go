package domain

import (
	"time"

	"github.com/shopspring/decimal"
)

// ─── Wallet Entity ───────────────────────────────────────────

type Wallet struct {
	ID        string          `json:"id" db:"id"`
	UserID    string          `json:"user_id" db:"user_id"`
	Balance   decimal.Decimal `json:"balance" db:"balance"`
	Currency  string          `json:"currency" db:"currency"`
	Version   int             `json:"version" db:"version"`
	IsLocked  bool            `json:"is_locked" db:"is_locked"`
	CreatedAt time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt time.Time       `json:"updated_at" db:"updated_at"`
}

// HasSufficientBalance checks if the wallet can cover a debit.
func (w *Wallet) HasSufficientBalance(amount decimal.Decimal) bool {
	return w.Balance.GreaterThanOrEqual(amount)
}

// ─── Transaction Entity ─────────────────────────────────────

type TransactionType string

const (
	TransactionTypeCredit TransactionType = "CREDIT"
	TransactionTypeDebit  TransactionType = "DEBIT"
)

type TransactionStatus string

const (
	TransactionStatusPending   TransactionStatus = "PENDING"
	TransactionStatusCompleted TransactionStatus = "COMPLETED"
	TransactionStatusReversed  TransactionStatus = "REVERSED"
	TransactionStatusFailed    TransactionStatus = "FAILED"
)

type TransactionChannel string

const (
	ChannelWalletFund      TransactionChannel = "WALLET_FUND"
	ChannelWalletTransfer  TransactionChannel = "WALLET_TRANSFER"
	ChannelVTUPurchase     TransactionChannel = "VTU_PURCHASE"
	ChannelEcomOrder       TransactionChannel = "ECOM_ORDER"
	ChannelEdtechEnrollment TransactionChannel = "EDTECH_ENROLLMENT"
	ChannelSagaCompensation TransactionChannel = "SAGA_COMPENSATION"
	ChannelAdmin           TransactionChannel = "ADMIN"
)

type Transaction struct {
	ID             string            `json:"id" db:"id"`
	WalletID       string            `json:"wallet_id" db:"wallet_id"`
	Type           TransactionType   `json:"type" db:"type"`
	Status         TransactionStatus `json:"status" db:"status"`
	Channel        TransactionChannel `json:"channel" db:"channel"`
	Amount         decimal.Decimal   `json:"amount" db:"amount"`
	BalanceBefore  decimal.Decimal   `json:"balance_before" db:"balance_before"`
	BalanceAfter   decimal.Decimal   `json:"balance_after" db:"balance_after"`
	ReferenceID    *string           `json:"reference_id,omitempty" db:"reference_id"`
	IdempotencyKey string            `json:"idempotency_key" db:"idempotency_key"`
	Description    *string           `json:"description,omitempty" db:"description"`
	Metadata       map[string]any    `json:"metadata" db:"metadata"`
	CreatedAt      time.Time         `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at" db:"updated_at"`
}

// ─── Saga Compensation Entity ───────────────────────────────

type SagaStatus string

const (
	SagaStatusPending      SagaStatus = "PENDING"
	SagaStatusCompleted    SagaStatus = "COMPLETED"
	SagaStatusCompensating SagaStatus = "COMPENSATING"
	SagaStatusCompensated  SagaStatus = "COMPENSATED"
	SagaStatusFailed       SagaStatus = "FAILED"
)

type SagaCompensation struct {
	ID               string          `json:"id" db:"id"`
	SagaID           string          `json:"saga_id" db:"saga_id"`
	TransactionID    string          `json:"transaction_id" db:"transaction_id"`
	WalletID         string          `json:"wallet_id" db:"wallet_id"`
	Amount           decimal.Decimal `json:"amount" db:"amount"`
	Status           SagaStatus      `json:"status" db:"status"`
	CompensationData map[string]any  `json:"compensation_data" db:"compensation_data"`
	Attempts         int             `json:"attempts" db:"attempts"`
	MaxAttempts      int             `json:"max_attempts" db:"max_attempts"`
	LastError        *string         `json:"last_error,omitempty" db:"last_error"`
	CompletedAt      *time.Time      `json:"completed_at,omitempty" db:"completed_at"`
	CreatedAt        time.Time       `json:"created_at" db:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at" db:"updated_at"`
}
