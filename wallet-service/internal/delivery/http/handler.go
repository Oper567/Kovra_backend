package http

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/kovra-dev/kovra/backend/wallet-service/internal/domain"
	"github.com/kovra-dev/kovra/backend/wallet-service/internal/usecase"
	"github.com/shopspring/decimal"
)

// WalletHandler handles HTTP requests for the wallet service.
// These endpoints are exposed through the API Gateway.
type WalletHandler struct {
	uc *usecase.WalletUsecase
}

func NewWalletHandler(uc *usecase.WalletUsecase) *WalletHandler {
	return &WalletHandler{uc: uc}
}

// RegisterRoutes sets up all wallet HTTP routes on the given Gin engine.
func (h *WalletHandler) RegisterRoutes(r *gin.RouterGroup) {
	wallet := r.Group("/wallet")
	{
		wallet.POST("", h.CreateWallet)
		wallet.GET("/balance", h.GetBalance)
		wallet.POST("/credit", h.CreditWallet)
		wallet.GET("/transactions", h.GetTransactions)
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
