package postgres

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/kovra-dev/kovra/backend/wallet-service/internal/domain"
	"github.com/shopspring/decimal"
)

// WalletRepo implements domain.WalletRepository against PostgreSQL.
type WalletRepo struct {
	db *sql.DB
}

func NewWalletRepo(db *sql.DB) *WalletRepo {
	return &WalletRepo{db: db}
}

func (r *WalletRepo) conn(ctx context.Context) querier {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *WalletRepo) Create(ctx context.Context, userID, currency string) (*domain.Wallet, error) {
	q := r.conn(ctx)

	wallet := &domain.Wallet{}
	err := q.QueryRowContext(ctx,
		`INSERT INTO wallets (user_id, currency)
		 VALUES ($1, $2)
		 RETURNING id, user_id, balance, currency, version, is_locked, created_at, updated_at`,
		userID, currency,
	).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency,
		&wallet.Version, &wallet.IsLocked, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrWalletAlreadyExists
		}
		return nil, fmt.Errorf("create wallet: %w", err)
	}

	return wallet, nil
}

func (r *WalletRepo) GetByUserID(ctx context.Context, userID string) (*domain.Wallet, error) {
	q := r.conn(ctx)

	wallet := &domain.Wallet{}
	err := q.QueryRowContext(ctx,
		`SELECT id, user_id, balance, currency, version, is_locked, created_at, updated_at
		 FROM wallets WHERE user_id = $1`,
		userID,
	).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency,
		&wallet.Version, &wallet.IsLocked, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("get wallet by user: %w", err)
	}

	return wallet, nil
}

func (r *WalletRepo) GetByID(ctx context.Context, walletID string) (*domain.Wallet, error) {
	q := r.conn(ctx)

	wallet := &domain.Wallet{}
	err := q.QueryRowContext(ctx,
		`SELECT id, user_id, balance, currency, version, is_locked, created_at, updated_at
		 FROM wallets WHERE id = $1`,
		walletID,
	).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency,
		&wallet.Version, &wallet.IsLocked, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrWalletNotFound
		}
		return nil, fmt.Errorf("get wallet by id: %w", err)
	}

	return wallet, nil
}

// UpdateBalance uses optimistic locking: the UPDATE only succeeds if the
// current version matches expectedVersion. This prevents double-spending
// under high concurrency without database-level row locks.
func (r *WalletRepo) UpdateBalance(ctx context.Context, walletID string, newBalance decimal.Decimal, expectedVersion int) (*domain.Wallet, error) {
	q := r.conn(ctx)

	wallet := &domain.Wallet{}
	err := q.QueryRowContext(ctx,
		`UPDATE wallets
		 SET balance = $1, version = version + 1
		 WHERE id = $2 AND version = $3
		 RETURNING id, user_id, balance, currency, version, is_locked, created_at, updated_at`,
		newBalance, walletID, expectedVersion,
	).Scan(
		&wallet.ID, &wallet.UserID, &wallet.Balance, &wallet.Currency,
		&wallet.Version, &wallet.IsLocked, &wallet.CreatedAt, &wallet.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrConcurrentModification
		}
		return nil, fmt.Errorf("update wallet balance: %w", err)
	}

	return wallet, nil
}

// ─── Transaction Repo ────────────────────────────────────────

type TransactionRepo struct {
	db *sql.DB
}

func NewTransactionRepo(db *sql.DB) *TransactionRepo {
	return &TransactionRepo{db: db}
}

func (r *TransactionRepo) conn(ctx context.Context) querier {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *TransactionRepo) Create(ctx context.Context, txn *domain.Transaction) (*domain.Transaction, error) {
	q := r.conn(ctx)

	metaJSON, _ := json.Marshal(txn.Metadata)

	result := &domain.Transaction{}
	err := q.QueryRowContext(ctx,
		`INSERT INTO wallet_transactions
		 (wallet_id, type, status, channel, amount, balance_before, balance_after,
		  reference_id, idempotency_key, description, metadata)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		 RETURNING id, wallet_id, type, status, channel, amount, balance_before,
		           balance_after, reference_id, idempotency_key, description, metadata,
		           created_at, updated_at`,
		txn.WalletID, txn.Type, txn.Status, txn.Channel, txn.Amount,
		txn.BalanceBefore, txn.BalanceAfter, txn.ReferenceID,
		txn.IdempotencyKey, txn.Description, metaJSON,
	).Scan(
		&result.ID, &result.WalletID, &result.Type, &result.Status,
		&result.Channel, &result.Amount, &result.BalanceBefore,
		&result.BalanceAfter, &result.ReferenceID, &result.IdempotencyKey,
		&result.Description, &metaJSON, &result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			// Idempotent: return the existing transaction
			existing, lookupErr := r.GetByIdempotencyKey(ctx, txn.IdempotencyKey)
			if lookupErr != nil {
				return nil, fmt.Errorf("idempotency lookup: %w", lookupErr)
			}
			return existing, domain.ErrDuplicateIdempotencyKey
		}
		return nil, fmt.Errorf("create transaction: %w", err)
	}

	_ = json.Unmarshal(metaJSON, &result.Metadata)
	return result, nil
}

