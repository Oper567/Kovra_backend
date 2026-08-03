package auth

import (
	"context"
	"crypto/rand"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/smtp"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
	"github.com/resend/resend-go/v2"
	"golang.org/x/crypto/bcrypt"

	"github.com/lucepay-dev/lucepay/backend/api-gateway/config"
)

func generateUUID() string {
	b := make([]byte, 16)
	_, err := rand.Read(b)
	if err != nil {
		return ""
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// EmailAuthHandler handles email/password registration, login, and verification.
type EmailAuthHandler struct {
	DB     *sql.DB
	RDB    *redis.Client
	Config *config.Config
	Logger *slog.Logger
	Resend *resend.Client
}

// NewEmailAuthHandler creates a new EmailAuthHandler.
func NewEmailAuthHandler(db *sql.DB, rdb *redis.Client, cfg *config.Config, logger *slog.Logger) *EmailAuthHandler {
	var resendClient *resend.Client
	if cfg.ResendAPIKey != "" {
		resendClient = resend.NewClient(cfg.ResendAPIKey)
	}

	if db != nil {
		schemaSQL := `
		CREATE EXTENSION IF NOT EXISTS "uuid-ossp";
		CREATE EXTENSION IF NOT EXISTS "pgcrypto";
		CREATE TABLE IF NOT EXISTS users (
			id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
			email         VARCHAR(255) NOT NULL UNIQUE,
			password_hash VARCHAR(255),
			full_name     VARCHAR(255),
			avatar_url    TEXT,
			is_verified   BOOLEAN NOT NULL DEFAULT FALSE,
			provider      VARCHAR(50) DEFAULT 'email',
			provider_id   VARCHAR(255),
			role          VARCHAR(50) NOT NULL DEFAULT 'user',
			created_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
			CONSTRAINT unique_provider_id UNIQUE (provider, provider_id)
		);
		ALTER TABLE users ALTER COLUMN provider SET DEFAULT 'email';
		ALTER TABLE users ALTER COLUMN provider_id DROP NOT NULL;
		ALTER TABLE users ALTER COLUMN provider_id SET DEFAULT '';
		CREATE INDEX IF NOT EXISTS idx_users_email ON users (email);
		`
		if _, err := db.Exec(schemaSQL); err != nil {
			logger.Error("failed to ensure users schema initialized", slog.String("error", err.Error()))
		} else {
			logger.Info("users database schema initialized successfully")
		}
	}

	return &EmailAuthHandler{
		DB:     db,
		RDB:    rdb,
		Config: cfg,
		Logger: logger,
		Resend: resendClient,
	}
}

// RegisterRequest represents the payload for registration.
type RegisterRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"required"`
}

// Register handles user signup.
func (h *EmailAuthHandler) Register(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Check if user exists
	var existingID string
	var isVerified bool
	err := h.DB.QueryRowContext(c.Request.Context(), "SELECT id, is_verified FROM users WHERE LOWER(email) = $1", req.Email).Scan(&existingID, &isVerified)
	
	if err == nil {
		if isVerified {
			c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
			return
		}
		
		// Unverified account exists, update it instead of returning 409
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			h.Logger.Error("failed to hash password", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
			return
		}
		
		_, err = h.DB.ExecContext(c.Request.Context(), "UPDATE users SET password_hash = $1, full_name = $2 WHERE id = $3", string(hashedPassword), req.FullName, existingID)
		if err != nil {
			h.Logger.Error("failed to update unverified user", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	} else if errors.Is(err, sql.ErrNoRows) {
		// Hash password
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			h.Logger.Error("failed to hash password", slog.String("error", err.Error()))
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
			return
		}

		// Insert into DB as unverified
		var userID string
		err = h.DB.QueryRowContext(c.Request.Context(), `
			INSERT INTO users (email, password_hash, full_name, is_verified, provider, provider_id)
			VALUES ($1, $2, $3, false, 'email', NULL)
			RETURNING id
		`, req.Email, string(hashedPassword), req.FullName).Scan(&userID)
		
		if err != nil {
			h.Logger.Warn("default insert user failed, retrying with explicit UUID", slog.String("error", err.Error()))
			genID := generateUUID()
			err = h.DB.QueryRowContext(c.Request.Context(), `
				INSERT INTO users (id, email, password_hash, full_name, is_verified, provider, provider_id)
				VALUES ($1, $2, $3, $4, false, 'email', NULL)
				RETURNING id
			`, genID, req.Email, string(hashedPassword), req.FullName).Scan(&userID)
		}

		if err != nil {
			h.Logger.Error("failed to insert user after retry", slog.String("error", err.Error()), slog.String("email", req.Email))
			if strings.Contains(err.Error(), "duplicate key") || strings.Contains(err.Error(), "unique") {
				c.JSON(http.StatusConflict, gin.H{"error": "Email already registered"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to register user: " + err.Error()})
			return
		}
	} else {
		h.Logger.Error("failed to check existing user", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Generate and send verification code
	err = h.sendVerificationEmail(c.Request.Context(), req.Email)
	if err != nil {
		h.Logger.Error("failed to send verification email", slog.String("error", err.Error()))
		// We still return 201 Created because the account exists, but tell them email failed
		c.JSON(http.StatusCreated, gin.H{"message": "User registered, but failed to send verification email. Please try logging in to resend."})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"message": "User registered successfully. Please check your email for the verification code."})
}

// VerifyRequest represents the payload for email verification.
type VerifyRequest struct {
	Email string `json:"email" binding:"required,email"`
	Code  string `json:"code" binding:"required,len=6"`
}

// Verify verifies the 6-digit code sent to the user's email.
func (h *EmailAuthHandler) Verify(c *gin.Context) {
	var req VerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	// Get code from Redis
	redisKey := fmt.Sprintf("verify:%s", req.Email)
	storedCode, err := h.RDB.Get(c.Request.Context(), redisKey).Result()
	if errors.Is(err, redis.Nil) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Verification code expired or invalid"})
		return
	} else if err != nil {
		h.Logger.Error("failed to get verification code from redis", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Internal error"})
		return
	}

	if storedCode != req.Code {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Incorrect verification code"})
		return
	}

	// Update user in DB
	var userID, role, fullName string
	err = h.DB.QueryRowContext(c.Request.Context(), `
		UPDATE users SET is_verified = true WHERE email = $1 RETURNING id, role, full_name
	`, req.Email).Scan(&userID, &role, &fullName)
	if err != nil {
		h.Logger.Error("failed to update user verification status", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to verify user"})
		return
	}

	// Delete code from Redis
	h.RDB.Del(c.Request.Context(), redisKey)

	// Issue JWT
	token, err := generateJWT(userID, req.Email, fullName, role, h.Config)
	if err != nil {
		h.Logger.Error("failed to generate jwt", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"token":   token,
		"message": "Email verified successfully",
		"user": gin.H{
			"id":    userID,
			"email": req.Email,
			"name":  fullName,
			"role":  role,
		},
	})
}

// LoginRequest represents the payload for login.
type LoginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

// Login authenticates a user and returns a JWT.
func (h *EmailAuthHandler) Login(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request payload"})
		return
	}

	// Normalize email
	req.Email = strings.ToLower(strings.TrimSpace(req.Email))

	var userID, passwordHash, fullName, role string
	var isVerified bool
	err := h.DB.QueryRowContext(c.Request.Context(), `
		SELECT id, password_hash, full_name, role, is_verified 
		FROM users WHERE email = $1 AND (provider IS NULL OR provider = 'email' OR provider = 'local')
	`, req.Email).Scan(&userID, &passwordHash, &fullName, &role, &isVerified)

	if errors.Is(err, sql.ErrNoRows) {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	} else if err != nil {
		h.Logger.Error("failed to query user for login", slog.String("error", err.Error()))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
		return
	}

	// Verify password
	if err := bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid email or password"})
		return
	}

	// Check if verified
	if !isVerified {
		// Optionally resend verification code here automatically or let them trigger it via another endpoint.
		// For UX, auto-resending here is a good idea.
		_ = h.sendVerificationEmail(c.Request.Context(), req.Email)
		c.JSON(http.StatusForbidden, gin.H{"error": "Email not verified. A new verification code has been sent to your email."})
		return
	}

	// Issue JWT
	token, err := generateJWT(userID, req.Email, fullName, role, h.Config)
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
			"email": req.Email,
			"name":  fullName,
			"role":  role,
		},
	})
}

func (h *EmailAuthHandler) sendVerificationEmail(ctx context.Context, email string) error {
	// Generate 6-digit code
	codeInt, err := rand.Int(rand.Reader, big.NewInt(900000))
	if err != nil {
		return err
	}
	code := fmt.Sprintf("%06d", codeInt.Int64()+100000)

	// Save code in Redis with 15-minute TTL
	redisKey := fmt.Sprintf("verify:%s", email)
	err = h.RDB.Set(ctx, redisKey, code, 15*time.Minute).Err()
	if err != nil {
		return err
	}

	if h.Config.SMTPHost != "" {
		// Use standard SMTP
		auth := smtp.PlainAuth("", h.Config.SMTPUser, h.Config.SMTPPass, h.Config.SMTPHost)
		from := h.Config.SMTPFrom
		if from == "" {
			from = "noreply@lucepay.com"
		}
		to := []string{email}
		msg := []byte("To: " + email + "\r\n" +
			"Subject: Your Luce Pay Verification Code\r\n" +
			"Content-Type: text/html; charset=UTF-8\r\n\r\n" +
			fmt.Sprintf("<p>Your verification code is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>", code))
			
		err := smtp.SendMail(h.Config.SMTPHost+":"+h.Config.SMTPPort, auth, from, to, msg)
		if err != nil {
			h.Logger.Error("failed to send via smtp", slog.String("error", err.Error()))
			return fmt.Errorf("failed to send via smtp: %w", err)
		}
		return nil
	}

	if h.Resend == nil {
		h.Logger.Warn("Resend API key not set, skipping actual email sending", slog.String("code", code), slog.String("email", email))
		return nil
	}

	from := h.Config.SMTPFrom
	if from == "" || from == "noreply@lucepay.com" {
		from = "Luce Pay Auth <noreply@unismart.com.ng>"
	}

	// Send email via Resend
	params := &resend.SendEmailRequest{
		From:    from,
		To:      []string{email},
		Subject: "Your Luce Pay Verification Code",
		Html:    fmt.Sprintf("<p>Your verification code is: <strong>%s</strong></p><p>This code expires in 15 minutes.</p>", code),
	}

	_, err = h.Resend.Emails.Send(params)
	if err != nil {
		return fmt.Errorf("failed to send via resend: %w", err)
	}

	return nil
}
