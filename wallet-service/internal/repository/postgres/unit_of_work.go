package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
)

// ─── Context-based Transaction Management ────────────────────
// This allows repositories to participate in a shared database
// transaction without knowing about each other — key for ACID
// compliance across multi-table Saga operations.

type ctxKey string

const txKey ctxKey = "pg_tx"

// querier is an interface that both *sql.DB and *sql.Tx satisfy.
type querier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

// extractTx pulls a *sql.Tx from context if one exists.
func extractTx(ctx context.Context) *sql.Tx {
	if tx, ok := ctx.Value(txKey).(*sql.Tx); ok {
		return tx
	}
	return nil
}

// ─── UnitOfWork Implementation ──────────────────────────────

type UnitOfWork struct {
	db *sql.DB
}

func NewUnitOfWork(db *sql.DB) *UnitOfWork {
	return &UnitOfWork{db: db}
}

// Execute runs fn inside a serializable database transaction.
// All repositories that call conn(ctx) within fn will use this transaction.
func (u *UnitOfWork) Execute(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := u.db.BeginTx(ctx, &sql.TxOptions{
		Isolation: sql.LevelSerializable,
	})
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}

	txCtx := context.WithValue(ctx, txKey, tx)

	if err := fn(txCtx); err != nil {
		if rbErr := tx.Rollback(); rbErr != nil {
			return fmt.Errorf("rollback failed: %v (original error: %w)", rbErr, err)
		}
		return err
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// ─── Helpers ─────────────────────────────────────────────────

// isUniqueViolation checks if a PostgreSQL error is a unique constraint violation.
func isUniqueViolation(err error) bool {
	// PostgreSQL error code 23505 = unique_violation
	return err != nil && strings.Contains(err.Error(), "23505")
}
