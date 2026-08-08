package http

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/lucepay-dev/lucepay/backend/wallet-service/internal/domain"
	"github.com/lucepay-dev/lucepay/backend/wallet-service/internal/infrastructure"
	"github.com/lucepay-dev/lucepay/backend/wallet-service/internal/usecase"
	"github.com/shopspring/decimal"
)

// WalletHandler handles HTTP requests for the wallet service.
// These endpoints are exposed through the API Gateway.
type WalletHandler struct {
	uc       *usecase.WalletUsecase
	paystack *infrastructure.PaystackClient
}

func NewWalletHandler(uc *usecase.WalletUsecase, paystack *infrastructure.PaystackClient) *WalletHandler {
	return &WalletHandler{uc: uc, paystack: paystack}
}

// RegisterRoutes sets up all wallet HTTP routes on the given Gin engine.
func (h *WalletHandler) RegisterRoutes(r *gin.RouterGroup) {
	wallet := r.Group("/wallet")
	{
		wallet.POST("", h.CreateWallet)
		wallet.GET("/balance", h.GetBalance)
		wallet.POST("/credit", h.CreditWallet)
		wallet.POST("/transfer", h.TransferWallet)
		wallet.GET("/transactions", h.GetTransactions)
		wallet.POST("/fund/initialize", h.FundInitialize)
	}
	
	// Webhooks
	r.POST("/wallet/webhook/paystack", h.PaystackWebhook)

	// Internal Saga Endpoints (called directly by other microservices)
	saga := r.Group("/internal/wallet/saga")
	{
		saga.POST("/debit", h.SagaDebit)
		saga.POST("/compensate", h.SagaCompensate)
		saga.POST("/complete", h.SagaComplete)
	}
}

// ─── Request/Response DTOs ──────────────────────────────────

type CreateWalletRequest struct {
	UserID   string `json:"user_id" binding:"required,uuid"`
	Currency string `json:"currency"`
}

type CreditWalletRequest struct {
	UserID         string         `json:"user_id" binding:"required,uuid"`
	Amount         decimal.Decimal `json:"amount" binding:"required"`
	Channel        string         `json:"channel" binding:"required"`
	Description    string         `json:"description"`
	IdempotencyKey string         `json:"idempotency_key" binding:"required"`
	ReferenceID    string         `json:"reference_id"`
	Metadata       map[string]any `json:"metadata"`
}

type APIResponse struct {
	Success bool   `json:"success"`
	Data    any    `json:"data,omitempty"`
	Error   string `json:"error,omitempty"`
	Code    string `json:"code,omitempty"`
}

// ─── Handlers ────────────────────────────────────────────────

// CreateWallet godoc
// @Summary Create a new wallet for a user
// @Tags wallet
// @Accept json
// @Produce json
// @Param body body CreateWalletRequest true "Create Wallet"
// @Success 201 {object} APIResponse
// @Router /wallet [post]
func (h *WalletHandler) CreateWallet(c *gin.Context) {
	var req CreateWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body: " + err.Error(),
			Code:    "VALIDATION_ERROR",
		})
		return
	}

	wallet, err := h.uc.CreateWallet(c.Request.Context(), req.UserID, req.Currency)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    wallet,
	})
}

// GetBalance godoc
// @Summary Get wallet balance for a user
// @Tags wallet
// @Produce json
// @Param user_id query string true "User ID"
// @Success 200 {object} APIResponse
// @Router /wallet/balance [get]
func (h *WalletHandler) GetBalance(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		// In production, this comes from JWT context via the API Gateway
		userID = c.GetString("user_id")
	}
	if userID == "" {
		userID = c.GetHeader("X-User-ID")
	}
	if userID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "user_id is required",
			Code:    "VALIDATION_ERROR",
		})
		return
	}

	wallet, err := h.uc.GetBalance(c.Request.Context(), userID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"wallet_id": wallet.ID,
			"balance":   wallet.Balance.StringFixed(4),
			"currency":  wallet.Currency,
			"version":   wallet.Version,
		},
	})
}

// CreditWallet godoc
// @Summary Credit (add funds to) a user's wallet
// @Tags wallet
// @Accept json
// @Produce json
// @Param body body CreditWalletRequest true "Credit Wallet"
// @Success 200 {object} APIResponse
// @Router /wallet/credit [post]
func (h *WalletHandler) CreditWallet(c *gin.Context) {
	var req CreditWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body: " + err.Error(),
			Code:    "VALIDATION_ERROR",
		})
		return
	}

	result, err := h.uc.CreditWallet(c.Request.Context(), usecase.CreditRequest{
		UserID:         req.UserID,
		Amount:         req.Amount,
		Channel:        domain.TransactionChannel(req.Channel),
		Description:    req.Description,
		IdempotencyKey: req.IdempotencyKey,
		ReferenceID:    req.ReferenceID,
		Metadata:       req.Metadata,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"transaction_id": result.Transaction.ID,
			"new_balance":    result.NewBalance.StringFixed(4),
			"status":         result.Transaction.Status,
		},
	})
}

