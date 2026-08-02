package postgres

import (
	"context"
	"database/sql"
	"errors"

	"github.com/kovra-dev/kovra/backend/vtu-gaming-service/internal/domain"
)

type ProductRepo struct {
	db *sql.DB
}

func NewProductRepo(db *sql.DB) *ProductRepo {
	return &ProductRepo{db: db}
}

func (r *ProductRepo) GetByID(ctx context.Context, id string) (*domain.Product, error) {
	query := `
		SELECT id, provider_id, category, name, description, amount, currency, provider_code, is_active
		FROM vtu_products
		WHERE id = $1
	`
	row := r.db.QueryRowContext(ctx, query, id)
	return scanProduct(row)
}

func (r *ProductRepo) ListByCategory(ctx context.Context, category domain.VTUCategory) ([]*domain.Product, error) {
	query := `
		SELECT id, provider_id, category, name, description, amount, currency, provider_code, is_active
		FROM vtu_products
		WHERE category = $1 AND is_active = true
		ORDER BY amount ASC
	`
	rows, err := r.db.QueryContext(ctx, query, category)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *ProductRepo) ListAll(ctx context.Context) ([]*domain.Product, error) {
	query := `
		SELECT id, provider_id, category, name, description, amount, currency, provider_code, is_active
		FROM vtu_products
		WHERE is_active = true
		ORDER BY category ASC, amount ASC
	`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []*domain.Product
	for rows.Next() {
		p, err := scanProduct(rows)
		if err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

type scanner interface {
	Scan(dest ...any) error
}

func scanProduct(s scanner) (*domain.Product, error) {
	var p domain.Product
	err := s.Scan(
		&p.ID,
		&p.ProviderID,
		&p.Category,
		&p.Name,
		&p.Description,
		&p.Amount,
		&p.Currency,
		&p.ProviderCode,
		&p.IsActive,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, domain.ErrProductNotFound
		}
		return nil, err
	}
	return &p, nil
}
