package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"sync"

	"github.com/gofiber/fiber/v2"
)

// CSRFConfig configuration for CSRF middleware
type CSRFConfig struct {
	TokenLength int
	CookieName  string
	HeaderName  string
	FormField   string
}

// DefaultCSRFConfig default CSRF configuration
var DefaultCSRFConfig = CSRFConfig{
	TokenLength: 32,
	CookieName:  "csrf_token",
	HeaderName:  "X-CSRF-Token",
	FormField:   "_csrf",
}

// tokenPool for generating tokens
var tokenPool = sync.Pool{
	New: func() interface{} {
		return make([]byte, DefaultCSRFConfig.TokenLength)
	},
}

// generateToken generates a new CSRF token
func generateToken() string {
	b := tokenPool.Get().([]byte)
	defer tokenPool.Put(b)

	if _, err := rand.Read(b); err != nil {
		return ""
	}

	return base64.URLEncoding.EncodeToString(b)
}

// CSRF middleware provides CSRF protection
func CSRF() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get or generate CSRF token
		token := c.Cookies(DefaultCSRFConfig.CookieName)
		if token == "" {
			token = generateToken()
			c.Cookie(&fiber.Cookie{
				Name:     DefaultCSRFConfig.CookieName,
				Value:    token,
				Path:     "/",
				HTTPOnly: false, // Needs to be readable by JS for HTMX
				Secure:   true,
				SameSite: "Strict",
			})
		}

		// Store token in locals for templates
		c.Locals("csrf_token", token)

		// Skip validation for safe methods
		method := c.Method()
		if method == "GET" || method == "HEAD" || method == "OPTIONS" {
			return c.Next()
		}

		// Skip CSRF for update endpoint (uses Basic Auth)
		if c.Path() == "/nic/update" {
			return c.Next()
		}

		// Validate token for unsafe methods
		submittedToken := c.FormValue(DefaultCSRFConfig.FormField)
		if submittedToken == "" {
			submittedToken = c.Get(DefaultCSRFConfig.HeaderName)
		}

		if subtle.ConstantTimeCompare([]byte(token), []byte(submittedToken)) != 1 {
			return c.Status(403).SendString("Invalid CSRF token")
		}

		return c.Next()
	}
}