type TransferWalletRequest struct {
	ReceiverID     string          `json:"receiver_id" binding:"required,uuid"`
	Amount         decimal.Decimal `json:"amount" binding:"required"`
	Description    string          `json:"description"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required"`
	Metadata       map[string]any  `json:"metadata"`
}

// TransferWallet godoc
// @Summary Transfer funds to another user
// @Tags wallet
// @Accept json
// @Produce json
// @Param body body TransferWalletRequest true "Transfer Wallet"
// @Success 200 {object} APIResponse
// @Router /wallet/transfer [post]
func (h *WalletHandler) TransferWallet(c *gin.Context) {
	senderID := c.GetString("user_id")
	if senderID == "" {
		// Fallback for testing if not set by middleware
		senderID = c.GetHeader("X-User-ID")
	}
	if senderID == "" {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Success: false,
			Error:   "Unauthorized",
			Code:    "UNAUTHORIZED",
		})
		return
	}

	var req TransferWalletRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "Invalid request body: " + err.Error(),
			Code:    "VALIDATION_ERROR",
		})
		return
	}

	result, err := h.uc.TransferWallet(c.Request.Context(), usecase.TransferRequest{
		SenderID:       senderID,
		ReceiverID:     req.ReceiverID,
		Amount:         req.Amount,
		Description:    req.Description,
		IdempotencyKey: req.IdempotencyKey,
		Metadata:       req.Metadata,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"transaction_id": result.TransactionID,
			"new_balance":    result.NewBalance.StringFixed(4),
			"status":         "COMPLETED",
		},
	})
}

// GetTransactions godoc
// @Summary Get paginated transaction history
// @Tags wallet
// @Produce json
// @Param user_id query string true "User ID"
// @Param cursor query string false "Pagination cursor"
// @Param limit query int false "Page size (max 100)"
// @Success 200 {object} APIResponse
// @Router /wallet/transactions [get]
func (h *WalletHandler) GetTransactions(c *gin.Context) {
	userID := c.Query("user_id")
	if userID == "" {
		userID = c.GetString("user_id")
	}
	if userID == "" {
		userID = c.GetHeader("X-User-ID")
	}
	if userID == "" {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "user_id is required",
			Code:    "VALIDATION_ERROR",
		})
		return
	}

	cursor := c.Query("cursor")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "20"))

	result, err := h.uc.GetTransactionHistory(c.Request.Context(), userID, cursor, limit)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// ─── Paystack Integration ────────────────────────────────────

type FundInitializeRequest struct {
	Email  string          `json:"email" binding:"required,email"`
	Amount decimal.Decimal `json:"amount" binding:"required"`
}

// FundInitialize godoc
// @Summary Initialize wallet funding via Paystack
// @Tags wallet
// @Accept json
// @Produce json
// @Param body body FundInitializeRequest true "Fund Wallet"
// @Success 200 {object} APIResponse
// @Router /wallet/fund/initialize [post]
func (h *WalletHandler) FundInitialize(c *gin.Context) {
	var req FundInitializeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	userID := c.GetString("user_id")
	if userID == "" {
		// Fallback for testing without gateway auth
		userID = c.Query("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, APIResponse{Success: false, Error: "user_id is required"})
			return
		}
	}

	// Generate a unique reference
	refID := "fund-" + userID + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)

	// Call Paystack
	paystackResp, err := h.paystack.InitializePayment(c.Request.Context(), req.Email, refID, req.Amount)
	if err != nil {
		h.handleError(c, fmt.Errorf("paystack initialization failed: %w", err))
		return
	}

	// We should technically create a PENDING transaction here in a real production system,
	// but for simplicity and idempotency, we can just let the webhook handle the creation 
	// of the actual credit transaction when the payment succeeds.

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"authorization_url": paystackResp.Data.AuthorizationURL,
			"reference":         paystackResp.Data.Reference,
		},
	})
}

