package middleware

import (
	"dynamic-route-53-dns/internal/service"

	"github.com/gofiber/fiber/v2"
)

// RequireAuth middleware ensures the user is authenticated
func RequireAuth(authService *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionID := c.Cookies("session_id")
		if sessionID == "" {
			return c.Redirect("/login")
		}

		username, valid := authService.ValidateSession(c.Context(), sessionID)
		if !valid {
			// Clear invalid cookie
			c.Cookie(&fiber.Cookie{
				Name:     "session_id",
				Value:    "",
				Path:     "/",
				HTTPOnly: true,
				Secure:   true,
				SameSite: "Strict",
				MaxAge:   -1,
			})
			return c.Redirect("/login")
		}

		// Store username in context for handlers
		c.Locals("username", username)
		c.Locals("is_logged_in", true)

		return c.Next()
	}
}
