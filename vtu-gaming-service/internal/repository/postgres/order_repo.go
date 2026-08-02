package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kovra-dev/kovra/backend/vtu-gaming-service/internal/domain"
)

type OrderRepo struct {
	db *sql.DB
}

func NewOrderRepo(db *sql.DB) *OrderRepo {
	return &OrderRepo{db: db}
}

func (r *OrderRepo) Create(ctx context.Context, order *domain.Order) (*domain.Order, error) {
	query := `
		INSERT INTO vtu_orders (
			user_id, product_id, saga_id, wallet_txn_id, recipient, amount, status
		) VALUES (
			$1, $2, $3, $4, $5, $6, $7
		) RETURNING id, created_at, updated_at
	`
	err := r.db.QueryRowContext(
		ctx, query,
		order.UserID, order.ProductID, order.SagaID, order.WalletTxnID, order.Recipient, order.Amount, order.Status,
	).Scan(&order.ID, &order.CreatedAt, &order.UpdatedAt)

	if err != nil {
		return nil, err
	}
	return order, nil
}

func (r *OrderRepo) GetByID(ctx context.Context, id string) (*domain.Order, error) {
	query := `
		SELECT id, user_id, product_id, saga_id, wallet_txn_id, recipient, amount, status, provider_ref, error_message, attempts, completed_at, created_at, updated_at
		FROM vtu_orders
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanOrder(row)
}

func (r *OrderRepo) GetBySagaID(ctx context.Context, sagaID string) (*domain.Order, error) {
	query := `
		SELECT id, user_id, product_id, saga_id, wallet_txn_id, recipient, amount, status, provider_ref, error_message, attempts, completed_at, created_at, updated_at
		FROM vtu_orders
		WHERE saga_id = $1
	`
	row := r.db.QueryRowContext(ctx, query, sagaID)
	return scanOrder(row)
}

func (r *OrderRepo) UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, providerRef *string, errMsg *string) error {
	query := `
		UPDATE vtu_orders
		SET status = $1, provider_ref = COALESCE($2, provider_ref), error_message = COALESCE($3, error_message), updated_at = NOW()
	`
	if status == domain.OrderCompleted {
		query += ", completed_at = NOW()"
	}
	query += " WHERE id = $4"
	
	_, err := r.db.ExecContext(ctx, query, status, providerRef, errMsg, id)
	return err
}

func (r *OrderRepo) ListByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*domain.Order, string, error) {
	// Simple pagination for now (limit/offset could be used, but cursor is better, ignoring cursor for MVP)
	query := `
		SELECT id, user_id, product_id, saga_id, wallet_txn_id, recipient, amount, status, provider_ref, error_message, attempts, completed_at, created_at, updated_at
		FROM vtu_orders
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`
	rows, err := r.db.QueryContext(ctx, query, userID, limit)
	if err != nil {
		return nil, "", err
	}
	defer rows.Close()

	var orders []*domain.Order
	for rows.Next() {
		o, err := scanOrder(rows)
		if err != nil {
			return nil, "", err
		}
		orders = append(orders, o)
	}

	nextCursor := "" // In MVP, just return empty cursor
	return orders, nextCursor, nil
}

func scanOrder(s scanner) (*domain.Order, error) {
	var o domain.Order
	err := s.Scan(
		&o.ID,
		&o.UserID,
		&o.ProductID,
		&o.SagaID,
		&o.WalletTxnID,
		&o.Recipient,
		&o.Amount,
		&o.Status,
		&o.ProviderRef,
		&o.ErrorMessage,
		&o.Attempts,
		&o.CompletedAt,
		&o.CreatedAt,
		&o.UpdatedAt,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrOrderNotFound
		}
		return nil, err
	}
	return &o, nil
}