func (r *TransactionRepo) GetByIdempotencyKey(ctx context.Context, key string) (*domain.Transaction, error) {
	q := r.conn(ctx)

	txn := &domain.Transaction{}
	var metaJSON []byte
	err := q.QueryRowContext(ctx,
		`SELECT id, wallet_id, type, status, channel, amount, balance_before,
		        balance_after, reference_id, idempotency_key, description, metadata,
		        created_at, updated_at
		 FROM wallet_transactions WHERE idempotency_key = $1`,
		key,
	).Scan(
		&txn.ID, &txn.WalletID, &txn.Type, &txn.Status, &txn.Channel,
		&txn.Amount, &txn.BalanceBefore, &txn.BalanceAfter, &txn.ReferenceID,
		&txn.IdempotencyKey, &txn.Description, &metaJSON, &txn.CreatedAt, &txn.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get txn by idempotency key: %w", err)
	}

	_ = json.Unmarshal(metaJSON, &txn.Metadata)
	return txn, nil
}

func (r *TransactionRepo) GetByID(ctx context.Context, id string) (*domain.Transaction, error) {
	q := r.conn(ctx)

	txn := &domain.Transaction{}
	var metaJSON []byte
	err := q.QueryRowContext(ctx,
		`SELECT id, wallet_id, type, status, channel, amount, balance_before,
		        balance_after, reference_id, idempotency_key, description, metadata,
		        created_at, updated_at
		 FROM wallet_transactions WHERE id = $1`,
		id,
	).Scan(
		&txn.ID, &txn.WalletID, &txn.Type, &txn.Status, &txn.Channel,
		&txn.Amount, &txn.BalanceBefore, &txn.BalanceAfter, &txn.ReferenceID,
		&txn.IdempotencyKey, &txn.Description, &metaJSON, &txn.CreatedAt, &txn.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrTransactionNotFound
		}
		return nil, fmt.Errorf("get txn by id: %w", err)
	}

	_ = json.Unmarshal(metaJSON, &txn.Metadata)
	return txn, nil
}

