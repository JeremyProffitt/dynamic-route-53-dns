package api

import (
	"github.com/gofiber/fiber/v2"

	"github.com/dynamic-route-53-dns/internal/api/handlers"
	"github.com/dynamic-route-53-dns/internal/api/middleware"
	"github.com/dynamic-route-53-dns/internal/service"
)

// Handlers contains all HTTP handlers for the application.
type Handlers struct {
	Auth   *handlers.AuthHandler
	Zone   *handlers.ZoneHandler
	DDNS   *handlers.DDNSHandler
	Update *handlers.UpdateHandler
}

// Middleware contains all middleware for the application.
type Middleware struct {
	RateLimit *middleware.RateLimitMiddleware
}

// SetupRoutes configures all routes for the application.
func SetupRoutes(app *fiber.App, h Handlers, m Middleware, authService *service.AuthService) {
	// Apply global logging middleware
	app.Use(middleware.RequestLogger())

	// Root redirect
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/ddns", fiber.StatusFound)
	})

	// Public routes
	app.Get("/ip", h.Auth.GetIP)
	app.Get("/login", h.Auth.GetLogin)
	app.Post("/login", h.Auth.PostLogin)
	app.Post("/logout", h.Auth.PostLogout)

	// DDNS update endpoint (public, with rate limiting)
	app.Get("/nic/update", m.RateLimit.DDNSUpdateRateLimit(), h.Update.HandleUpdate)

	// Protected routes group
	protected := app.Group("", middleware.RequireAuth(authService))

	// Apply CSRF middleware to protected routes
	protected.Use(middleware.CSRFProtection())

	// Zone routes
	protected.Get("/zones", h.Zone.ListZones)
	protected.Get("/zones/:zoneId", h.Zone.GetZone)
	protected.Get("/zones/:zoneId/records", h.Zone.GetZoneRecords)

	// DDNS management routes
	protected.Get("/ddns", h.DDNS.ListDDNS)
	protected.Get("/ddns/new", h.DDNS.NewDDNSForm)
	protected.Post("/ddns", h.DDNS.CreateDDNS)
	protected.Get("/ddns/:hostname", h.DDNS.GetDDNS)
	protected.Put("/ddns/:hostname", h.DDNS.UpdateDDNS)
	protected.Post("/ddns/:hostname", h.DDNS.UpdateDDNS) // Also accept POST for form submissions
	protected.Delete("/ddns/:hostname", h.DDNS.DeleteDDNS)
	protected.Post("/ddns/:hostname/regenerate-token", h.DDNS.RegenerateToken)
	protected.Get("/ddns/:hostname/history", h.DDNS.GetHistory)
}
