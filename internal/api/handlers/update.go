package handlers

import (
	"encoding/base64"
	"strings"

	"dynamic-route-53-dns/internal/service"

	"github.com/gofiber/fiber/v2"
)

// UpdateHandler handles DDNS update requests (DynDNS2 compatible)
type UpdateHandler struct {
	updateService *service.UpdateService
}

// NewUpdateHandler creates a new update handler
func NewUpdateHandler() *UpdateHandler {
	return &UpdateHandler{
		updateService: service.NewUpdateService(),
	}
}

// Update handles the DynDNS2 update endpoint
// GET /nic/update?hostname={hostname}&myip={ip}
// Authorization: Basic {base64(username:token)}
func (h *UpdateHandler) Update(c *fiber.Ctx) error {
	hostname := c.Query("hostname")
	ip := c.Query("myip")

	// If myip not provided, use source IP
	if ip == "" {
		ip = c.IP()
	}

	// Parse Basic Auth
	auth := c.Get("Authorization")
	if !strings.HasPrefix(auth, "Basic ") {
		return c.Status(401).SendString(service.ResponseBadAuth)
	}

	decoded, err := base64.StdEncoding.DecodeString(strings.TrimPrefix(auth, "Basic "))
	if err != nil {
		return c.Status(401).SendString(service.ResponseBadAuth)
	}

	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return c.Status(401).SendString(service.ResponseBadAuth)
	}

	// Username is ignored for DDNS updates, only token matters
	token := parts[1]

	// Get source IP and user agent for logging
	sourceIP := c.IP()
	userAgent := c.Get("User-Agent")

	// Process the update
	result := h.updateService.ProcessUpdate(c.Context(), hostname, token, ip, sourceIP, userAgent)

	// DynDNS2 response format
	if result.Code == service.ResponseGood || result.Code == service.ResponseNoChg {
		return c.SendString(result.Code + " " + result.IP)
	}

	// Error responses
	switch result.Code {
	case service.ResponseBadAuth:
		return c.Status(401).SendString(result.Code)
	case service.ResponseAbuse:
		return c.Status(429).SendString(result.Code)
	default:
		return c.SendString(result.Code)
	}
}

// GetIP returns the caller's IP address
func (h *UpdateHandler) GetIP(c *fiber.Ctx) error {
	return c.SendString(c.IP())
}