// PaystackWebhook godoc
// @Summary Paystack Webhook endpoint
// @Tags wallet
// @Router /wallet/webhook/paystack [post]
func (h *WalletHandler) PaystackWebhook(c *gin.Context) {
	log.Println("PAYSTACK WEBHOOK HIT: Attempting to process...")

	// 1. Verify Signature
	signature := c.GetHeader("x-paystack-signature")
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	if !h.paystack.VerifyWebhookSignature(payload, signature) {
		log.Println("[PAYSTACK WEBHOOK] Signature validation failed")
		c.AbortWithStatus(http.StatusUnauthorized)
		return
	}

	// 2. Parse Event
	var event struct {
		Event string `json:"event"`
		Data  struct {
			Reference string  `json:"reference"`
			Status    string  `json:"status"`
			Amount    float64 `json:"amount"` // in kobo
			Customer  struct {
				Email string `json:"email"`
			} `json:"customer"`
		} `json:"data"`
	}

	if err := json.Unmarshal(payload, &event); err != nil {
		c.AbortWithStatus(http.StatusBadRequest)
		return
	}

	// Paystack expects a 200 OK immediately for webhooks
	c.Status(http.StatusOK)

	// 3. Handle charge.success
	if event.Event == "charge.success" && event.Data.Status == "success" {
		// Amount is in kobo, convert to Naira
		amountNaira := decimal.NewFromFloat(event.Data.Amount).Div(decimal.NewFromInt(100))

		// Extract user ID from reference (format: fund-{userID}-{timestamp})
		parts := strings.Split(event.Data.Reference, "-")
		if len(parts) < 2 {
			return // Unknown reference format
		}
		userID := parts[1]

		// Call usecase to credit the wallet
		// The Reference serves as the IdempotencyKey so we don't double-fund
		_, _ = h.uc.CreditWallet(context.Background(), usecase.CreditRequest{
			UserID:         userID,
			Amount:         amountNaira,
			Channel:        domain.TransactionChannel("paystack"),
			Description:    "Wallet funding via Paystack",
			IdempotencyKey: event.Data.Reference,
			ReferenceID:    event.Data.Reference,
			Metadata: map[string]any{
				"paystack_reference": event.Data.Reference,
				"customer_email":     event.Data.Customer.Email,
			},
		})
	}
}

// ─── Error Mapping ───────────────────────────────────────────
// Maps domain errors to appropriate HTTP status codes.

func (h *WalletHandler) handleError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, domain.ErrWalletNotFound):
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false, Error: err.Error(), Code: "WALLET_NOT_FOUND",
		})
	case errors.Is(err, domain.ErrWalletAlreadyExists):
		c.JSON(http.StatusConflict, APIResponse{
			Success: false, Error: err.Error(), Code: "WALLET_EXISTS",
		})
	case errors.Is(err, domain.ErrInsufficientBalance):
		c.JSON(http.StatusUnprocessableEntity, APIResponse{
			Success: false, Error: err.Error(), Code: "INSUFFICIENT_BALANCE",
		})
	case errors.Is(err, domain.ErrWalletLocked):
		c.JSON(http.StatusLocked, APIResponse{
			Success: false, Error: err.Error(), Code: "WALLET_LOCKED",
		})
	case errors.Is(err, domain.ErrConcurrentModification):
		c.JSON(http.StatusConflict, APIResponse{
			Success: false, Error: "Transaction conflict, please retry", Code: "CONCURRENT_MODIFICATION",
		})
	case errors.Is(err, domain.ErrInvalidAmount):
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false, Error: err.Error(), Code: "INVALID_AMOUNT",
		})
	case errors.Is(err, domain.ErrDuplicateIdempotencyKey):
		// Not an error — idempotent success
		return
	default:
		c.JSON(http.StatusInternalServerError, APIResponse{
			Success: false, Error: "An internal error occurred", Code: "INTERNAL_ERROR",
		})
	}
}

// ─── Internal Saga Endpoints ─────────────────────────────────

type SagaDebitRequest struct {
	SagaID         string          `json:"saga_id" binding:"required"`
	UserID         string          `json:"user_id" binding:"required"`
	Amount         decimal.Decimal `json:"amount" binding:"required"`
	Channel        string          `json:"channel" binding:"required"`
	Description    string          `json:"description"`
	IdempotencyKey string          `json:"idempotency_key" binding:"required"`
}

func (h *WalletHandler) SagaDebit(c *gin.Context) {
	var req SagaDebitRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	result, err := h.uc.DebitForSaga(c.Request.Context(), usecase.DebitSagaRequest{
		SagaID:         req.SagaID,
		UserID:         req.UserID,
		Amount:         req.Amount,
		Channel:        domain.TransactionChannel(req.Channel),
		Description:    req.Description,
		IdempotencyKey: req.IdempotencyKey,
	})
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: gin.H{
			"transaction_id": result.TransactionID,
			"new_balance":    result.NewBalance,
		},
	})
}

type SagaCompensateRequest struct {
	SagaID string `json:"saga_id" binding:"required"`
	Reason string `json:"reason"`
}

func (h *WalletHandler) SagaCompensate(c *gin.Context) {
	var req SagaCompensateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	_, err := h.uc.CompensateSaga(c.Request.Context(), req.SagaID, req.Reason)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true})
}

type SagaCompleteRequest struct {
	SagaID string `json:"saga_id" binding:"required"`
}

func (h *WalletHandler) SagaComplete(c *gin.Context) {
	var req SagaCompleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Success: false, Error: err.Error()})
		return
	}

	err := h.uc.CompleteSaga(c.Request.Context(), req.SagaID)
	if err != nil {
		h.handleError(c, err)
		return
	}

	c.JSON(http.StatusOK, APIResponse{Success: true})
}
