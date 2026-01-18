package service

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"

	"github.com/dynamic-route-53-dns/internal/database"
	"github.com/dynamic-route-53-dns/internal/route53"
)

const (
	// TokenLength is the number of random bytes used for token generation.
	TokenLength = 32
	// BcryptCost is the cost factor for bcrypt hashing.
	BcryptCost = 10
)

var (
	// ErrRecordNotFound is returned when a DDNS record does not exist.
	ErrRecordNotFound = errors.New("ddns record not found")
	// ErrRecordExists is returned when attempting to create a duplicate record.
	ErrRecordExists = errors.New("ddns record already exists")
	// ErrTokenGeneration is returned when token generation fails.
	ErrTokenGeneration = errors.New("failed to generate token")
)

// DDNSService handles DDNS record management operations.
type DDNSService struct {
	db            *database.Client
	route53Client *route53.Route53Client
}

// NewDDNSService creates a new DDNSService with the provided database and Route 53 clients.
func NewDDNSService(db *database.Client, r53 *route53.Route53Client) *DDNSService {
	return &DDNSService{
		db:            db,
		route53Client: r53,
	}
}

// ListDDNSRecords returns all configured DDNS records.
func (s *DDNSService) ListDDNSRecords(ctx context.Context) ([]database.DDNSRecord, error) {
	return s.db.ListDDNSRecords(ctx)
}

// GetDDNSRecord retrieves a specific DDNS record by hostname.
func (s *DDNSService) GetDDNSRecord(ctx context.Context, hostname string) (*database.DDNSRecord, error) {
	record, err := s.db.GetDDNSRecord(ctx, hostname)
	if err != nil {
		if errors.Is(err, database.ErrRecordNotFound) {
			return nil, ErrRecordNotFound
		}
		return nil, err
	}
	return record, nil
}

// CreateDDNSRecord creates a new DDNS record and returns the plaintext token.
// The plaintext token is only returned once at creation time (like API key UX).
func (s *DDNSService) CreateDDNSRecord(ctx context.Context, hostname, zoneID, zoneName string, ttl int) (string, error) {
	// Check if record already exists
	existing, err := s.db.GetDDNSRecord(ctx, hostname)
	if err != nil && !errors.Is(err, database.ErrRecordNotFound) {
		return "", fmt.Errorf("failed to check existing record: %w", err)
	}
	if existing != nil {
		return "", ErrRecordExists
	}

	// Generate random token (32 bytes, base64 encoded)
	plainTextToken, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGeneration, err)
	}

	// Hash token with bcrypt (cost 10)
	hashedToken, err := bcrypt.GenerateFromPassword([]byte(plainTextToken), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash token: %w", err)
	}

	// Create record with hashed token
	now := time.Now()
	record := &database.DDNSRecord{
		Hostname:        hostname,
		ZoneID:          zoneID,
		ZoneName:        zoneName,
		TTL:             ttl,
		UpdateTokenHash: string(hashedToken),
		CurrentIP:       "",
		Enabled:         true,
		LastUpdated:     now,
		CreatedAt:       now,
	}

	if err := s.db.CreateDDNSRecord(ctx, record); err != nil {
		return "", fmt.Errorf("failed to create record: %w", err)
	}

	// Return plaintext token ONCE
	return plainTextToken, nil
}

// UpdateDDNSRecord updates an existing DDNS record's configuration.
func (s *DDNSService) UpdateDDNSRecord(ctx context.Context, hostname string, ttl int, enabled bool) error {
	record, err := s.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return err
	}

	record.TTL = ttl
	record.Enabled = enabled

	return s.db.UpdateDDNSRecord(ctx, record)
}

// DeleteDDNSRecord deletes a DDNS record and its corresponding DNS record from Route 53.
func (s *DDNSService) DeleteDDNSRecord(ctx context.Context, hostname string) error {
	record, err := s.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return err
	}

	// Delete the DNS record from Route 53 if an IP is currently set
	if record.CurrentIP != "" {
		_, isIPv6 := isValidIP(record.CurrentIP)
		recordType := "A"
		if isIPv6 {
			recordType = "AAAA"
		}

		err := s.route53Client.DeleteRecord(ctx, record.ZoneID, hostname, recordType)
		if err != nil {
			return fmt.Errorf("failed to delete DNS record from Route 53: %w", err)
		}
	}

	// Delete the record from the database
	return s.db.DeleteDDNSRecord(ctx, hostname)
}

// RegenerateToken generates a new token for an existing DDNS record.
// Returns the new plaintext token (only shown once).
func (s *DDNSService) RegenerateToken(ctx context.Context, hostname string) (string, error) {
	record, err := s.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return "", err
	}

	// Generate new random token
	plainTextToken, err := generateToken()
	if err != nil {
		return "", fmt.Errorf("%w: %v", ErrTokenGeneration, err)
	}

	// Hash new token with bcrypt
	hashedToken, err := bcrypt.GenerateFromPassword([]byte(plainTextToken), BcryptCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash token: %w", err)
	}

	// Update record with new hashed token
	record.UpdateTokenHash = string(hashedToken)

	if err := s.db.UpdateDDNSRecord(ctx, record); err != nil {
		return "", fmt.Errorf("failed to update record: %w", err)
	}

	return plainTextToken, nil
}

// GetUpdateHistory retrieves the update history for a hostname.
func (s *DDNSService) GetUpdateHistory(ctx context.Context, hostname string, limit int) ([]database.UpdateLog, error) {
	// First verify the record exists
	_, err := s.GetDDNSRecord(ctx, hostname)
	if err != nil {
		return nil, err
	}

	return s.db.GetUpdateLogs(ctx, hostname, limit)
}

// generateToken generates a cryptographically secure random token.
// Returns a base64-encoded string of 32 random bytes.
func generateToken() (string, error) {
	bytes := make([]byte, TokenLength)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}
