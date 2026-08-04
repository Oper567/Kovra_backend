package usecase

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/lucepay-dev/lucepay/backend/vtu-gaming-service/internal/domain"
)

// VTUUsecase orchestrates the Saga flow for VTU purchases:
// 1. Validate product → 2. Debit wallet (Saga) → 3. Call provider API
// 4a. On success → Complete saga
// 4b. On failure → Compensate saga (refund wallet)
type VTUUsecase struct {
	orderRepo    OrderRepository
	productRepo  ProductRepository
	walletClient domain.WalletClient
	providers    map[string]domain.ProviderAPI
	logger       *slog.Logger
}

// OrderRepository handles VTU order persistence.
type OrderRepository interface {
	Create(ctx context.Context, order *domain.Order) (*domain.Order, error)
	GetByID(ctx context.Context, id string) (*domain.Order, error)
	GetBySagaID(ctx context.Context, sagaID string) (*domain.Order, error)
	UpdateStatus(ctx context.Context, id string, status domain.OrderStatus, providerRef *string, errMsg *string) error
	ListByUserID(ctx context.Context, userID string, cursor string, limit int) ([]*domain.Order, string, error)
}

// ProductRepository handles VTU product catalog queries.
type ProductRepository interface {
	GetByID(ctx context.Context, id string) (*domain.Product, error)
	ListByCategory(ctx context.Context, category domain.VTUCategory) ([]*domain.Product, error)
	ListAll(ctx context.Context) ([]*domain.Product, error)
	Create(ctx context.Context, p *domain.Product) error
	Update(ctx context.Context, p *domain.Product) error
	Delete(ctx context.Context, id string) error
}

func NewVTUUsecase(
	orderRepo OrderRepository,
	productRepo ProductRepository,
	walletClient domain.WalletClient,
	providers map[string]domain.ProviderAPI,
	logger *slog.Logger,
) *VTUUsecase {
	return &VTUUsecase{
		orderRepo:    orderRepo,
		productRepo:  productRepo,
		walletClient: walletClient,
		providers:    providers,
		logger:       logger,
	}
}

// ─── Purchase Flow (Saga Pattern) ───────────────────────────

type PurchaseRequest struct {
	UserID    string
	ProductID string
	Recipient string // Phone number, account ID, etc.
}

type PurchaseResponse struct {
	OrderID     string `json:"order_id"`
	SagaID      string `json:"saga_id"`
	Status      string `json:"status"`
	ProviderRef string `json:"provider_ref,omitempty"`
	Message     string `json:"message"`
}

func (uc *VTUUsecase) Purchase(ctx context.Context, req PurchaseRequest) (*PurchaseResponse, error) {
	// 1. Validate product
	product, err := uc.productRepo.GetByID(ctx, req.ProductID)
	if err != nil {
		return nil, err
	}
	if !product.IsActive {
		return nil, domain.ErrProductInactive
	}

	if req.Recipient == "" {
		return nil, domain.ErrInvalidRecipient
	}

	sagaID := fmt.Sprintf("vtu-%s", uuid.New().String())
	idempotencyKey := fmt.Sprintf("vtu-debit-%s", sagaID)

	// 2. Debit wallet via Saga
	uc.logger.InfoContext(ctx, "initiating saga debit",
		slog.String("saga_id", sagaID),
		slog.String("user_id", req.UserID),
		slog.String("amount", product.Amount.String()),
	)

	walletTxnID, _, err := uc.walletClient.DebitForSaga(
		sagaID, req.UserID, product.Amount.String(),
		"VTU_PURCHASE",
		fmt.Sprintf("VTU Purchase: %s", product.Name),
		idempotencyKey,
	)
	if err != nil {
		return nil, fmt.Errorf("wallet debit failed: %w", err)
	}

	// 3. Create order record
	order := &domain.Order{
		UserID:      req.UserID,
		ProductID:   req.ProductID,
		SagaID:      sagaID,
		WalletTxnID: &walletTxnID,
		Recipient:   req.Recipient,
		Amount:      product.Amount,
		Status:      domain.OrderProcessing,
	}

	createdOrder, err := uc.orderRepo.Create(ctx, order)
	if err != nil {
		// Order creation failed — compensate the wallet debit
		uc.compensateWallet(sagaID, "order creation failed")
		return nil, fmt.Errorf("create order: %w", err)
	}

	// 4. Call third-party provider API
	providerRef, _, providerErr := uc.callProvider(product, req.Recipient)

	if providerErr != nil {
		// PROVIDER FAILED — Saga rollback!
		uc.logger.ErrorContext(ctx, "provider API failed, initiating saga compensation",
			slog.String("saga_id", sagaID),
			slog.String("error", providerErr.Error()),
		)

		errMsg := providerErr.Error()
		uc.orderRepo.UpdateStatus(ctx, createdOrder.ID, domain.OrderFailed, nil, &errMsg)
		uc.compensateWallet(sagaID, "provider API failed: "+errMsg)

		return &PurchaseResponse{
			OrderID: createdOrder.ID,
			SagaID:  sagaID,
			Status:  string(domain.OrderFailed),
			Message: "Purchase failed. Your wallet has been refunded automatically.",
		}, nil
	}

	// 5. SUCCESS — Complete the saga
	uc.orderRepo.UpdateStatus(ctx, createdOrder.ID, domain.OrderCompleted, &providerRef, nil)

	if err := uc.walletClient.CompleteSaga(sagaID); err != nil {
		uc.logger.ErrorContext(ctx, "saga completion failed (non-critical)",
			slog.String("saga_id", sagaID),
			slog.String("error", err.Error()),
		)
	}

	uc.logger.InfoContext(ctx, "VTU purchase completed",
		slog.String("order_id", createdOrder.ID),
		slog.String("saga_id", sagaID),
		slog.String("provider_ref", providerRef),
	)

	return &PurchaseResponse{
		OrderID:     createdOrder.ID,
		SagaID:      sagaID,
		Status:      string(domain.OrderCompleted),
		ProviderRef: providerRef,
		Message:     "Purchase successful!",
	}, nil
}

