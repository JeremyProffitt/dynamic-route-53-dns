package service

import (
	"context"
	"fmt"
	"net"
	"time"

	"dynamic-route-53-dns/internal/database"
	"dynamic-route-53-dns/internal/route53"
)

// UpdateService handles DDNS update requests
type UpdateService struct{}

// NewUpdateService creates a new update service
func NewUpdateService() *UpdateService {
	return &UpdateService{}
}

// UpdateResult represents the result of a DDNS update
type UpdateResult struct {
	Success  bool
	Code     string // DynDNS2 response code
	Message  string
	IP       string
}

// Response codes for DynDNS2 protocol
const (
	ResponseGood    = "good"
	ResponseNoChg   = "nochg"
	ResponseNoHost  = "nohost"
	ResponseBadAuth = "badauth"
	ResponseAbuse   = "abuse"
	ResponseBadIP   = "911"
)

// ValidateIP validates an IP address (IPv4 or IPv6)
func ValidateIP(ip string) bool {
	return net.ParseIP(ip) != nil
}

// ProcessUpdate processes a DDNS update request
func (s *UpdateService) ProcessUpdate(ctx context.Context, hostname, token, ip, sourceIP, userAgent string) *UpdateResult {
	// Validate IP format
	if !ValidateIP(ip) {
		return &UpdateResult{
			Success: false,
			Code:    ResponseBadIP,
			Message: "Invalid IP address format",
		}
	}

	// Get the DDNS record
	record, err := database.GetDDNSRecord(ctx, hostname)
	if err != nil || record == nil {
		return &UpdateResult{
			Success: false,
			Code:    ResponseNoHost,
			Message: "Hostname not found",
		}
	}

	// Verify the token
	if !VerifyToken(token, record.UpdateTokenHash) {
		return &UpdateResult{
			Success: false,
			Code:    ResponseBadAuth,
			Message: "Invalid credentials",
		}
	}

	// Check if record is enabled
	if !record.Enabled {
		return &UpdateResult{
			Success: false,
			Code:    ResponseNoHost,
			Message: "DDNS record is disabled",
		}
	}

	// Check rate limit (60 requests per hour)
	count, exceeded, err := database.IncrementRateLimit(ctx, fmt.Sprintf("ddns:%s", hostname), 60, 3600)
	if err != nil {
		return &UpdateResult{
			Success: false,
			Code:    ResponseBadIP,
			Message: "Internal error",
		}
	}
	if exceeded {
		return &UpdateResult{
			Success: false,
			Code:    ResponseAbuse,
			Message: fmt.Sprintf("Rate limit exceeded: %d requests in the last hour", count),
		}
	}

	// Check if IP has changed
	previousIP := record.CurrentIP
	if previousIP == ip {
		return &UpdateResult{
			Success: true,
			Code:    ResponseNoChg,
			Message: "IP unchanged",
			IP:      ip,
		}
	}

	// Update Route 53 record
	if err := route53.UpdateRecord(ctx, record.ZoneID, hostname, ip, record.TTL); err != nil {
		return &UpdateResult{
			Success: false,
			Code:    ResponseBadIP,
			Message: "Failed to update DNS record",
		}
	}

	// Update database record
	record.CurrentIP = ip
	if err := database.UpdateDDNSRecord(ctx, record); err != nil {
		// Log error but don't fail - Route 53 was already updated
		fmt.Printf("Warning: Failed to update database record: %v\n", err)
	}

	// Log the update
	log := &database.UpdateLog{
		PreviousIP: previousIP,
		NewIP:      ip,
		SourceIP:   sourceIP,
		UserAgent:  userAgent,
		Status:     "success",
		Timestamp:  time.Now().UTC(),
	}
	// Overwrite the PK to use hostname
	log.PK = fmt.Sprintf("LOG#%s", hostname)
	if err := database.CreateUpdateLog(ctx, log); err != nil {
		// Log error but don't fail
		fmt.Printf("Warning: Failed to create update log: %v\n", err)
	}

	return &UpdateResult{
		Success: true,
		Code:    ResponseGood,
		Message: "Update successful",
		IP:      ip,
	}
}
