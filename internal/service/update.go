package service

import (
	"context"
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/dynamic-route-53-dns/internal/database"
	"github.com/dynamic-route-53-dns/internal/route53"
)

// DynDNS2 response codes
const (
	ResponseGood    = "good"    // Update successful
	ResponseNoChg   = "nochg"   // IP unchanged
	ResponseNoHost  = "nohost"  // Hostname not found
	ResponseBadAuth = "badauth" // Invalid credentials
	ResponseAbuse   = "abuse"   // Rate limited
)

var (
	// ErrInvalidIP is returned when the provided IP address is invalid.
	ErrInvalidIP = errors.New("invalid IP address")
	// ErrIPMismatch is returned when providedIP doesn't match sourceIP (anti-spoofing).
	ErrIPMismatch = errors.New("provided IP does not match source IP")
	// ErrDisabled is returned when the DDNS record is disabled.
	ErrDisabled = errors.New("ddns record is disabled")
	// ErrRateLimited is returned when too many requests have been made.
	ErrRateLimited = errors.New("rate limited")
)

// UpdateService handles DDNS update operations.
type UpdateService struct {
	db            *database.Client
	route53Client *route53.Route53Client
}

// NewUpdateService creates a new UpdateService with the provided database and Route 53 clients.
func NewUpdateService(db *database.Client, r53 *route53.Route53Client) *UpdateService {
	return &UpdateService{
		db:            db,
		route53Client: r53,
	}
}

// ProcessUpdate processes a DDNS update request and returns a DynDNS2 response code.
// Parameters:
//   - hostname: The FQDN to update
//   - providedIP: The IP address provided in the request (may be empty)
//   - sourceIP: The source IP address of the request
//   - token: The authentication token
//   - userAgent: The user agent string for logging
//
// Returns a DynDNS2-compatible response string.
func (s *UpdateService) ProcessUpdate(ctx context.Context, hostname, providedIP, sourceIP, token, userAgent string) (string, error) {
	// Get the DDNS record
	record, err := s.db.GetDDNSRecord(ctx, hostname)
	if err != nil {
		if errors.Is(err, database.ErrRecordNotFound) {
			return ResponseNoHost, ErrRecordNotFound
		}
		return ResponseNoHost, fmt.Errorf("failed to get record: %w", err)
	}

	// Check if record is enabled
	if !record.Enabled {
		return ResponseNoHost, ErrDisabled
	}

	// Validate token against stored hash (bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(record.UpdateTokenHash), []byte(token)); err != nil {
		s.logUpdate(ctx, hostname, record.CurrentIP, "", sourceIP, userAgent, "badauth")
		return ResponseBadAuth, errors.New("invalid token")
	}

	// Determine the IP to use
	ipToUse := sourceIP
	if providedIP != "" {
		ipToUse = providedIP
	}

	// Validate IP format
	isValid, isIPv6 := isValidIP(ipToUse)
	if !isValid {
		s.logUpdate(ctx, hostname, record.CurrentIP, ipToUse, sourceIP, userAgent, "invalid_ip")
		return ResponseNoHost, ErrInvalidIP
	}

	// IP spoofing protection: if providedIP is set, it must match sourceIP
	if providedIP != "" && providedIP != sourceIP {
		s.logUpdate(ctx, hostname, record.CurrentIP, providedIP, sourceIP, userAgent, "ip_mismatch")
		return ResponseAbuse, ErrIPMismatch
	}

	// Check if IP changed
	if record.CurrentIP == ipToUse {
		s.logUpdate(ctx, hostname, record.CurrentIP, ipToUse, sourceIP, userAgent, "nochg")
		return fmt.Sprintf("%s %s", ResponseNoChg, ipToUse), nil
	}

	// IP changed - update Route 53 record
	recordType := "A"
	if isIPv6 {
		recordType = "AAAA"
	}

	// Delete old record if exists with different type or value
	if record.CurrentIP != "" {
		_, oldIsIPv6 := isValidIP(record.CurrentIP)
		oldRecordType := "A"
		if oldIsIPv6 {
			oldRecordType = "AAAA"
		}

		// If the record type changed (IPv4 <-> IPv6), delete the old record
		if oldRecordType != recordType {
			_ = s.route53Client.DeleteRecord(ctx, record.ZoneID, hostname, oldRecordType)
		}
	}

	// Create/update the DNS record
	err = s.route53Client.UpsertRecord(ctx, record.ZoneID, hostname, recordType, ipToUse, int64(record.TTL))
	if err != nil {
		s.logUpdate(ctx, hostname, record.CurrentIP, ipToUse, sourceIP, userAgent, "route53_error")
		return ResponseNoHost, fmt.Errorf("failed to update Route 53: %w", err)
	}

	// Update record in database
	previousIP := record.CurrentIP
	record.CurrentIP = ipToUse
	record.LastUpdated = time.Now()

	if err := s.db.UpdateDDNSRecord(ctx, record); err != nil {
		s.logUpdate(ctx, hostname, previousIP, ipToUse, sourceIP, userAgent, "db_error")
		return ResponseNoHost, fmt.Errorf("failed to update database: %w", err)
	}

	// Log successful update
	s.logUpdate(ctx, hostname, previousIP, ipToUse, sourceIP, userAgent, "good")

	return fmt.Sprintf("%s %s", ResponseGood, ipToUse), nil
}

// logUpdate logs a DDNS update attempt to the database.
func (s *UpdateService) logUpdate(ctx context.Context, hostname, previousIP, newIP, sourceIP, userAgent, status string) {
	now := time.Now()
	log := database.NewUpdateLog(hostname, now)
	log.PreviousIP = previousIP
	log.NewIP = newIP
	log.SourceIP = sourceIP
	log.UserAgent = userAgent
	log.Status = status

	// Best effort logging - don't fail the update if logging fails
	_ = s.db.CreateUpdateLog(ctx, log)
}

// isValidIP validates an IP address and returns whether it's valid and whether it's IPv6.
func isValidIP(ip string) (isValid bool, isIPv6 bool) {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false, false
	}

	// Check if it's IPv6
	// If To4() returns nil, it's an IPv6 address
	if parsed.To4() == nil {
		return true, true
	}

	return true, false
}
