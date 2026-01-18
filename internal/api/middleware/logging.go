package middleware

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/gofiber/fiber/v2"
)

// LogEntry represents a structured log entry
type LogEntry struct {
	Timestamp string `json:"timestamp"`
	Who       string `json:"who"`
	What      string `json:"what"`
	Why       string `json:"why"`
	Where     string `json:"where"`
	Method    string `json:"method"`
	Path      string `json:"path"`
	Status    int    `json:"status"`
	Latency   string `json:"latency"`
	IP        string `json:"ip"`
	UserAgent string `json:"user_agent"`
}

// Logging middleware provides structured logging
func Logging() fiber.Handler {
	return func(c *fiber.Ctx) error {
		start := time.Now()

		// Process request
		err := c.Next()

		// Build log entry
		entry := LogEntry{
			Timestamp: time.Now().UTC().Format(time.RFC3339),
			Method:    c.Method(),
			Path:      c.Path(),
			Status:    c.Response().StatusCode(),
			Latency:   time.Since(start).String(),
			IP:        c.IP(),
			UserAgent: c.Get("User-Agent"),
			Where:     "ddns:http",
		}

		// Add user info if available
		if username, ok := c.Locals("username").(string); ok && username != "" {
			entry.Who = "user:" + username
		} else {
			entry.Who = "anonymous"
		}

		// Determine what happened
		switch {
		case entry.Status >= 500:
			entry.What = "server_error"
			entry.Why = "internal server error occurred"
		case entry.Status >= 400:
			entry.What = "client_error"
			entry.Why = "client request failed"
		case entry.Status >= 300:
			entry.What = "redirect"
			entry.Why = "request redirected"
		default:
			entry.What = "request_completed"
			entry.Why = "successful request"
		}

		// Special handling for specific paths
		switch c.Path() {
		case "/login":
			if c.Method() == "POST" {
				if entry.Status == 302 {
					entry.What = "login_success"
					entry.Why = "user authenticated successfully"
				} else {
					entry.What = "login_failed"
					entry.Why = "authentication failed"
				}
			}
		case "/logout":
			entry.What = "logout"
			entry.Why = "user logged out"
		case "/nic/update":
			if entry.Status == 200 {
				entry.What = "ddns_update"
				entry.Why = "dynamic dns record updated"
			} else {
				entry.What = "ddns_update_failed"
				entry.Why = "dynamic dns update failed"
			}
		}

		// Output as JSON
		logJSON, _ := json.Marshal(entry)
		fmt.Println(string(logJSON))

		return err
	}
}
