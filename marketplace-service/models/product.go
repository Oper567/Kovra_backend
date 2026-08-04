package models

import "time"

// Product represents an item in the marketplace
type Product struct {
	ID          string    `json:"id"`
	MerchantID  string    `json:"merchant_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Price       float64   `json:"price"`
	ImageURL    string    `json:"image_url"`
	Category    string    `json:"category"`
	StockCount  int       `json:"stock_count"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Merchant represents a seller in the marketplace
type Merchant struct {
	ID           string    `json:"id"`
	UserID       string    `json:"user_id"`
	StoreName    string    `json:"store_name"`
	Description  string    `json:"description"`
	LogoURL      string    `json:"logo_url"`
	TotalSales   float64   `json:"total_sales"`
	TotalOrders  int       `json:"total_orders"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}
