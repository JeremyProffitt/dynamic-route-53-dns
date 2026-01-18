package api

import (
	"dynamic-route-53-dns/internal/api/handlers"
	"dynamic-route-53-dns/internal/api/middleware"
	"dynamic-route-53-dns/internal/service"

	"github.com/gofiber/fiber/v2"
)

// SetupRoutes configures all application routes
func SetupRoutes(app *fiber.App) {
	// Initialize handlers
	authHandler := handlers.NewAuthHandler()
	zonesHandler := handlers.NewZonesHandler()
	ddnsHandler := handlers.NewDDNSHandler()
	updateHandler := handlers.NewUpdateHandler()

	// Initialize auth service for middleware
	authService := service.NewAuthService()

	// Apply global middleware
	app.Use(middleware.Logging())
	app.Use(middleware.CSRF())

	// Public routes
	app.Get("/", func(c *fiber.Ctx) error {
		return c.Redirect("/login")
	})
	app.Get("/login", authHandler.LoginPage)
	app.Post("/login", authHandler.Login)
	app.Post("/logout", authHandler.Logout)

	// IP endpoint (public)
	app.Get("/ip", updateHandler.GetIP)

	// DynDNS2 update endpoint (uses Basic Auth)
	app.Get("/nic/update", updateHandler.Update)

	// Protected routes - require authentication
	protected := app.Group("", middleware.RequireAuth(authService))

	// Zone routes
	protected.Get("/zones", zonesHandler.ListZones)
	protected.Get("/zones/:zoneId", zonesHandler.ZoneDetail)

	// DDNS management routes
	protected.Get("/ddns", ddnsHandler.ListDDNS)
	protected.Get("/ddns/new", ddnsHandler.NewDDNSForm)
	protected.Post("/ddns", ddnsHandler.CreateDDNS)
	protected.Get("/ddns/:hostname", ddnsHandler.DDNSDetail)
	protected.Put("/ddns/:hostname", ddnsHandler.UpdateDDNS)
	protected.Post("/ddns/:hostname", ddnsHandler.UpdateDDNS) // HTML forms only support GET/POST
	protected.Delete("/ddns/:hostname", ddnsHandler.DeleteDDNS)
	protected.Post("/ddns/:hostname/delete", ddnsHandler.DeleteDDNS) // HTML forms only support GET/POST
	protected.Post("/ddns/:hostname/regenerate-token", ddnsHandler.RegenerateToken)
	protected.Get("/ddns/:hostname/history", ddnsHandler.DDNSHistory)
}
