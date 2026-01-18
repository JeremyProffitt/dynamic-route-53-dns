package handlers

import (
	"dynamic-route-53-dns/internal/service"

	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles authentication routes
type AuthHandler struct {
	authService *service.AuthService
}

// NewAuthHandler creates a new auth handler
func NewAuthHandler() *AuthHandler {
	return &AuthHandler{
		authService: service.NewAuthService(),
	}
}

// LoginPage renders the login page
func (h *AuthHandler) LoginPage(c *fiber.Ctx) error {
	// Check if already logged in
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		if _, valid := h.authService.ValidateSession(c.Context(), sessionID); valid {
			return c.Redirect("/zones")
		}
	}

	return c.Render("auth/login", fiber.Map{
		"PageTitle":   "Login - Dynamic DNS",
		"CurrentPath": "/login",
		"CSRFToken":   c.Locals("csrf_token"),
	})
}

// Login processes login requests
func (h *AuthHandler) Login(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")

	result := h.authService.Login(c.Context(), username, password)

	if !result.Success {
		return c.Render("auth/login", fiber.Map{
			"PageTitle":   "Login - Dynamic DNS",
			"CurrentPath": "/login",
			"CSRFToken":   c.Locals("csrf_token"),
			"FlashError":  result.Error,
			"Username":    username,
		})
	}

	// Set session cookie
	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    result.SessionID,
		Path:     "/",
		HTTPOnly: true,
		Secure:   true,
		SameSite: "Strict",
		MaxAge:   86400, // 24 hours
	})

	return c.Redirect("/zones")
}

// Logout handles logout requests
func (h *AuthHandler) Logout(c *fiber.Ctx) error {
	sessionID := c.Cookies("session_id")
	if sessionID != "" {
		_ = h.authService.Logout(c.Context(), sessionID)
	}

	// Clear cookie
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
