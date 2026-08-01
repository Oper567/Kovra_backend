package domain

import (
	"context"

	"github.com/shopspring/decimal"
)

// ─── Repository Interfaces ──────────────────────────────────
// Defined in the domain layer so that the usecase layer depends
// only on abstractions, never on concrete database implementations.

// WalletRepository handles wallet CRUD with optimistic locking.
type WalletRepository interface {
	// Create creates a new wallet for a user. Returns ErrWalletAlreadyExists
	// if a wallet already exists for the given user_id.
	Create(ctx context.Context, userID, currency string) (*Wallet, error)

	// GetByUserID returns the wallet for a given user. Returns ErrWalletNotFound
	// if no wallet exists.
	GetByUserID(ctx context.Context, userID string) (*Wallet, error)

	// GetByID returns the wallet by its primary key.
	GetByID(ctx context.Context, walletID string) (*Wallet, error)

	// UpdateBalance atomically updates the balance using optimistic locking.
	// The expectedVersion must match the current version in the DB.
	// Returns ErrConcurrentModification if versions don't match.
	UpdateBalance(ctx context.Context, walletID string, newBalance decimal.Decimal, expectedVersion int) (*Wallet, error)
}

// TransactionRepository handles wallet transaction records.
type TransactionRepository interface {
	// Create persists a new transaction. Returns ErrDuplicateIdempotencyKey
	// if the idempotency_key already exists (returns the existing transaction).
	Create(ctx context.Context, txn *Transaction) (*Transaction, error)

	// GetByIdempotencyKey retrieves a transaction by its idempotency key.
	// Returns nil if not found.
	GetByIdempotencyKey(ctx context.Context, key string) (*Transaction, error)

	// GetByID retrieves a single transaction by ID.
	GetByID(ctx context.Context, id string) (*Transaction, error)

	// UpdateStatus updates the status of a transaction.
	UpdateStatus(ctx context.Context, id string, status TransactionStatus) error

	// ListByWalletID returns paginated transactions for a wallet.
	ListByWalletID(ctx context.Context, walletID string, cursor string, limit int) ([]*Transaction, string, error)
}

// SagaRepository handles saga compensation records.
type SagaRepository interface {
	// Create persists a new saga compensation record.
	Create(ctx context.Context, saga *SagaCompensation) (*SagaCompensation, error)

	// GetBySagaID retrieves a saga by its business-level saga ID.
	GetBySagaID(ctx context.Context, sagaID string) (*SagaCompensation, error)

	// UpdateStatus updates the saga status and optionally sets error/completion info.
	UpdateStatus(ctx context.Context, id string, status SagaStatus, lastError *string) error

	// MarkCompleted marks the saga as successfully completed (no rollback needed).
	MarkCompleted(ctx context.Context, sagaID string) error
}

// UnitOfWork provides transactional boundaries across repositories.
// This is the key to ACID compliance for multi-table operations.
type UnitOfWork interface {
	// Execute runs fn inside a database transaction. If fn returns an error,
	// the transaction is rolled back. Otherwise, it's committed.
	Execute(ctx context.Context, fn func(ctx context.Context) error) error
}
