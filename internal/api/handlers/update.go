package handlers

import (
	"encoding/base64"
	"strings"

	"github.com/dynamic-route-53-dns/internal/service"
	"github.com/gofiber/fiber/v2"
)

// UpdateHandler handles DynDNS2-compatible update requests
type UpdateHandler struct {
	updateService *service.UpdateService
}

// NewUpdateHandler creates a new UpdateHandler with the provided service
func NewUpdateHandler(updateService *service.UpdateService) *UpdateHandler {
	return &UpdateHandler{
		updateService: updateService,
	}
}

// HandleUpdate processes a DynDNS2-compatible update request
// Endpoint: GET /nic/update?hostname={hostname}&myip={ip}
// Authorization: Basic {base64(username:token)}
//
// Response codes:
// - good {ip} - Update successful
// - nochg {ip} - IP unchanged
// - nohost - Hostname not found
// - badauth - Invalid credentials
// - abuse - Too many requests
func (h *UpdateHandler) HandleUpdate(c *fiber.Ctx) error {
	// Set content type to plain text for DynDNS2 compatibility
	c.Set("Content-Type", "text/plain; charset=utf-8")

	// Parse Basic Auth header
	_, token, ok := parseBasicAuth(c.Get("Authorization"))
	if !ok {
		return c.Status(fiber.StatusUnauthorized).SendString("badauth")
	}

	// Get hostname from query parameter
	hostname := c.Query("hostname")
	if hostname == "" {
		return c.Status(fiber.StatusBadRequest).SendString("nohost")
	}

	// Get source IP from X-Forwarded-For or request IP
	sourceIP := getSourceIP(c)

	// Get myip from query parameter (optional, defaults to source IP)
	providedIP := c.Query("myip")

	// Get user agent for logging
	userAgent := c.Get("User-Agent")

	// Process the update
	response, err := h.updateService.ProcessUpdate(c.Context(), hostname, providedIP, sourceIP, token, userAgent)
	if err != nil {
		// The response string contains the appropriate DynDNS2 response code
		// Just return it directly
		return c.SendString(response)
	}

	return c.SendString(response)
}

// parseBasicAuth parses the Authorization header for Basic authentication
// Returns username, token, and whether parsing was successful
func parseBasicAuth(auth string) (username, token string, ok bool) {
	if auth == "" {
		return "", "", false
	}

	// Basic auth header format: "Basic base64(username:password)"
	const prefix = "Basic "
	if !strings.HasPrefix(auth, prefix) {
		return "", "", false
	}

	// Decode base64
	decoded, err := base64.StdEncoding.DecodeString(auth[len(prefix):])
	if err != nil {
		return "", "", false
	}

	// Split username:token
	credentials := string(decoded)
	colonIdx := strings.IndexByte(credentials, ':')
	if colonIdx < 0 {
		return "", "", false
	}

	return credentials[:colonIdx], credentials[colonIdx+1:], true
}

// getSourceIP extracts the client's IP address from the request
// Checks X-Forwarded-For header first (for proxy/load balancer scenarios),
// then falls back to the direct connection IP
func getSourceIP(c *fiber.Ctx) string {
	// Check X-Forwarded-For header (common with proxies/load balancers)
	xff := c.Get("X-Forwarded-For")
	if xff != "" {
		// X-Forwarded-For can contain multiple IPs: "client, proxy1, proxy2"
		// The first IP is the original client
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			clientIP := strings.TrimSpace(ips[0])
			if clientIP != "" {
				return clientIP
			}
		}
	}

	// Check X-Real-IP header (used by some proxies)
	xri := c.Get("X-Real-IP")
	if xri != "" {
		return xri
	}

	// Fall back to direct connection IP
	return c.IP()
}
