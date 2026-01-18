package service

import (
	"context"

	"github.com/dynamic-route-53-dns/internal/route53"
)

// ZoneService handles DNS zone operations via Route 53.
type ZoneService struct {
	route53Client *route53.Route53Client
}

// NewZoneService creates a new ZoneService with the provided Route 53 client.
func NewZoneService(r53 *route53.Route53Client) *ZoneService {
	return &ZoneService{
		route53Client: r53,
	}
}

// ListZones returns all hosted zones accessible via the Route 53 client.
func (s *ZoneService) ListZones(ctx context.Context) ([]route53.HostedZone, error) {
	return s.route53Client.ListHostedZones(ctx)
}

// GetZone retrieves a specific hosted zone by its ID.
func (s *ZoneService) GetZone(ctx context.Context, zoneID string) (*route53.HostedZone, error) {
	return s.route53Client.GetHostedZone(ctx, zoneID)
}

// GetZoneRecords retrieves all DNS records for a specific zone.
func (s *ZoneService) GetZoneRecords(ctx context.Context, zoneID string) ([]route53.DNSRecord, error) {
	return s.route53Client.ListRecords(ctx, zoneID)
}
