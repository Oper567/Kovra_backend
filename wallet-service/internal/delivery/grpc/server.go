package grpc

import (
	"context"
	"log/slog"

	"github.com/kovra-dev/kovra/backend/wallet-service/internal/domain"
	"github.com/kovra-dev/kovra/backend/wallet-service/internal/usecase"
	"github.com/shopspring/decimal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// WalletGRPCServer implements the gRPC WalletService interface.
// This is the internal API consumed by other microservices
// (vtu-gaming-service, ecom-service, edtech-service).
type WalletGRPCServer struct {
	uc     *usecase.WalletUsecase
	logger *slog.Logger
}

func NewWalletGRPCServer(uc *usecase.WalletUsecase, logger *slog.Logger) *WalletGRPCServer {
	return &WalletGRPCServer{uc: uc, logger: logger}
}

// GetBalance returns the current wallet balance for a user.
func (s *WalletGRPCServer) GetBalance(ctx context.Context, userID string) (walletID string, balance string, currency string, version int32, err error) {
	wallet, ucErr := s.uc.GetBalance(ctx, userID)
	if ucErr != nil {
		return "", "", "", 0, s.mapError(ucErr)
	}

	return wallet.ID, wallet.Balance.StringFixed(4), wallet.Currency, int32(wallet.Version), nil
}

// DebitForSaga debits the wallet as part of a distributed saga.
// Creates a compensation record so the debit can be rolled back.
func (s *WalletGRPCServer) DebitForSaga(
	ctx context.Context,
	sagaID, userID, amount, channel, description, idempotencyKey string,
	metadata map[string]string,
) (transactionID, newBalance string, err error) {
	amt, parseErr := decimal.NewFromString(amount)
	if parseErr != nil {
		return "", "", status.Errorf(codes.InvalidArgument, "invalid amount: %v", parseErr)
	}

	// Convert metadata
	meta := make(map[string]any, len(metadata))
	for k, v := range metadata {
		meta[k] = v
	}

	result, ucErr := s.uc.DebitForSaga(ctx, usecase.DebitSagaRequest{
		SagaID:         sagaID,
		UserID:         userID,
		Amount:         amt,
		Channel:        domain.TransactionChannel(channel),
		Description:    description,
		IdempotencyKey: idempotencyKey,
		Metadata:       meta,
	})
	if ucErr != nil {
		return "", "", s.mapError(ucErr)
	}

	return result.TransactionID, result.NewBalance.StringFixed(4), nil
}

// CompensateSaga rolls back a saga debit, crediting the amount back.
func (s *WalletGRPCServer) CompensateSaga(ctx context.Context, sagaID, reason string) (transactionID, refundedAmount, newBalance string, err error) {
	result, ucErr := s.uc.CompensateSaga(ctx, sagaID, reason)
	if ucErr != nil {
		return "", "", "", s.mapError(ucErr)
	}

	return result.TransactionID, result.RefundedAmount.StringFixed(4), result.NewBalance.StringFixed(4), nil
}

// CompleteSaga marks a saga as successfully completed.
func (s *WalletGRPCServer) CompleteSaga(ctx context.Context, sagaID string) error {
	if err := s.uc.CompleteSaga(ctx, sagaID); err != nil {
		return s.mapError(err)
	}
	return nil
}

// CreditWallet credits funds to a user's wallet.
func (s *WalletGRPCServer) CreditWallet(
	ctx context.Context,
	userID, amount, channel, description, idempotencyKey, referenceID string,
	metadata map[string]string,
) (transactionID, newBalance string, err error) {
	amt, parseErr := decimal.NewFromString(amount)
	if parseErr != nil {
		return "", "", status.Errorf(codes.InvalidArgument, "invalid amount: %v", parseErr)
	}

	meta := make(map[string]any, len(metadata))
	for k, v := range metadata {
		meta[k] = v
	}

	result, ucErr := s.uc.CreditWallet(ctx, usecase.CreditRequest{
		UserID:         userID,
		Amount:         amt,
		Channel:        domain.TransactionChannel(channel),
		Description:    description,
		IdempotencyKey: idempotencyKey,
		ReferenceID:    referenceID,
		Metadata:       meta,
	})
	if ucErr != nil {
		return "", "", s.mapError(ucErr)
	}

	return result.Transaction.ID, result.NewBalance.StringFixed(4), nil
}

// ─── Error Mapping ───────────────────────────────────────────
// Maps domain errors to gRPC status codes.

func (s *WalletGRPCServer) mapError(err error) error {
	switch err {
	case domain.ErrWalletNotFound:
		return status.Error(codes.NotFound, err.Error())
	case domain.ErrInsufficientBalance:
		return status.Error(codes.FailedPrecondition, err.Error())
	case domain.ErrWalletLocked:
		return status.Error(codes.Unavailable, err.Error())
	case domain.ErrConcurrentModification:
		return status.Error(codes.Aborted, "concurrent modification, retry")
	case domain.ErrInvalidAmount:
		return status.Error(codes.InvalidArgument, err.Error())
	case domain.ErrWalletAlreadyExists:
		return status.Error(codes.AlreadyExists, err.Error())
	case domain.ErrSagaNotFound:
		return status.Error(codes.NotFound, err.Error())
	case domain.ErrSagaAlreadyCompensated:
		return status.Error(codes.FailedPrecondition, err.Error())
	case domain.ErrSagaAlreadyCompleted:
		return status.Error(codes.FailedPrecondition, err.Error())
	default:
		s.logger.Error("unhandled gRPC error", slog.String("error", err.Error()))
		return status.Error(codes.Internal, "internal server error")
	}
}
