package handlers

import (
	"github.com/dynamic-route-53-dns/internal/service"
	"github.com/gofiber/fiber/v2"
)

// AuthHandler handles authentication-related HTTP requests.
type AuthHandler struct {
	service *service.AuthService
}

// NewAuthHandler creates a new AuthHandler with the given AuthService.
func NewAuthHandler(service *service.AuthService) *AuthHandler {
	return &AuthHandler{
		service: service,
	}
}

// GetLogin renders the login page.
func (h *AuthHandler) GetLogin(c *fiber.Ctx) error {
	// Check if already logged in
	sessionID, err := h.service.SessionManager().GetSessionFromCookie(c)
	if err == nil && sessionID != "" {
		_, err := h.service.ValidateSession(c.Context(), sessionID)
		if err == nil {
			// Already authenticated, redirect to dashboard
			return c.Redirect("/zones")
		}
	}

	// Render login page
	return c.Render("auth/login", fiber.Map{
		"Title": "Login",
		"Error": c.Query("error"),
	})
}

// PostLogin processes the login form submission.
func (h *AuthHandler) PostLogin(c *fiber.Ctx) error {
	username := c.FormValue("username")
	password := c.FormValue("password")
	clientIP := c.IP()

	// Attempt login
	sessionID, err := h.service.Login(c.Context(), username, password, clientIP)
	if err != nil {
		// Redirect back to login with error
		errorMsg := "Invalid username or password"
		if err == service.ErrIPLocked {
			errorMsg = "Too many failed attempts. Please try again later."
		}
		return c.Redirect("/login?error=" + errorMsg)
	}

	// Set session cookie
	h.service.SessionManager().SetSessionCookie(c, sessionID)

	// Redirect to dashboard
	return c.Redirect("/zones")
}

// PostLogout handles the logout request.
func (h *AuthHandler) PostLogout(c *fiber.Ctx) error {
	// Get session ID from cookie
	sessionID, err := h.service.SessionManager().GetSessionFromCookie(c)
	if err == nil && sessionID != "" {
		// Destroy the session
		_ = h.service.Logout(c.Context(), sessionID)
	}

	// Clear the session cookie
	h.service.SessionManager().ClearSessionCookie(c)

	// Redirect to login page
	return c.Redirect("/login")
}

// GetIP returns the caller's IP address as plain text.
// This endpoint is useful for testing and for clients to determine their public IP.
func (h *AuthHandler) GetIP(c *fiber.Ctx) error {
	return c.SendString(c.IP())
}
