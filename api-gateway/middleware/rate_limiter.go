package middleware

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

// RateLimiter implements a Redis-backed sliding window rate limiter.
// Each user gets `maxRequests` per minute. IP-based fallback for
// unauthenticated requests.
type RateLimiter struct {
	rdb         *redis.Client
	maxRequests int
	window      time.Duration
}

func NewRateLimiter(rdb *redis.Client, maxPerMinute int) *RateLimiter {
	return &RateLimiter{
		rdb:         rdb,
		maxRequests: maxPerMinute,
		window:      time.Minute,
	}
}

// Limit returns a Gin middleware that enforces rate limiting.
func (rl *RateLimiter) Limit() gin.HandlerFunc {
	return func(c *gin.Context) {
		var key string
		if userID, exists := c.Get("user_id"); exists {
			key = fmt.Sprintf("rl:user:%s", userID)
		} else {
			key = fmt.Sprintf("rl:ip:%s", c.ClientIP())
		}

		ctx := context.Background()

		// Sliding window counter using Redis INCR + EXPIRE
		pipe := rl.rdb.Pipeline()
		incr := pipe.Incr(ctx, key)
		pipe.Expire(ctx, key, rl.window)
		_, err := pipe.Exec(ctx)
		if err != nil {
			// Redis down — fail open (allow the request)
			c.Next()
			return
		}

		count := incr.Val()
		remaining := int64(rl.maxRequests) - count
		if remaining < 0 {
			remaining = 0
		}

		// Set rate limit headers
		c.Header("X-RateLimit-Limit", fmt.Sprintf("%d", rl.maxRequests))
		c.Header("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		c.Header("X-RateLimit-Reset", fmt.Sprintf("%d", time.Now().Add(rl.window).Unix()))

		if count > int64(rl.maxRequests) {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{
				"success": false,
				"error":   "Rate limit exceeded. Please slow down.",
				"code":    "RATE_LIMITED",
			})
			return
		}

		c.Next()
	}
}
