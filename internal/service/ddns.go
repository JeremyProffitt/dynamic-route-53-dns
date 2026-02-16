package service

import (
	"context"
	"fmt"
	"net"
	"regexp"
	"strings"

	"dynamic-route-53-dns/internal/auth"
	"dynamic-route-53-dns/internal/database"
	"dynamic-route-53-dns/internal/route53"
)

// DDNSService handles DDNS record management
type DDNSService struct{}

// NewDDNSService creates a new DDNS service
func NewDDNSService() *DDNSService {
	return &DDNSService{}
}

// DDNSConfig represents configuration for creating a DDNS record
type DDNSConfig struct {
	Hostname  string
	ZoneID    string
	ZoneName  string
	TTL       int64
	InitialIP string
}

// CreateDDNSResult represents the result of creating a DDNS record
type CreateDDNSResult struct {
	Success  bool
	Token    string
	Hostname string
	Error    string
}

// hostnameRegex validates RFC 1123 hostnames
var hostnameRegex = regexp.MustCompile(`^[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?(\.[a-zA-Z0-9]([a-zA-Z0-9\-]{0,61}[a-zA-Z0-9])?)*$`)

// ValidateHostname validates a hostname against RFC 1123
func ValidateHostname(hostname string) bool {
	if len(hostname) > 253 {
		return false
	}
	return hostnameRegex.MatchString(hostname)
}

// CreateDDNSRecord creates a new DDNS record
func (s *DDNSService) CreateDDNSRecord(ctx context.Context, config *DDNSConfig) *CreateDDNSResult {
	// Validate zone exists first (needed for auto-suffix)
	zone, err := route53.GetZone(ctx, config.ZoneID)
	if err != nil || zone == nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Invalid zone ID",
		}
	}

	// Auto-append zone suffix if hostname doesn't already include it
	if !strings.HasSuffix(config.Hostname, "."+zone.Name) && config.Hostname != zone.Name {
		config.Hostname = config.Hostname + "." + zone.Name
	}

	// Validate hostname
	if !ValidateHostname(config.Hostname) {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Invalid hostname format",
		}
	}

	// Check if record already exists
	existing, err := database.GetDDNSRecord(ctx, config.Hostname)
	if err != nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Failed to check existing record",
		}
	}
	if existing != nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "DDNS record already exists for this hostname",
		}
	}

	// Generate update token
	token, err := auth.GenerateUpdateToken()
	if err != nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Failed to generate token",
		}
	}

	// Hash the token
	tokenHash, err := HashToken(token)
	if err != nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Failed to hash token",
		}
	}

	// Set default TTL
	ttl := config.TTL
	if ttl <= 0 {
		ttl = 60
	}

	// Validate initial IP if provided
	if config.InitialIP != "" {
		if net.ParseIP(config.InitialIP) == nil {
			return &CreateDDNSResult{
				Success: false,
				Error:   "Invalid IP address format",
			}
		}
	}

	// Create the record
	record := &database.DDNSRecord{
		Hostname:        config.Hostname,
		ZoneID:          config.ZoneID,
		ZoneName:        zone.Name,
		TTL:             ttl,
		UpdateTokenHash: tokenHash,
		CurrentIP:       config.InitialIP,
		Enabled:         true,
	}

	if err := database.CreateDDNSRecord(ctx, record); err != nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Failed to create record",
		}
	}

	// If initial IP was provided, create the Route 53 record
	if config.InitialIP != "" {
		if err := route53.UpdateRecord(ctx, config.ZoneID, config.Hostname, config.InitialIP, ttl); err != nil {
			// Record was created in DB but Route 53 failed - not fatal
			fmt.Printf("Warning: Failed to create initial Route 53 record: %v\n", err)
		}
	}

	return &CreateDDNSResult{
		Success:  true,
		Token:    token,
		Hostname: config.Hostname,
	}
}

// GetDDNSRecord retrieves a DDNS record
func (s *DDNSService) GetDDNSRecord(ctx context.Context, hostname string) (*database.DDNSRecord, error) {
	return database.GetDDNSRecord(ctx, hostname)
}

// ListDDNSRecords lists all DDNS records
func (s *DDNSService) ListDDNSRecords(ctx context.Context) ([]database.DDNSRecord, error) {
	return database.ListDDNSRecords(ctx)
}

// UpdateDDNSRecord updates a DDNS record
func (s *DDNSService) UpdateDDNSRecord(ctx context.Context, hostname string, enabled bool, ttl int64) error {
	record, err := database.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("record not found")
	}

	record.Enabled = enabled
	if ttl > 0 {
		record.TTL = ttl
	}

	return database.UpdateDDNSRecord(ctx, record)
}

// DeleteDDNSRecord deletes a DDNS record and its Route 53 record
func (s *DDNSService) DeleteDDNSRecord(ctx context.Context, hostname string) error {
	record, err := database.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("record not found")
	}

	// Delete Route 53 record if IP exists
	if record.CurrentIP != "" {
		_ = route53.DeleteRecord(ctx, record.ZoneID, hostname, record.CurrentIP, record.TTL)
	}

	return database.DeleteDDNSRecord(ctx, hostname)
}

// RegenerateToken generates a new token for a DDNS record
func (s *DDNSService) RegenerateToken(ctx context.Context, hostname string) (string, error) {
	record, err := database.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return "", err
	}
	if record == nil {
		return "", fmt.Errorf("record not found")
	}

	// Generate new token
	token, err := auth.GenerateUpdateToken()
	if err != nil {
		return "", err
	}

	// Hash and store
	tokenHash, err := HashToken(token)
	if err != nil {
		return "", err
	}

	record.UpdateTokenHash = tokenHash
	if err := database.UpdateDDNSRecord(ctx, record); err != nil {
		return "", err
	}

	return token, nil
}

// ManualUpdateIP manually updates the IP address for a DDNS record
func (s *DDNSService) ManualUpdateIP(ctx context.Context, hostname, ip string) error {
	if net.ParseIP(ip) == nil {
		return fmt.Errorf("invalid IP address format")
	}

	record, err := database.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return err
	}
	if record == nil {
		return fmt.Errorf("record not found")
	}

	// Update Route 53 record
	if err := route53.UpdateRecord(ctx, record.ZoneID, hostname, ip, record.TTL); err != nil {
		return fmt.Errorf("failed to update DNS record: %w", err)
	}

	// Update database record
	record.CurrentIP = ip
	if err := database.UpdateDDNSRecord(ctx, record); err != nil {
		return fmt.Errorf("failed to update database record: %w", err)
	}

	return nil
}

// GetUpdateHistory retrieves update history for a hostname
func (s *DDNSService) GetUpdateHistory(ctx context.Context, hostname string, limit int32) ([]database.UpdateLog, error) {
	return database.GetUpdateLogs(ctx, hostname, limit)
}
