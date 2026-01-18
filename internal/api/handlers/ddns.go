package handlers

import (
	"strconv"

	"dynamic-route-53-dns/internal/service"

	"github.com/gofiber/fiber/v2"
)

// DDNSHandler handles DDNS management routes
type DDNSHandler struct {
	ddnsService *service.DDNSService
	zoneService *service.ZoneService
}

// NewDDNSHandler creates a new DDNS handler
func NewDDNSHandler() *DDNSHandler {
	return &DDNSHandler{
		ddnsService: service.NewDDNSService(),
		zoneService: service.NewZoneService(),
	}
}

// ListDDNS renders the DDNS list page
func (h *DDNSHandler) ListDDNS(c *fiber.Ctx) error {
	records, err := h.ddnsService.ListDDNSRecords(c.Context())
	if err != nil {
		return c.Render("ddns/list", fiber.Map{
			"PageTitle":   "DDNS Records - Dynamic DNS",
			"CurrentPath": "/ddns",
			"IsLoggedIn":  true,
			"Username":    c.Locals("username"),
			"CSRFToken":   c.Locals("csrf_token"),
			"FlashError":  "Failed to load records: " + err.Error(),
		})
	}

	return c.Render("ddns/list", fiber.Map{
		"PageTitle":   "DDNS Records - Dynamic DNS",
		"CurrentPath": "/ddns",
		"IsLoggedIn":  true,
		"Username":    c.Locals("username"),
		"CSRFToken":   c.Locals("csrf_token"),
		"Records":     records,
	})
}

// NewDDNSForm renders the new DDNS form
func (h *DDNSHandler) NewDDNSForm(c *fiber.Ctx) error {
	zones, err := h.zoneService.ListZones(c.Context())
	if err != nil {
		return c.Render("ddns/new", fiber.Map{
			"PageTitle":   "New DDNS Record - Dynamic DNS",
			"CurrentPath": "/ddns",
			"IsLoggedIn":  true,
			"Username":    c.Locals("username"),
			"CSRFToken":   c.Locals("csrf_token"),
			"FlashError":  "Failed to load zones: " + err.Error(),
		})
	}

	return c.Render("ddns/new", fiber.Map{
		"PageTitle":   "New DDNS Record - Dynamic DNS",
		"CurrentPath": "/ddns",
		"IsLoggedIn":  true,
		"Username":    c.Locals("username"),
		"CSRFToken":   c.Locals("csrf_token"),
		"Zones":       zones,
		"DefaultTTL":  60,
	})
}

// CreateDDNS creates a new DDNS record
func (h *DDNSHandler) CreateDDNS(c *fiber.Ctx) error {
	hostname := c.FormValue("hostname")
	zoneID := c.FormValue("zone_id")
	ttlStr := c.FormValue("ttl")

	ttl, err := strconv.ParseInt(ttlStr, 10, 64)
	if err != nil {
		ttl = 60
	}

	result := h.ddnsService.CreateDDNSRecord(c.Context(), &service.DDNSConfig{
		Hostname: hostname,
		ZoneID:   zoneID,
		TTL:      ttl,
	})

	if !result.Success {
		zones, _ := h.zoneService.ListZones(c.Context())
		return c.Render("ddns/new", fiber.Map{
			"PageTitle":   "New DDNS Record - Dynamic DNS",
			"CurrentPath": "/ddns",
			"IsLoggedIn":  true,
			"Username":    c.Locals("username"),
			"CSRFToken":   c.Locals("csrf_token"),
			"FlashError":  result.Error,
			"Zones":       zones,
			"Hostname":    hostname,
			"ZoneID":      zoneID,
			"TTL":         ttl,
		})
	}

	// Show the token page (token is only shown once)
	return c.Render("ddns/token", fiber.Map{
		"PageTitle":   "Token Created - Dynamic DNS",
		"CurrentPath": "/ddns",
		"IsLoggedIn":  true,
		"Username":    c.Locals("username"),
		"CSRFToken":   c.Locals("csrf_token"),
		"Hostname":    hostname,
		"Token":       result.Token,
		"ServerURL":   c.Hostname(),
	})
}

// DDNSDetail renders the DDNS detail page
func (h *DDNSHandler) DDNSDetail(c *fiber.Ctx) error {
	hostname := c.Params("hostname")

	record, err := h.ddnsService.GetDDNSRecord(c.Context(), hostname)
	if err != nil || record == nil {
		return c.Redirect("/ddns")
	}

	history, _ := h.ddnsService.GetUpdateHistory(c.Context(), hostname, 50)

	return c.Render("ddns/detail", fiber.Map{
		"PageTitle":   hostname + " - Dynamic DNS",
		"CurrentPath": "/ddns",
		"IsLoggedIn":  true,
		"Username":    c.Locals("username"),
		"CSRFToken":   c.Locals("csrf_token"),
		"Record":      record,
		"History":     history,
		"ServerURL":   c.Hostname(),
	})
}

