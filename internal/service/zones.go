package service

import (
	"context"

	"dynamic-route-53-dns/internal/route53"
)

// ZoneService handles zone-related operations
type ZoneService struct{}

// NewZoneService creates a new zone service
func NewZoneService() *ZoneService {
	return &ZoneService{}
}

// ListZones returns all hosted zones
func (s *ZoneService) ListZones(ctx context.Context) ([]route53.Zone, error) {
	return route53.ListZones(ctx)
}

// GetZone returns a specific zone
func (s *ZoneService) GetZone(ctx context.Context, zoneID string) (*route53.Zone, error) {
	return route53.GetZone(ctx, zoneID)
}

// GetZoneRecords returns all records for a zone
func (s *ZoneService) GetZoneRecords(ctx context.Context, zoneID string) ([]route53.Record, error) {
	return route53.ListRecords(ctx, zoneID)
}
