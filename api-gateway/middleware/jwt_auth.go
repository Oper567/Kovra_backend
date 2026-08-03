package middleware

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

// JWTClaims represents the claims in a Luce Pay JWT token.
type JWTClaims struct {
	UserID   string `json:"user_id"`
	Email    string `json:"email"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

// JWTAuth is a Gin middleware that validates JWT tokens.
// It extracts user_id and injects it into the request context
// for downstream handlers to use.
func JWTAuth(secret, issuer string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authorization header is required",
				"code":    "AUTH_MISSING",
			})
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Authorization header must be: Bearer <token>",
				"code":    "AUTH_MALFORMED",
			})
			return
		}

		tokenStr := parts[1]
		claims := &JWTClaims{}

		token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (interface{}, error) {
			if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
				return nil, jwt.ErrSignatureInvalid
			}
			return []byte(secret), nil
		}, jwt.WithIssuer(issuer))

		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{
				"success": false,
				"error":   "Invalid or expired token",
				"code":    "AUTH_INVALID",
			})
			return
		}

		// Inject user context for downstream handlers
		c.Set("user_id", claims.UserID)
		c.Set("email", claims.Email)
		c.Set("username", claims.Username)
		c.Set("role", claims.Role)

		// Forward user info to downstream services via headers
		c.Request.Header.Set("X-User-ID", claims.UserID)
		c.Request.Header.Set("X-User-Email", claims.Email)
		c.Request.Header.Set("X-User-Role", claims.Role)

		c.Next()
	}
}

// OptionalJWTAuth is like JWTAuth but doesn't reject unauthenticated requests.
// Used for public endpoints that behave differently for logged-in users.
func OptionalJWTAuth(secret, issuer string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.Next()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "bearer") {
			c.Next()
			return
		}

		claims := &JWTClaims{}
		token, err := jwt.ParseWithClaims(parts[1], claims, func(t *jwt.Token) (interface{}, error) {
			return []byte(secret), nil
		}, jwt.WithIssuer(issuer))

		if err == nil && token.Valid {
			c.Set("user_id", claims.UserID)
			c.Set("email", claims.Email)
			c.Set("username", claims.Username)
			c.Set("role", claims.Role)
			c.Request.Header.Set("X-User-ID", claims.UserID)
		}

		c.Next()
	}
}
