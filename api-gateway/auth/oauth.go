package auth

import (
	"database/sql"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/kovra-dev/kovra/backend/api-gateway/config"
	"github.com/kovra-dev/kovra/backend/api-gateway/middleware"
	"github.com/markbates/goth/gothic"
)

// OAuthHandler encapsulates the database and config for OAuth flows.
type OAuthHandler struct {
	DB     *sql.DB
	Config *config.Config
	Logger *slog.Logger
}

// NewOAuthHandler creates a new OAuthHandler.
func NewOAuthHandler(db *sql.DB, cfg *config.Config, logger *slog.Logger) *OAuthHandler {
	return &OAuthHandler{
		DB:     db,
		Config: cfg,
		Logger: logger,
	}
}

// BeginAuth initiates the OAuth flow by redirecting the user to the provider.
func (h *OAuthHandler) BeginAuth(c *gin.Context) {
	// gothic expects provider to be in the URL query or in context.
	// Since gin has params, we manually add it to URL query so gothic finds it.
	provider := c.Param("provider")
	q := c.Request.URL.Query()
	q.Add("provider", provider)
	c.Request.URL.RawQuery = q.Encode()

	gothic.BeginAuthHandler(c.Writer, c.Request)
}

// Callback handles the response from the OAuth provider.
func (h *OAuthHandler) Callback(c *gin.Context) {
	provider := c.Param("provider")
	q := c.Request.URL.Query()
	q.Add("provider", provider)
	c.Request.URL.RawQuery = q.Encode()

	user, err := gothic.CompleteUserAuth(c.Writer, c.Request)
	if err != nil {
		h.Logger.Error("oauth complete auth failed", slog.String("error", err.Error()))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication failed"})
		return
	}

	// Find or create user in database
	var userID string
	var role string

	err = h.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO users (email, full_name, avatar_url, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE SET 
			full_name = EXCLUDED.full_name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
		RETURNING id, role
	`, user.Email, user.Name, user.AvatarURL, provider, user.UserID).Scan(&userID, &role)

	if err != nil {
		h.Logger.Error("failed to upsert user", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create/login user"})
		return
	}

	// Generate Kovra JWT
	token, err := generateJWT(userID, user.Email, user.Name, role, h.Config)
	if err != nil {
		h.Logger.Error("failed to generate jwt", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	// Return success (or redirect in a real app, typically redirect back to frontend app with token)
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user": gin.H{
			"id":    userID,
			"email": user.Email,
			"name":  user.Name,
			"role":  role,
		},
	})
}

// TokenCallback handles token-based authentication directly from client SDKs.
func (h *OAuthHandler) TokenCallback(c *gin.Context) {
	provider := c.Param("provider")
	if provider != "google" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Unsupported provider"})
		return
	}

	var req struct {
		AccessToken string `json:"access_token"`
		IDToken     string `json:"id_token"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		h.Logger.Error("failed to parse token request", slog.String("error", err.Error()))
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	tokenToVerify := req.AccessToken
	if tokenToVerify == "" {
		tokenToVerify = req.IDToken
	}

	if tokenToVerify == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "access_token or id_token is required"})
		return
	}

	// Verify with Google UserInfo API
	userInfoURL := "https://www.googleapis.com/oauth2/v3/userinfo"
	client := &http.Client{Timeout: 10 * time.Second}
	googleReq, err := http.NewRequest("GET", userInfoURL, nil)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create request"})
		return
	}

	googleReq.Header.Set("Authorization", "Bearer "+tokenToVerify)
	resp, err := client.Do(googleReq)
	if err != nil {
		h.Logger.Error("failed to fetch user info from google", slog.String("error", err.Error()))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Failed to verify token with Google"})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		h.Logger.Error("google userinfo returned non-200", slog.Int("status", resp.StatusCode), slog.String("body", string(bodyBytes)))
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
		return
	}

	var googleUser struct {
		Sub     string `json:"sub"`
		Name    string `json:"name"`
		Email   string `json:"email"`
		Picture string `json:"picture"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&googleUser); err != nil {
		h.Logger.Error("failed to decode google user info", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to parse Google response"})
		return
	}

	// Find or create user in database
	var userID string
	var role string

	err = h.DB.QueryRowContext(c.Request.Context(), `
		INSERT INTO users (email, full_name, avatar_url, provider, provider_id)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (email) DO UPDATE SET 
			full_name = EXCLUDED.full_name,
			avatar_url = EXCLUDED.avatar_url,
			updated_at = NOW()
		RETURNING id, role
	`, googleUser.Email, googleUser.Name, googleUser.Picture, provider, googleUser.Sub).Scan(&userID, &role)

	if err != nil {
		h.Logger.Error("failed to upsert user", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to create/login user"})
		return
	}

	// Generate Kovra JWT
	token, err := generateJWT(userID, googleUser.Email, googleUser.Name, role, h.Config)
	if err != nil {
		h.Logger.Error("failed to generate jwt", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"user": gin.H{
			"id":    userID,
			"email": googleUser.Email,
			"name":  googleUser.Name,
			"role":  role,
		},
	})
}


// generateJWT creates a new JWT token for the authenticated user.
func generateJWT(userID, email, name, role string, cfg *config.Config) (string, error) {
	claims := middleware.JWTClaims{
		UserID:   userID,
		Email:    email,
		Username: name,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.JWTIssuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}
