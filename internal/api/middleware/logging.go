package middleware

import (
	"encoding/json"
	"log"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

// LogEntry represents a structured log entry with mandatory fields
type LogEntry struct {
	Who        string        `json:"who"`         // type:identifier (e.g., user:john@example.com or ip:192.168.1.1)
	What       string        `json:"what"`        // Past tense action (e.g., updated_ddns_record)
	Why        string        `json:"why"`         // Business reason (max 100 chars)
	Where      string        `json:"where"`       // service:component (e.g., ddns:update_handler)
	Timestamp  time.Time     `json:"timestamp"`   // When the event occurred
	RequestID  string        `json:"request_id"`  // Unique request identifier
	Method     string        `json:"method"`      // HTTP method
	Path       string        `json:"path"`        // Request path
	StatusCode int           `json:"status_code"` // HTTP response status code
	Duration   time.Duration `json:"duration"`    // Request duration
}

// RequestLogger returns a Fiber handler that logs request start and completion
// with structured JSON logging including user context if authenticated
func RequestLogger() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Generate unique request ID
		requestID := uuid.New().String()
		c.Locals("request_id", requestID)

		// Record start time
		startTime := time.Now()

		// Determine "who" - check for authenticated user first
		who := determineWho(c)

		// Log request start
		startEntry := LogEntry{
			Who:       who,
			What:      "request_started",
			Why:       "incoming HTTP request",
			Where:     "api:request_logger",
			Timestamp: startTime,
			RequestID: requestID,
			Method:    c.Method(),
			Path:      c.Path(),
		}
		logJSON(startEntry)

		// Process request
		err := c.Next()

		// Calculate duration
		duration := time.Since(startTime)

		// Update "who" in case authentication happened during request
		who = determineWho(c)

		// Log request completion
		completionEntry := LogEntry{
			Who:        who,
			What:       "request_completed",
			Why:        "HTTP request finished",
			Where:      "api:request_logger",
			Timestamp:  time.Now(),
			RequestID:  requestID,
			Method:     c.Method(),
			Path:       c.Path(),
			StatusCode: c.Response().StatusCode(),
			Duration:   duration,
		}
		logJSON(completionEntry)

		return err
	}
}

// determineWho extracts the identity for logging from the request context
func determineWho(c *fiber.Ctx) string {
	// Check for authenticated username in locals
	if username, ok := c.Locals("username").(string); ok && username != "" {
		return "user:" + username
	}

	// Fall back to IP address
	ip := c.IP()
	if ip == "" {
		ip = "unknown"
	}
	return "ip:" + ip
}

// LogAction logs a specific business action with structured format
// This is a helper for logging specific business events outside of request context
func LogAction(who, what, why, where string) {
	// Truncate "why" to max 100 characters as per logging standards
	if len(why) > 100 {
		why = why[:97] + "..."
	}

	entry := LogEntry{
		Who:       who,
		What:      what,
		Why:       why,
		Where:     where,
		Timestamp: time.Now(),
	}
	logJSON(entry)
}

// LogActionWithContext logs a business action with request context
func LogActionWithContext(c *fiber.Ctx, what, why, where string) {
	who := determineWho(c)
	requestID, _ := c.Locals("request_id").(string)

	// Truncate "why" to max 100 characters as per logging standards
	if len(why) > 100 {
		why = why[:97] + "..."
	}

	entry := LogEntry{
		Who:       who,
		What:      what,
		Why:       why,
		Where:     where,
		Timestamp: time.Now(),
		RequestID: requestID,
		Method:    c.Method(),
		Path:      c.Path(),
	}
	logJSON(entry)
}

// logJSON outputs a log entry as JSON to standard output
func logJSON(entry LogEntry) {
	data, err := json.Marshal(entry)
	if err != nil {
		log.Printf("error marshaling log entry: %v", err)
		return
	}
	log.Println(string(data))
}
