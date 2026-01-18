package middleware

import (
	"github.com/dynamic-route-53-dns/internal/service"
	"github.com/gofiber/fiber/v2"
)

// RequireAuth returns a middleware that checks for a valid session.
// If the session is invalid or missing, it redirects to the login page.
// On success, it sets the username in c.Locals("username").
func RequireAuth(authService *service.AuthService) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Get session ID from cookie
		sessionID, err := authService.SessionManager().GetSessionFromCookie(c)
		if err != nil {
			return c.Redirect("/login")
		}

		// Validate the session
		username, err := authService.ValidateSession(c.Context(), sessionID)
		if err != nil {
			// Clear invalid session cookie
			authService.SessionManager().ClearSessionCookie(c)
			return c.Redirect("/login")
		}

		// Set username in locals for use by handlers
		c.Locals("username", username)

		return c.Next()
	}
}
