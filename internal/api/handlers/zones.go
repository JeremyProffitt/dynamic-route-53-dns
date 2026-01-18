package handlers

import (
	"dynamic-route-53-dns/internal/service"

	"github.com/gofiber/fiber/v2"
)

// ZonesHandler handles zone-related routes
type ZonesHandler struct {
	zoneService *service.ZoneService
}

// NewZonesHandler creates a new zones handler
func NewZonesHandler() *ZonesHandler {
	return &ZonesHandler{
		zoneService: service.NewZoneService(),
	}
}

// ListZones renders the zones list page
func (h *ZonesHandler) ListZones(c *fiber.Ctx) error {
	zones, err := h.zoneService.ListZones(c.Context())
	if err != nil {
		return c.Render("zones/list", fiber.Map{
			"PageTitle":   "Zones - Dynamic DNS",
			"CurrentPath": "/zones",
			"IsLoggedIn":  true,
			"Username":    c.Locals("username"),
			"CSRFToken":   c.Locals("csrf_token"),
			"FlashError":  "Failed to load zones: " + err.Error(),
		})
	}

	return c.Render("zones/list", fiber.Map{
		"PageTitle":   "Zones - Dynamic DNS",
		"CurrentPath": "/zones",
		"IsLoggedIn":  true,
		"Username":    c.Locals("username"),
		"CSRFToken":   c.Locals("csrf_token"),
		"Zones":       zones,
	})
}

// ZoneDetail renders the zone detail page with records
func (h *ZonesHandler) ZoneDetail(c *fiber.Ctx) error {
	zoneID := c.Params("zoneId")

	zone, err := h.zoneService.GetZone(c.Context(), zoneID)
	if err != nil || zone == nil {
		return c.Redirect("/zones")
	}

	records, err := h.zoneService.GetZoneRecords(c.Context(), zoneID)
	if err != nil {
		return c.Render("zones/detail", fiber.Map{
			"PageTitle":   zone.Name + " - Dynamic DNS",
			"CurrentPath": "/zones",
			"IsLoggedIn":  true,
			"Username":    c.Locals("username"),
			"CSRFToken":   c.Locals("csrf_token"),
			"Zone":        zone,
			"FlashError":  "Failed to load records: " + err.Error(),
		})
	}

	return c.Render("zones/detail", fiber.Map{
		"PageTitle":   zone.Name + " - Dynamic DNS",
		"CurrentPath": "/zones",
		"IsLoggedIn":  true,
		"Username":    c.Locals("username"),
		"CSRFToken":   c.Locals("csrf_token"),
		"Zone":        zone,
		"Records":     records,
	})
}
