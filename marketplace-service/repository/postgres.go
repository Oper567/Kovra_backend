package repository

import (
	"database/sql"
	"time"

	"github.com/lucepay-dev/lucepay/backend/marketplace-service/models"
)

type PostgresRepository struct {
	db *sql.DB
}

func NewPostgresRepository(db *sql.DB) *PostgresRepository {
	return &PostgresRepository{db: db}
}

// Ensure tables exist (for prototype simplicity)
func (r *PostgresRepository) InitSchema() error {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS merchants (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			user_id UUID NOT NULL,
			store_name VARCHAR(255) NOT NULL,
			description TEXT,
			logo_url VARCHAR(255),
			status VARCHAR(20) DEFAULT 'pending',
			total_sales DECIMAL(15,2) DEFAULT 0,
			total_orders INT DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			merchant_id UUID NOT NULL REFERENCES merchants(id) ON DELETE CASCADE,
			name VARCHAR(255) NOT NULL,
			description TEXT,
			price DECIMAL(10,2) NOT NULL,
			image_url VARCHAR(255),
			category VARCHAR(100),
			stock_count INT DEFAULT 0,
			created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
			updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
		)`,
	}
	for _, q := range queries {
		if _, err := r.db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func (r *PostgresRepository) CreateProduct(p *models.Product) error {
	query := `
		INSERT INTO products (merchant_id, name, description, price, image_url, category, stock_count, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`
	now := time.Now()
	p.CreatedAt = now
	p.UpdatedAt = now
	return r.db.QueryRow(
		query, p.MerchantID, p.Name, p.Description, p.Price, p.ImageURL, p.Category, p.StockCount, p.CreatedAt, p.UpdatedAt,
	).Scan(&p.ID)
}

func (r *PostgresRepository) GetProducts() ([]models.Product, error) {
	query := `SELECT id, merchant_id, name, description, price, image_url, category, stock_count, created_at, updated_at FROM products ORDER BY created_at DESC`
	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var products []models.Product
	for rows.Next() {
		var p models.Product
		if err := rows.Scan(
			&p.ID, &p.MerchantID, &p.Name, &p.Description, &p.Price, &p.ImageURL, &p.Category, &p.StockCount, &p.CreatedAt, &p.UpdatedAt,
		); err != nil {
			return nil, err
		}
		products = append(products, p)
	}
	return products, nil
}

func (r *PostgresRepository) CreateMerchant(m *models.Merchant) error {
	query := `
		INSERT INTO merchants (user_id, store_name, description, logo_url, status, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, total_sales, total_orders
	`
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
	return r.db.QueryRow(
		query, m.UserID, m.StoreName, m.Description, m.LogoURL, m.Status, m.CreatedAt, m.UpdatedAt,
	).Scan(&m.ID, &m.TotalSales, &m.TotalOrders)
}

func (r *PostgresRepository) GetMerchantByUserID(userID string) (*models.Merchant, error) {
	query := `SELECT id, user_id, store_name, description, logo_url, status, total_sales, total_orders, created_at, updated_at FROM merchants WHERE user_id = $1`
	var m models.Merchant
	err := r.db.QueryRow(query, userID).Scan(
		&m.ID, &m.UserID, &m.StoreName, &m.Description, &m.LogoURL, &m.Status, &m.TotalSales, &m.TotalOrders, &m.CreatedAt, &m.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil // Return nil, nil if no merchant found
	}
	if err != nil {
		return nil, err
	}
	return &m, nil
}

func (r *PostgresRepository) UpdateMerchantStatus(id, status string) error {
	query := `UPDATE merchants SET status = $1, updated_at = $2 WHERE id = $3`
	_, err := r.db.Exec(query, status, time.Now(), id)
	return err
}
