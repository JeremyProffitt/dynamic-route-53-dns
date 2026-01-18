package middleware

import (
	"fmt"
	"time"

	"github.com/dynamic-route-53-dns/internal/database"
	"github.com/gofiber/fiber/v2"
)

const (
	// RateLimitWindow is the sliding window duration for rate limiting
	RateLimitWindow = time.Hour
	// MaxRequestsPerWindow is the maximum number of requests allowed per hostname per window
	MaxRequestsPerWindow = 60
)

// RateLimitMiddleware provides rate limiting functionality for DDNS updates
type RateLimitMiddleware struct {
	db *database.Client
}

// NewRateLimitMiddleware creates a new RateLimitMiddleware instance
func NewRateLimitMiddleware(db *database.Client) *RateLimitMiddleware {
	return &RateLimitMiddleware{
		db: db,
	}
}

// DDNSUpdateRateLimit returns a Fiber handler that enforces rate limiting on DDNS update requests
// Rate limit: 60 requests per hour per hostname using a sliding window approach
func (rl *RateLimitMiddleware) DDNSUpdateRateLimit() fiber.Handler {
	return func(c *fiber.Ctx) error {
		hostname := c.Query("hostname")
		if hostname == "" {
			// Let the handler deal with missing hostname
			return c.Next()
		}

		// Get current window timestamp (floor to the current hour for fixed window)
		// Using fixed window for simplicity and DynamoDB efficiency
		windowStart := time.Now().Truncate(RateLimitWindow)

		// Check and increment rate limit in DynamoDB
		// Key format includes the hostname for per-hostname rate limiting
		count, err := rl.db.IncrementRateLimit(c.Context(), hostname)
		if err != nil {
			// Log error but allow request to proceed (fail open)
			LogAction(
				fmt.Sprintf("hostname:%s", hostname),
				"rate_limit_check_failed",
				err.Error(),
				"middleware:ratelimit",
			)
			return c.Next()
		}

		// Set rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", MaxRequestsPerWindow))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", max(0, MaxRequestsPerWindow-count)))
		c.Set("X-RateLimit-Reset", fmt.Sprintf("%d", windowStart.Add(RateLimitWindow).Unix()))

		if count > MaxRequestsPerWindow {
			LogAction(
				fmt.Sprintf("hostname:%s", hostname),
				"rate_limit_exceeded",
				fmt.Sprintf("count=%d, limit=%d", count, MaxRequestsPerWindow),
				"middleware:ratelimit",
			)

			// Return DynDNS2 format "abuse" response
			c.Set("Content-Type", "text/plain; charset=utf-8")
			return c.Status(fiber.StatusTooManyRequests).SendString("abuse")
		}

		return c.Next()
	}
}

// max returns the larger of two integers
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
