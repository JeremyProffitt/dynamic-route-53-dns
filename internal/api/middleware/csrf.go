package middleware

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
)

const (
	// CSRFTokenLength is the length of the generated CSRF token in bytes
	CSRFTokenLength = 32
	// CSRFCookieName is the name of the cookie storing the CSRF token
	CSRFCookieName = "csrf_token"
	// CSRFHeaderName is the header name for CSRF token submission
	CSRFHeaderName = "X-CSRF-Token"
	// CSRFFormFieldName is the form field name for CSRF token submission
	CSRFFormFieldName = "csrf_token"
	// CSRFTokenExpiry is the duration for which a CSRF token is valid
	CSRFTokenExpiry = 24 * time.Hour
)

// GenerateCSRFToken generates a cryptographically secure random token
func GenerateCSRFToken() string {
	bytes := make([]byte, CSRFTokenLength)
	if _, err := rand.Read(bytes); err != nil {
		// Fallback to a less secure method if crypto/rand fails
		// This should never happen in practice
		return base64.URLEncoding.EncodeToString([]byte(time.Now().String()))
	}
	return base64.URLEncoding.EncodeToString(bytes)
}

// CSRFProtection returns a Fiber handler that provides CSRF protection
// It generates tokens, stores them in cookies, and validates on state-changing requests
// The /nic/update endpoint is skipped as it uses Basic Auth
func CSRFProtection() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Skip CSRF protection for /nic/update endpoint (uses Basic Auth)
		if strings.HasPrefix(c.Path(), "/nic/update") {
			return c.Next()
		}

		// Get or generate CSRF token
		token := c.Cookies(CSRFCookieName)
		if token == "" {
			token = GenerateCSRFToken()
			c.Cookie(&fiber.Cookie{
				Name:     CSRFCookieName,
				Value:    token,
				Expires:  time.Now().Add(CSRFTokenExpiry),
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Strict",
				Path:     "/",
			})
		}

		// Store token in locals for template access
		c.Locals("csrf_token", token)

		// For safe methods, just set the token and continue
		method := c.Method()
		if method == fiber.MethodGet || method == fiber.MethodHead || method == fiber.MethodOptions {
			return c.Next()
		}

		// For state-changing methods (POST, PUT, DELETE, PATCH), validate the token
		submittedToken := getSubmittedToken(c)
		if submittedToken == "" {
			LogActionWithContext(c, "csrf_validation_failed", "no CSRF token provided", "middleware:csrf")
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "CSRF token missing",
			})
		}

		// Constant-time comparison to prevent timing attacks
		if subtle.ConstantTimeCompare([]byte(token), []byte(submittedToken)) != 1 {
			LogActionWithContext(c, "csrf_validation_failed", "CSRF token mismatch", "middleware:csrf")
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"error": "CSRF token invalid",
			})
		}

		return c.Next()
	}
}

// getSubmittedToken retrieves the CSRF token from the request
// It checks the header first, then the form field
func getSubmittedToken(c *fiber.Ctx) string {
	// Check header first (for AJAX requests)
	token := c.Get(CSRFHeaderName)
	if token != "" {
		return token
	}

	// Check form field (for regular form submissions)
	return c.FormValue(CSRFFormFieldName)
}

// GetCSRFToken retrieves the CSRF token from the context for use in templates
func GetCSRFToken(c *fiber.Ctx) string {
	token, ok := c.Locals("csrf_token").(string)
	if !ok {
		return ""
	}
	return token
}

// CSRFTokenField returns an HTML hidden input field with the CSRF token
// This is a helper for templates
func CSRFTokenField(c *fiber.Ctx) string {
	token := GetCSRFToken(c)
	return `<input type="hidden" name="` + CSRFFormFieldName + `" value="` + token + `">`
}
