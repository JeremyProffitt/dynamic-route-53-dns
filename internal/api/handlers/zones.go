package handlers

import (
	"github.com/dynamic-route-53-dns/internal/service"
	"github.com/gofiber/fiber/v2"
)

// ZoneHandler handles HTTP requests for DNS zone management
type ZoneHandler struct {
	zoneService *service.ZoneService
}

// NewZoneHandler creates a new ZoneHandler with the provided ZoneService
func NewZoneHandler(service *service.ZoneService) *ZoneHandler {
	return &ZoneHandler{
		zoneService: service,
	}
}

// ListZones renders the zones list template with all hosted zones
func (h *ZoneHandler) ListZones(c *fiber.Ctx) error {
	zones, err := h.zoneService.ListZones(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to fetch hosted zones",
		})
	}

	return c.Render("zones/list", fiber.Map{
		"Title": "Hosted Zones",
		"Zones": zones,
	})
}

// GetZone renders the zone detail view with its DNS records
func (h *ZoneHandler) GetZone(c *fiber.Ctx) error {
	zoneId := c.Params("zoneId")
	if zoneId == "" {
		return c.Status(fiber.StatusBadRequest).Render("error", fiber.Map{
			"Error": "Zone ID is required",
		})
	}

	zone, err := h.zoneService.GetZone(c.Context(), zoneId)
	if err != nil {
		return c.Status(fiber.StatusNotFound).Render("error", fiber.Map{
			"Error": "Zone not found",
		})
	}

	records, err := h.zoneService.GetZoneRecords(c.Context(), zoneId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to fetch zone records",
		})
	}

	return c.Render("zones/detail", fiber.Map{
		"Title":   zone.Name,
		"Zone":    zone,
		"Records": records,
	})
}

// GetZoneRecords returns an HTMX partial for the records table
func (h *ZoneHandler) GetZoneRecords(c *fiber.Ctx) error {
	zoneId := c.Params("zoneId")
	if zoneId == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Zone ID is required")
	}

	records, err := h.zoneService.GetZoneRecords(c.Context(), zoneId)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to fetch records")
	}

	return c.Render("zones/partials/records_table", fiber.Map{
		"Records": records,
	})
}