// callProvider dispatches the request to the appropriate third-party API.
func (uc *VTUUsecase) callProvider(product *domain.Product, recipient string) (string, map[string]any, error) {
	// Retry with exponential backoff (max 3 attempts)
	var lastErr error
	for attempt := 1; attempt <= 3; attempt++ {
		for _, provider := range uc.providers {
			ref, resp, err := provider.Execute(product, recipient)
			if err == nil {
				return ref, resp, nil
			}
			lastErr = err
			uc.logger.Warn("provider attempt failed",
				slog.String("provider", provider.Name()),
				slog.Int("attempt", attempt),
				slog.String("error", err.Error()),
			)
		}
		if attempt < 3 {
			time.Sleep(time.Duration(attempt*500) * time.Millisecond)
		}
	}
	return "", nil, fmt.Errorf("%w: %v", domain.ErrProviderAPIFailed, lastErr)
}

// compensateWallet fires the saga compensation to refund the wallet.
func (uc *VTUUsecase) compensateWallet(sagaID, reason string) {
	if err := uc.walletClient.CompensateSaga(sagaID, reason); err != nil {
		uc.logger.Error("CRITICAL: saga compensation failed",
			slog.String("saga_id", sagaID),
			slog.String("reason", reason),
			slog.String("error", err.Error()),
		)
		// In production: push to dead-letter queue for manual intervention
	}
}

// ─── Product Catalog ────────────────────────────────────────

func (uc *VTUUsecase) ListProducts(ctx context.Context, category string, network string) ([]*domain.Product, error) {
	category = strings.ToUpper(category)
	network = strings.ToUpper(network)
    
	var products []*domain.Product
	var err error

	if category == "" {
		products, err = uc.productRepo.ListAll(ctx)
	} else {
		products, err = uc.productRepo.ListByCategory(ctx, domain.VTUCategory(category))
	}
	if err != nil {
		return nil, err
	}
	
	if network != "" {
		var filtered []*domain.Product
		for _, p := range products {
			if strings.Contains(strings.ToUpper(p.Name), network) {
				filtered = append(filtered, p)
			}
		}
		return filtered, nil
	}
	return products, nil
}

func (uc *VTUUsecase) CreateProduct(ctx context.Context, product *domain.Product) error {
	return uc.productRepo.Create(ctx, product)
}

func (uc *VTUUsecase) UpdateProduct(ctx context.Context, product *domain.Product) error {
	return uc.productRepo.Update(ctx, product)
}

func (uc *VTUUsecase) DeleteProduct(ctx context.Context, id string) error {
	return uc.productRepo.Delete(ctx, id)
}

func (uc *VTUUsecase) ValidateMeter(ctx context.Context, meterNumber, discoName, meterType string) (map[string]any, error) {
	provider, ok := uc.providers["datastation"]
	if !ok {
		return nil, fmt.Errorf("datastation provider not configured")
	}

	type Validator interface {
		ValidateMeter(meterNumber, discoName, meterType string) (map[string]any, error)
	}

	if p, ok := provider.(Validator); ok {
		return p.ValidateMeter(meterNumber, discoName, meterType)
	}
	return nil, fmt.Errorf("provider does not support meter validation")
}

// ─── Order History ──────────────────────────────────────────

func (uc *VTUUsecase) GetOrder(ctx context.Context, userID, orderID string) (*domain.Order, error) {
	order, err := uc.orderRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, err
	}
	if order.UserID != userID {
		return nil, domain.ErrOrderNotFound // Don't leak other users' orders
	}
	return order, nil
}

func (uc *VTUUsecase) ListOrders(ctx context.Context, userID, cursor string, limit int) ([]*domain.Order, string, error) {
	return uc.orderRepo.ListByUserID(ctx, userID, cursor, limit)
}
