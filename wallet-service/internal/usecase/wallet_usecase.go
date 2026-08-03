package usecase

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/lucepay-dev/lucepay/backend/wallet-service/internal/domain"
	"github.com/shopspring/decimal"
)

// WalletUsecase encapsulates all wallet business logic.
// It orchestrates repositories within transactional boundaries
// using UnitOfWork for ACID compliance.
type WalletUsecase struct {
	walletRepo domain.WalletRepository
	txnRepo    domain.TransactionRepository
	sagaRepo   domain.SagaRepository
	uow        domain.UnitOfWork
	logger     *slog.Logger
}

func NewWalletUsecase(
	walletRepo domain.WalletRepository,
	txnRepo domain.TransactionRepository,
	sagaRepo domain.SagaRepository,
	uow domain.UnitOfWork,
	logger *slog.Logger,
) *WalletUsecase {
	return &WalletUsecase{
		walletRepo: walletRepo,
		txnRepo:    txnRepo,
		sagaRepo:   sagaRepo,
		uow:        uow,
		logger:     logger,
	}
}

// ─── Create Wallet ───────────────────────────────────────────

func (uc *WalletUsecase) CreateWallet(ctx context.Context, userID, currency string) (*domain.Wallet, error) {
	if currency == "" {
		currency = "NGN"
	}

	wallet, err := uc.walletRepo.Create(ctx, userID, currency)
	if err != nil {
		return nil, err
	}

	uc.logger.InfoContext(ctx, "wallet created",
		slog.String("wallet_id", wallet.ID),
		slog.String("user_id", userID),
	)
	return wallet, nil
}

// ─── Get Balance ─────────────────────────────────────────────

func (uc *WalletUsecase) GetBalance(ctx context.Context, userID string) (*domain.Wallet, error) {
	return uc.walletRepo.GetByUserID(ctx, userID)
}

// ─── Credit Wallet ───────────────────────────────────────────

type CreditRequest struct {
	UserID         string
	Amount         decimal.Decimal
	Channel        domain.TransactionChannel
	Description    string
	IdempotencyKey string
	ReferenceID    string
	Metadata       map[string]any
}

type CreditResponse struct {
	Transaction *domain.Transaction
	NewBalance  decimal.Decimal
}

