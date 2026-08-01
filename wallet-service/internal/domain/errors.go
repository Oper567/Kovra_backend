package domain

import "errors"

// ─── Domain Errors ───────────────────────────────────────────
// These are business-logic errors, NOT infrastructure errors.
// The delivery layer maps these to appropriate HTTP/gRPC status codes.

var (
	// Wallet errors
	ErrWalletNotFound          = errors.New("wallet not found")
	ErrWalletLocked            = errors.New("wallet is locked for maintenance")
	ErrInsufficientBalance     = errors.New("insufficient wallet balance")
	ErrConcurrentModification  = errors.New("wallet was modified by another transaction, retry required")
	ErrWalletAlreadyExists     = errors.New("wallet already exists for this user")

	// Transaction errors
	ErrDuplicateIdempotencyKey = errors.New("transaction with this idempotency key already exists")
	ErrTransactionNotFound     = errors.New("transaction not found")
	ErrInvalidAmount           = errors.New("amount must be greater than zero")
	ErrInvalidChannel          = errors.New("invalid transaction channel")

	// Saga errors
	ErrSagaNotFound            = errors.New("saga compensation record not found")
	ErrSagaAlreadyCompensated  = errors.New("saga has already been compensated")
	ErrSagaAlreadyCompleted    = errors.New("saga has already been completed")
	ErrSagaMaxRetriesExceeded  = errors.New("saga compensation max retries exceeded")
	ErrSagaDuplicateID         = errors.New("saga with this ID already exists")
)
