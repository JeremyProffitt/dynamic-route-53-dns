package service

import (
	"context"
	"fmt"
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
	Hostname string
	ZoneID   string
	ZoneName string
	TTL      int64
}

// CreateDDNSResult represents the result of creating a DDNS record
type CreateDDNSResult struct {
	Success     bool
	Token       string
	Error       string
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

	// Validate zone exists
	zone, err := route53.GetZone(ctx, config.ZoneID)
	if err != nil || zone == nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Invalid zone ID",
		}
	}

	// Verify hostname is in the zone
	if !strings.HasSuffix(config.Hostname, zone.Name) {
		return &CreateDDNSResult{
			Success: false,
			Error:   fmt.Sprintf("Hostname must be in zone %s", zone.Name),
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

	// Create the record
	record := &database.DDNSRecord{
		Hostname:        config.Hostname,
		ZoneID:          config.ZoneID,
		ZoneName:        zone.Name,
		TTL:             ttl,
		UpdateTokenHash: tokenHash,
		Enabled:         true,
	}

	if err := database.CreateDDNSRecord(ctx, record); err != nil {
		return &CreateDDNSResult{
			Success: false,
			Error:   "Failed to create record",
		}
	}

	return &CreateDDNSResult{
		Success: true,
		Token:   token,
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

// GetUpdateHistory retrieves update history for a hostname
func (s *DDNSService) GetUpdateHistory(ctx context.Context, hostname string, limit int32) ([]database.UpdateLog, error) {
	return database.GetUpdateLogs(ctx, hostname, limit)
}