func (uc *WalletUsecase) CreditWallet(ctx context.Context, req CreditRequest) (*CreditResponse, error) {
	if !req.Amount.IsPositive() {
		return nil, domain.ErrInvalidAmount
	}

	// Check idempotency first (outside transaction for performance)
	existing, err := uc.txnRepo.GetByIdempotencyKey(ctx, req.IdempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("idempotency check: %w", err)
	}
	if existing != nil {
		uc.logger.InfoContext(ctx, "idempotent credit request, returning existing",
			slog.String("idempotency_key", req.IdempotencyKey),
		)
		wallet, _ := uc.walletRepo.GetByUserID(ctx, req.UserID)
		return &CreditResponse{Transaction: existing, NewBalance: wallet.Balance}, nil
	}

	var response *CreditResponse

	err = uc.uow.Execute(ctx, func(txCtx context.Context) error {
		wallet, err := uc.walletRepo.GetByUserID(txCtx, req.UserID)
		if err != nil {
			return err
		}
		if wallet.IsLocked {
			return domain.ErrWalletLocked
		}

		newBalance := wallet.Balance.Add(req.Amount)

		desc := req.Description
		refID := req.ReferenceID

		txn := &domain.Transaction{
			WalletID:       wallet.ID,
			Type:           domain.TransactionTypeCredit,
			Status:         domain.TransactionStatusCompleted,
			Channel:        req.Channel,
			Amount:         req.Amount,
			BalanceBefore:  wallet.Balance,
			BalanceAfter:   newBalance,
			ReferenceID:    strPtr(refID),
			IdempotencyKey: req.IdempotencyKey,
			Description:    strPtr(desc),
			Metadata:       req.Metadata,
		}

		createdTxn, err := uc.txnRepo.Create(txCtx, txn)
		if err != nil {
			return err
		}

		updatedWallet, err := uc.walletRepo.UpdateBalance(txCtx, wallet.ID, newBalance, wallet.Version)
		if err != nil {
			return err
		}

		response = &CreditResponse{
			Transaction: createdTxn,
			NewBalance:  updatedWallet.Balance,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	uc.logger.InfoContext(ctx, "wallet credited",
		slog.String("user_id", req.UserID),
		slog.String("amount", req.Amount.String()),
		slog.String("new_balance", response.NewBalance.String()),
	)

	return response, nil
}

// ─── Debit for Saga ──────────────────────────────────────────
// This is called by other services (VTU, E-Comm) via gRPC.
// It creates a debit transaction AND a saga compensation record
// so the debit can be reversed if the downstream operation fails.

type DebitSagaRequest struct {
	SagaID         string
	UserID         string
	Amount         decimal.Decimal
	Channel        domain.TransactionChannel
	Description    string
	IdempotencyKey string
	Metadata       map[string]any
}

type DebitSagaResponse struct {
	TransactionID string
	NewBalance    decimal.Decimal
	SagaID        string
}

func (uc *WalletUsecase) DebitForSaga(ctx context.Context, req DebitSagaRequest) (*DebitSagaResponse, error) {
	if !req.Amount.IsPositive() {
		return nil, domain.ErrInvalidAmount
	}

	// Idempotency: check if saga already exists
	existingSaga, err := uc.sagaRepo.GetBySagaID(ctx, req.SagaID)
	if err == nil && existingSaga != nil {
		uc.logger.InfoContext(ctx, "idempotent saga debit, returning existing",
			slog.String("saga_id", req.SagaID),
		)
		wallet, _ := uc.walletRepo.GetByUserID(ctx, req.UserID)
		return &DebitSagaResponse{
			TransactionID: existingSaga.TransactionID,
			NewBalance:    wallet.Balance,
			SagaID:        req.SagaID,
		}, nil
	}

	var response *DebitSagaResponse

	err = uc.uow.Execute(ctx, func(txCtx context.Context) error {
		wallet, err := uc.walletRepo.GetByUserID(txCtx, req.UserID)
		if err != nil {
			return err
		}
		if wallet.IsLocked {
			return domain.ErrWalletLocked
		}
		if !wallet.HasSufficientBalance(req.Amount) {
			return domain.ErrInsufficientBalance
		}

		newBalance := wallet.Balance.Sub(req.Amount)
		desc := req.Description

		// 1. Create the debit transaction
		txn := &domain.Transaction{
			WalletID:       wallet.ID,
			Type:           domain.TransactionTypeDebit,
			Status:         domain.TransactionStatusPending, // PENDING until saga completes
			Channel:        req.Channel,
			Amount:         req.Amount,
			BalanceBefore:  wallet.Balance,
			BalanceAfter:   newBalance,
			IdempotencyKey: req.IdempotencyKey,
			Description:    strPtr(desc),
			Metadata:       req.Metadata,
		}

		createdTxn, err := uc.txnRepo.Create(txCtx, txn)
		if err != nil {
			return err
		}

		// 2. Create saga compensation record
		saga := &domain.SagaCompensation{
			SagaID:           req.SagaID,
			TransactionID:    createdTxn.ID,
			WalletID:         wallet.ID,
			Amount:           req.Amount,
			Status:           domain.SagaStatusPending,
			CompensationData: req.Metadata,
		}

		if _, err := uc.sagaRepo.Create(txCtx, saga); err != nil {
			return err
		}

		// 3. Update wallet balance (optimistic lock)
		updatedWallet, err := uc.walletRepo.UpdateBalance(txCtx, wallet.ID, newBalance, wallet.Version)
		if err != nil {
			return err
		}

		response = &DebitSagaResponse{
			TransactionID: createdTxn.ID,
			NewBalance:    updatedWallet.Balance,
			SagaID:        req.SagaID,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	uc.logger.InfoContext(ctx, "saga debit executed",
		slog.String("saga_id", req.SagaID),
		slog.String("user_id", req.UserID),
		slog.String("amount", req.Amount.String()),
	)

	return response, nil
}

// ─── Compensate Saga (Rollback) ──────────────────────────────
// Called when a downstream operation fails. Atomically credits
// the debited amount back and marks the saga as COMPENSATED.

type CompensateSagaResponse struct {
	TransactionID  string
	RefundedAmount decimal.Decimal
	NewBalance     decimal.Decimal
}

func (uc *WalletUsecase) CompensateSaga(ctx context.Context, sagaID, reason string) (*CompensateSagaResponse, error) {
	var response *CompensateSagaResponse

	err := uc.uow.Execute(ctx, func(txCtx context.Context) error {
		saga, err := uc.sagaRepo.GetBySagaID(txCtx, sagaID)
		if err != nil {
			return err
		}

		if saga.Status == domain.SagaStatusCompensated {
			return domain.ErrSagaAlreadyCompensated
		}
		if saga.Status == domain.SagaStatusCompleted {
			return domain.ErrSagaAlreadyCompleted
		}

		// Mark original debit as REVERSED
		if err := uc.txnRepo.UpdateStatus(txCtx, saga.TransactionID, domain.TransactionStatusReversed); err != nil {
			return fmt.Errorf("reverse original txn: %w", err)
		}

		// Get current wallet state
		wallet, err := uc.walletRepo.GetByID(txCtx, saga.WalletID)
		if err != nil {
			return err
		}

		newBalance := wallet.Balance.Add(saga.Amount)
		desc := fmt.Sprintf("Saga compensation: %s — %s", sagaID, reason)

		// Create refund transaction
		refundTxn := &domain.Transaction{
			WalletID:       wallet.ID,
			Type:           domain.TransactionTypeCredit,
			Status:         domain.TransactionStatusCompleted,
			Channel:        domain.ChannelSagaCompensation,
			Amount:         saga.Amount,
			BalanceBefore:  wallet.Balance,
			BalanceAfter:   newBalance,
			ReferenceID:    strPtr(saga.TransactionID),
			IdempotencyKey: fmt.Sprintf("saga-comp-%s", sagaID),
			Description:    strPtr(desc),
			Metadata:       map[string]any{"saga_id": sagaID, "reason": reason},
		}

		createdRefund, err := uc.txnRepo.Create(txCtx, refundTxn)
		if err != nil {
			return err
		}

		// Update wallet balance
		updatedWallet, err := uc.walletRepo.UpdateBalance(txCtx, wallet.ID, newBalance, wallet.Version)
		if err != nil {
			return err
		}

		// Mark saga as compensated
		if err := uc.sagaRepo.UpdateStatus(txCtx, saga.ID, domain.SagaStatusCompensated, strPtr(reason)); err != nil {
			return err
		}

		response = &CompensateSagaResponse{
			TransactionID:  createdRefund.ID,
			RefundedAmount: saga.Amount,
			NewBalance:     updatedWallet.Balance,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}

	uc.logger.WarnContext(ctx, "saga compensated (rollback)",
		slog.String("saga_id", sagaID),
		slog.String("reason", reason),
		slog.String("refunded", response.RefundedAmount.String()),
	)

	return response, nil
}

// ─── Complete Saga ───────────────────────────────────────────
// Called when a downstream operation succeeds. Marks the saga
// as completed and the transaction as finalized.

func (uc *WalletUsecase) CompleteSaga(ctx context.Context, sagaID string) error {
	return uc.uow.Execute(ctx, func(txCtx context.Context) error {
		saga, err := uc.sagaRepo.GetBySagaID(txCtx, sagaID)
		if err != nil {
			return err
		}

		if saga.Status != domain.SagaStatusPending {
			if saga.Status == domain.SagaStatusCompleted {
				return nil // Idempotent
			}
			return domain.ErrSagaAlreadyCompensated
		}

		// Mark the original debit transaction as COMPLETED
		if err := uc.txnRepo.UpdateStatus(txCtx, saga.TransactionID, domain.TransactionStatusCompleted); err != nil {
			return err
		}

		// Mark saga as completed
		return uc.sagaRepo.MarkCompleted(txCtx, sagaID)
	})
}

// ─── Transaction History ─────────────────────────────────────

type TransactionListResponse struct {
	Transactions []*domain.Transaction `json:"transactions"`
	NextCursor   string                `json:"next_cursor,omitempty"`
}

func (uc *WalletUsecase) GetTransactionHistory(ctx context.Context, userID string, cursor string, limit int) (*TransactionListResponse, error) {
	wallet, err := uc.walletRepo.GetByUserID(ctx, userID)
	if err != nil {
		return nil, err
	}

	transactions, nextCursor, err := uc.txnRepo.ListByWalletID(ctx, wallet.ID, cursor, limit)
	if err != nil {
		return nil, err
	}

	return &TransactionListResponse{
		Transactions: transactions,
		NextCursor:   nextCursor,
	}, nil
}

// ─── Helpers ─────────────────────────────────────────────────

func strPtr(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