// UpdateDDNS updates a DDNS record
func (h *DDNSHandler) UpdateDDNS(c *fiber.Ctx) error {
	hostname := c.Params("hostname")
	enabled := c.FormValue("enabled") == "on"
	ttlStr := c.FormValue("ttl")

	ttl, _ := strconv.ParseInt(ttlStr, 10, 64)

	err := h.ddnsService.UpdateDDNSRecord(c.Context(), hostname, enabled, ttl)
	if err != nil {
		record, _ := h.ddnsService.GetDDNSRecord(c.Context(), hostname)
		history, _ := h.ddnsService.GetUpdateHistory(c.Context(), hostname, 50)
		return c.Render("ddns/detail", fiber.Map{
			"PageTitle":   hostname + " - Dynamic DNS",
			"CurrentPath": "/ddns",
			"IsLoggedIn":  true,
			"Username":    c.Locals("username"),
			"CSRFToken":   c.Locals("csrf_token"),
			"Record":      record,
			"History":     history,
			"FlashError":  "Failed to update: " + err.Error(),
			"ServerURL":   c.Hostname(),
		})
	}

	record, _ := h.ddnsService.GetDDNSRecord(c.Context(), hostname)
	history, _ := h.ddnsService.GetUpdateHistory(c.Context(), hostname, 50)
	return c.Render("ddns/detail", fiber.Map{
		"PageTitle":    hostname + " - Dynamic DNS",
		"CurrentPath":  "/ddns",
		"IsLoggedIn":   true,
		"Username":     c.Locals("username"),
		"CSRFToken":    c.Locals("csrf_token"),
		"Record":       record,
		"History":      history,
		"FlashSuccess": "Record updated successfully",
		"ServerURL":    c.Hostname(),
	})
}

// DeleteDDNS deletes a DDNS record
func (h *DDNSHandler) DeleteDDNS(c *fiber.Ctx) error {
	hostname := c.Params("hostname")

	if err := h.ddnsService.DeleteDDNSRecord(c.Context(), hostname); err != nil {
		return c.Status(500).SendString("Failed to delete record")
	}

	return c.Redirect("/ddns")
}

// RegenerateToken regenerates the update token
func (h *DDNSHandler) RegenerateToken(c *fiber.Ctx) error {
	hostname := c.Params("hostname")

	token, err := h.ddnsService.RegenerateToken(c.Context(), hostname)
	if err != nil {
		return c.Status(500).SendString("Failed to regenerate token")
	}

	return c.Render("ddns/token", fiber.Map{
		"PageTitle":   "Token Regenerated - Dynamic DNS",
		"CurrentPath": "/ddns",
		"IsLoggedIn":  true,
		"Username":    c.Locals("username"),
		"CSRFToken":   c.Locals("csrf_token"),
		"Hostname":    hostname,
		"Token":       token,
		"Regenerated": true,
		"ServerURL":   c.Hostname(),
	})
}

// DDNSHistory returns the update history (HTMX partial)
func (h *DDNSHandler) DDNSHistory(c *fiber.Ctx) error {
	hostname := c.Params("hostname")

	history, err := h.ddnsService.GetUpdateHistory(c.Context(), hostname, 50)
	if err != nil {
		return c.Status(500).SendString("Failed to load history")
	}

	// For HTMX partial response
	c.Set("Content-Type", "text/html")

	html := "<table class=\"min-w-full divide-y divide-gray-700\">"
	html += "<thead><tr>"
	html += "<th class=\"px-4 py-2 text-left text-gray-300\">Time</th>"
	html += "<th class=\"px-4 py-2 text-left text-gray-300\">Previous IP</th>"
	html += "<th class=\"px-4 py-2 text-left text-gray-300\">New IP</th>"
	html += "<th class=\"px-4 py-2 text-left text-gray-300\">Source</th>"
	html += "<th class=\"px-4 py-2 text-left text-gray-300\">Status</th>"
	html += "</tr></thead><tbody>"

	for _, log := range history {
		html += "<tr class=\"border-b border-gray-700\">"
		html += "<td class=\"px-4 py-2 text-gray-300\">" + log.Timestamp.Format("2006-01-02 15:04:05") + "</td>"
		html += "<td class=\"px-4 py-2 text-gray-300\">" + log.PreviousIP + "</td>"
		html += "<td class=\"px-4 py-2 text-gray-300\">" + log.NewIP + "</td>"
		html += "<td class=\"px-4 py-2 text-gray-300\">" + log.SourceIP + "</td>"
		html += "<td class=\"px-4 py-2 text-gray-300\">" + log.Status + "</td>"
		html += "</tr>"
	}

	html += "</tbody></table>"

	if len(history) == 0 {
		html = "<p class=\"text-gray-400 text-center py-4\">No update history yet</p>"
	}

	return c.SendString(html)
}
