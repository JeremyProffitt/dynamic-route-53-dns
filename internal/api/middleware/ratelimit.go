package middleware

import (
	"fmt"

	"dynamic-route-53-dns/internal/database"

	"github.com/gofiber/fiber/v2"
)

// RateLimitConfig configuration for rate limiting
type RateLimitConfig struct {
	Max           int   // Maximum requests per window
	WindowSeconds int64 // Window duration in seconds
	KeyGenerator  func(*fiber.Ctx) string
}

// DefaultRateLimitConfig default rate limit configuration
var DefaultRateLimitConfig = RateLimitConfig{
	Max:           60,   // 60 requests
	WindowSeconds: 3600, // per hour
	KeyGenerator: func(c *fiber.Ctx) string {
		return c.IP()
	},
}

// RateLimit middleware provides rate limiting
func RateLimit(config ...RateLimitConfig) fiber.Handler {
	cfg := DefaultRateLimitConfig
	if len(config) > 0 {
		cfg = config[0]
	}

	return func(c *fiber.Ctx) error {
		key := cfg.KeyGenerator(c)

		count, exceeded, err := database.IncrementRateLimit(
			c.Context(),
			fmt.Sprintf("ratelimit:%s", key),
			cfg.Max,
			cfg.WindowSeconds,
		)
		if err != nil {
			// Log error but don't block request
			fmt.Printf("Rate limit error: %v\n", err)
			return c.Next()
		}

		// Set rate limit headers
		c.Set("X-RateLimit-Limit", fmt.Sprintf("%d", cfg.Max))
		c.Set("X-RateLimit-Remaining", fmt.Sprintf("%d", cfg.Max-count))

		if exceeded {
			c.Set("Retry-After", fmt.Sprintf("%d", cfg.WindowSeconds))
			return c.Status(429).SendString("Too many requests")
		}

		return c.Next()
	}
}

// DDNSRateLimit rate limiter specifically for DDNS updates
func DDNSRateLimit() fiber.Handler {
	return RateLimit(RateLimitConfig{
		Max:           60,   // 60 requests
		WindowSeconds: 3600, // per hour
		KeyGenerator: func(c *fiber.Ctx) string {
			hostname := c.Query("hostname")
			return fmt.Sprintf("ddns:%s", hostname)
		},
	})
}
