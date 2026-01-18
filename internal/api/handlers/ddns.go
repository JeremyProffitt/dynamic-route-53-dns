package handlers

import (
	"strconv"

	"github.com/dynamic-route-53-dns/internal/service"
	"github.com/gofiber/fiber/v2"
)

// DDNSHandler handles HTTP requests for DDNS record management
type DDNSHandler struct {
	ddnsService *service.DDNSService
	zoneService *service.ZoneService
}

// NewDDNSHandler creates a new DDNSHandler with the provided services
func NewDDNSHandler(ddnsService *service.DDNSService, zoneService *service.ZoneService) *DDNSHandler {
	return &DDNSHandler{
		ddnsService: ddnsService,
		zoneService: zoneService,
	}
}

// ListDDNS renders the DDNS records list
func (h *DDNSHandler) ListDDNS(c *fiber.Ctx) error {
	records, err := h.ddnsService.ListDDNSRecords(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to fetch DDNS records",
		})
	}

	return c.Render("ddns/list", fiber.Map{
		"Title":   "DDNS Records",
		"Records": records,
	})
}

// NewDDNSForm renders the form for creating a new DDNS record
func (h *DDNSHandler) NewDDNSForm(c *fiber.Ctx) error {
	zones, err := h.zoneService.ListZones(c.Context())
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to fetch hosted zones",
		})
	}

	return c.Render("ddns/new", fiber.Map{
		"Title": "New DDNS Record",
		"Zones": zones,
	})
}

// CreateDDNS creates a new DDNS record from form data
func (h *DDNSHandler) CreateDDNS(c *fiber.Ctx) error {
	hostname := c.FormValue("hostname")
	zoneID := c.FormValue("zone_id")
	zoneName := c.FormValue("zone_name")
	ttlStr := c.FormValue("ttl")

	// Validate required fields
	if hostname == "" || zoneID == "" || zoneName == "" {
		zones, _ := h.zoneService.ListZones(c.Context())
		return c.Status(fiber.StatusBadRequest).Render("ddns/new", fiber.Map{
			"Title": "New DDNS Record",
			"Error": "Hostname, Zone ID, and Zone Name are required",
			"Zones": zones,
		})
	}

	// Parse TTL with default value
	ttl := 60 // default TTL
	if ttlStr != "" {
		parsedTTL, err := strconv.Atoi(ttlStr)
		if err != nil || parsedTTL < 60 {
			zones, _ := h.zoneService.ListZones(c.Context())
			return c.Status(fiber.StatusBadRequest).Render("ddns/new", fiber.Map{
				"Title": "New DDNS Record",
				"Error": "TTL must be a valid number >= 60",
				"Zones": zones,
			})
		}
		ttl = parsedTTL
	}

	// Create the DDNS record and get the plaintext token
	plaintextToken, err := h.ddnsService.CreateDDNSRecord(c.Context(), hostname, zoneID, zoneName, ttl)
	if err != nil {
		zones, _ := h.zoneService.ListZones(c.Context())
		return c.Status(fiber.StatusInternalServerError).Render("ddns/new", fiber.Map{
			"Title": "New DDNS Record",
			"Error": "Failed to create DDNS record: " + err.Error(),
			"Zones": zones,
		})
	}

	// Render success page with the token (shown only once!)
	return c.Render("ddns/token", fiber.Map{
		"Title":          "DDNS Record Created",
		"Hostname":       hostname,
		"PlaintextToken": plaintextToken,
		"IsNew":          true,
	})
}

// GetDDNS renders the DDNS record detail view
func (h *DDNSHandler) GetDDNS(c *fiber.Ctx) error {
	hostname := c.Params("hostname")
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).Render("error", fiber.Map{
			"Error": "Hostname is required",
		})
	}

	record, err := h.ddnsService.GetDDNSRecord(c.Context(), hostname)
	if err != nil {
		return c.Status(fiber.StatusNotFound).Render("error", fiber.Map{
			"Error": "DDNS record not found",
		})
	}

	// Get recent history
	history, _ := h.ddnsService.GetUpdateHistory(c.Context(), hostname, 10)

	return c.Render("ddns/detail", fiber.Map{
		"Title":   hostname,
		"Record":  record,
		"History": history,
	})
}

// UpdateDDNS updates a DDNS record's TTL and enabled status
func (h *DDNSHandler) UpdateDDNS(c *fiber.Ctx) error {
	hostname := c.Params("hostname")
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).Render("error", fiber.Map{
			"Error": "Hostname is required",
		})
	}

	// Get current record
	record, err := h.ddnsService.GetDDNSRecord(c.Context(), hostname)
	if err != nil {
		return c.Status(fiber.StatusNotFound).Render("error", fiber.Map{
			"Error": "DDNS record not found",
		})
	}

	// Parse form values
	ttlStr := c.FormValue("ttl")
	enabledStr := c.FormValue("enabled")

	ttl := record.TTL
	if ttlStr != "" {
		parsedTTL, err := strconv.Atoi(ttlStr)
		if err != nil || parsedTTL < 60 {
			return c.Status(fiber.StatusBadRequest).Render("error", fiber.Map{
				"Error": "TTL must be a valid number >= 60",
			})
		}
		ttl = parsedTTL
	}

	enabled := record.Enabled
	if enabledStr != "" {
		enabled = enabledStr == "true" || enabledStr == "on" || enabledStr == "1"
	}

	err = h.ddnsService.UpdateDDNSRecord(c.Context(), hostname, ttl, enabled)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to update DDNS record: " + err.Error(),
		})
	}

	// Redirect back to detail page
	return c.Redirect("/ddns/" + hostname)
}

// DeleteDDNS deletes a DDNS record
func (h *DDNSHandler) DeleteDDNS(c *fiber.Ctx) error {
	hostname := c.Params("hostname")
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).Render("error", fiber.Map{
			"Error": "Hostname is required",
		})
	}

	err := h.ddnsService.DeleteDDNSRecord(c.Context(), hostname)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to delete DDNS record: " + err.Error(),
		})
	}

	// Redirect to DDNS list after successful deletion
	return c.Redirect("/ddns")
}

// RegenerateToken generates a new update token for a DDNS record
func (h *DDNSHandler) RegenerateToken(c *fiber.Ctx) error {
	hostname := c.Params("hostname")
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).Render("error", fiber.Map{
			"Error": "Hostname is required",
		})
	}

	plaintextToken, err := h.ddnsService.RegenerateToken(c.Context(), hostname)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to regenerate token: " + err.Error(),
		})
	}

	// Render the token display page (shown only once!)
	return c.Render("ddns/token", fiber.Map{
		"Title":          "Token Regenerated",
		"Hostname":       hostname,
		"PlaintextToken": plaintextToken,
		"IsNew":          false,
	})
}

// GetHistory renders the update history for a DDNS record
func (h *DDNSHandler) GetHistory(c *fiber.Ctx) error {
	hostname := c.Params("hostname")
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).Render("error", fiber.Map{
			"Error": "Hostname is required",
		})
	}

	history, err := h.ddnsService.GetUpdateHistory(c.Context(), hostname, 50)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).Render("error", fiber.Map{
			"Error": "Failed to fetch update history",
		})
	}

	return c.Render("ddns/history", fiber.Map{
		"Title":    hostname + " - Update History",
		"Hostname": hostname,
		"History":  history,
	})
}