func (r *TransactionRepo) UpdateStatus(ctx context.Context, id string, status domain.TransactionStatus) error {
	q := r.conn(ctx)

	result, err := q.ExecContext(ctx,
		`UPDATE wallet_transactions SET status = $1 WHERE id = $2`,
		status, id,
	)
	if err != nil {
		return fmt.Errorf("update txn status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrTransactionNotFound
	}
	return nil
}

// ListByWalletID returns paginated transactions using cursor-based pagination.
// The cursor is the ID of the last transaction from the previous page.
func (r *TransactionRepo) ListByWalletID(ctx context.Context, walletID string, cursor string, limit int) ([]*domain.Transaction, string, error) {
	q := r.conn(ctx)

	if limit <= 0 || limit > 100 {
		limit = 20
	}

	var rows *sql.Rows
	var err error

	if cursor == "" {
		rows, err = q.QueryContext(ctx,
			`SELECT id, wallet_id, type, status, channel, amount, balance_before,
			        balance_after, reference_id, idempotency_key, description, metadata,
			        created_at, updated_at
			 FROM wallet_transactions
			 WHERE wallet_id = $1
			 ORDER BY created_at DESC
			 LIMIT $2`,
			walletID, limit+1,
		)
	} else {
		rows, err = q.QueryContext(ctx,
			`SELECT id, wallet_id, type, status, channel, amount, balance_before,
			        balance_after, reference_id, idempotency_key, description, metadata,
			        created_at, updated_at
			 FROM wallet_transactions
			 WHERE wallet_id = $1
			   AND created_at < (SELECT created_at FROM wallet_transactions WHERE id = $2)
			 ORDER BY created_at DESC
			 LIMIT $3`,
			walletID, cursor, limit+1,
		)
	}
	if err != nil {
		return nil, "", fmt.Errorf("list transactions: %w", err)
	}
	defer rows.Close()

	var transactions []*domain.Transaction
	for rows.Next() {
		txn := &domain.Transaction{}
		var metaJSON []byte
		if err := rows.Scan(
			&txn.ID, &txn.WalletID, &txn.Type, &txn.Status, &txn.Channel,
			&txn.Amount, &txn.BalanceBefore, &txn.BalanceAfter, &txn.ReferenceID,
			&txn.IdempotencyKey, &txn.Description, &metaJSON, &txn.CreatedAt, &txn.UpdatedAt,
		); err != nil {
			return nil, "", fmt.Errorf("scan transaction: %w", err)
		}
		_ = json.Unmarshal(metaJSON, &txn.Metadata)
		transactions = append(transactions, txn)
	}

	var nextCursor string
	if len(transactions) > limit {
		nextCursor = transactions[limit-1].ID
		transactions = transactions[:limit]
	}

	return transactions, nextCursor, nil
}

// ─── Saga Repo ───────────────────────────────────────────────

type SagaRepo struct {
	db *sql.DB
}

func NewSagaRepo(db *sql.DB) *SagaRepo {
	return &SagaRepo{db: db}
}

func (r *SagaRepo) conn(ctx context.Context) querier {
	if tx := extractTx(ctx); tx != nil {
		return tx
	}
	return r.db
}

func (r *SagaRepo) Create(ctx context.Context, saga *domain.SagaCompensation) (*domain.SagaCompensation, error) {
	q := r.conn(ctx)

	compDataJSON, _ := json.Marshal(saga.CompensationData)

	result := &domain.SagaCompensation{}
	err := q.QueryRowContext(ctx,
		`INSERT INTO saga_compensations
		 (saga_id, transaction_id, wallet_id, amount, status, compensation_data)
		 VALUES ($1, $2, $3, $4, $5, $6)
		 RETURNING id, saga_id, transaction_id, wallet_id, amount, status,
		           compensation_data, attempts, max_attempts, last_error,
		           completed_at, created_at, updated_at`,
		saga.SagaID, saga.TransactionID, saga.WalletID, saga.Amount,
		saga.Status, compDataJSON,
	).Scan(
		&result.ID, &result.SagaID, &result.TransactionID, &result.WalletID,
		&result.Amount, &result.Status, &compDataJSON, &result.Attempts,
		&result.MaxAttempts, &result.LastError, &result.CompletedAt,
		&result.CreatedAt, &result.UpdatedAt,
	)
	if err != nil {
		if isUniqueViolation(err) {
			return nil, domain.ErrSagaDuplicateID
		}
		return nil, fmt.Errorf("create saga: %w", err)
	}

	_ = json.Unmarshal(compDataJSON, &result.CompensationData)
	return result, nil
}

func (r *SagaRepo) GetBySagaID(ctx context.Context, sagaID string) (*domain.SagaCompensation, error) {
	q := r.conn(ctx)

	saga := &domain.SagaCompensation{}
	var compDataJSON []byte
	err := q.QueryRowContext(ctx,
		`SELECT id, saga_id, transaction_id, wallet_id, amount, status,
		        compensation_data, attempts, max_attempts, last_error,
		        completed_at, created_at, updated_at
		 FROM saga_compensations WHERE saga_id = $1`,
		sagaID,
	).Scan(
		&saga.ID, &saga.SagaID, &saga.TransactionID, &saga.WalletID,
		&saga.Amount, &saga.Status, &compDataJSON, &saga.Attempts,
		&saga.MaxAttempts, &saga.LastError, &saga.CompletedAt,
		&saga.CreatedAt, &saga.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrSagaNotFound
		}
		return nil, fmt.Errorf("get saga: %w", err)
	}

	_ = json.Unmarshal(compDataJSON, &saga.CompensationData)
	return saga, nil
}

func (r *SagaRepo) UpdateStatus(ctx context.Context, id string, status domain.SagaStatus, lastError *string) error {
	q := r.conn(ctx)

	result, err := q.ExecContext(ctx,
		`UPDATE saga_compensations
		 SET status = $1, last_error = $2, attempts = attempts + 1
		 WHERE id = $3`,
		status, lastError, id,
	)
	if err != nil {
		return fmt.Errorf("update saga status: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrSagaNotFound
	}
	return nil
}

func (r *SagaRepo) MarkCompleted(ctx context.Context, sagaID string) error {
	q := r.conn(ctx)

	result, err := q.ExecContext(ctx,
		`UPDATE saga_compensations
		 SET status = 'COMPLETED', completed_at = NOW()
		 WHERE saga_id = $1 AND status = 'PENDING'`,
		sagaID,
	)
	if err != nil {
		return fmt.Errorf("complete saga: %w", err)
	}
	rows, _ := result.RowsAffected()
	if rows == 0 {
		return domain.ErrSagaAlreadyCompleted
	}
	return nil
}
