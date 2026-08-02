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

func (r *ProductRepo) Create(ctx context.Context, p *domain.Product) error {
	query := `
		INSERT INTO vtu_products (id, provider_id, category, name, description, amount, currency, provider_code, is_active, metadata)
		VALUES (COALESCE($1, uuid_generate_v4()), $2, $3, $4, $5, $6, $7, $8, $9, '{}'::jsonb)
		RETURNING id
	`
	err := r.db.QueryRowContext(ctx, query, p.ID, p.ProviderID, p.Category, p.Name, p.Description, p.Amount, p.Currency, p.ProviderCode, p.IsActive).Scan(&p.ID)
	return err
}

func (r *ProductRepo) Update(ctx context.Context, p *domain.Product) error {
	query := `
		UPDATE vtu_products
		SET name = $2, description = $3, amount = $4, is_active = $5, updated_at = NOW()
		WHERE id = $1
	`
	_, err := r.db.ExecContext(ctx, query, p.ID, p.Name, p.Description, p.Amount, p.IsActive)
	return err
}

func (r *ProductRepo) Delete(ctx context.Context, id string) error {
	query := `UPDATE vtu_products SET is_active = false WHERE id = $1`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}
