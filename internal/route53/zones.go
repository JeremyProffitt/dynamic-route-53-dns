package route53

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
)

const (
	hostedZonesCacheKey = "hosted_zones"
)

// HostedZone represents a simplified view of a Route53 hosted zone
type HostedZone struct {
	ID          string
	Name        string
	RecordCount int64
	IsPrivate   bool
}

// ListHostedZones retrieves all hosted zones with caching
func (r *Route53Client) ListHostedZones(ctx context.Context) ([]HostedZone, error) {
	// Check cache first
	if cached, ok := r.getFromCache(hostedZonesCacheKey); ok {
		if zones, ok := cached.([]HostedZone); ok {
			return zones, nil
		}
	}

	var zones []HostedZone
	var marker *string

	for {
		input := &route53.ListHostedZonesInput{
			Marker: marker,
		}

		output, err := r.client.ListHostedZones(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, zone := range output.HostedZones {
			hz := HostedZone{
				ID:          extractZoneID(*zone.Id),
				Name:        strings.TrimSuffix(*zone.Name, "."),
				RecordCount: *zone.ResourceRecordSetCount,
				IsPrivate:   zone.Config != nil && zone.Config.PrivateZone,
			}
			zones = append(zones, hz)
		}

		if !output.IsTruncated {
			break
		}
		marker = output.NextMarker
	}

	// Cache the results
	r.setInCache(hostedZonesCacheKey, zones)

	return zones, nil
}

// GetHostedZone retrieves a specific hosted zone by ID
func (r *Route53Client) GetHostedZone(ctx context.Context, zoneID string) (*HostedZone, error) {
	// Check if we have it in the cached list
	if cached, ok := r.getFromCache(hostedZonesCacheKey); ok {
		if zones, ok := cached.([]HostedZone); ok {
			for _, zone := range zones {
				if zone.ID == zoneID {
					return &zone, nil
				}
			}
		}
	}

	// Fetch directly from AWS
	input := &route53.GetHostedZoneInput{
		Id: &zoneID,
	}

	output, err := r.client.GetHostedZone(ctx, input)
	if err != nil {
		return nil, err
	}

	zone := &HostedZone{
		ID:          extractZoneID(*output.HostedZone.Id),
		Name:        strings.TrimSuffix(*output.HostedZone.Name, "."),
		RecordCount: *output.HostedZone.ResourceRecordSetCount,
		IsPrivate:   output.HostedZone.Config != nil && output.HostedZone.Config.PrivateZone,
	}

	return zone, nil
}

// extractZoneID removes the "/hostedzone/" prefix from the zone ID
func extractZoneID(id string) string {
	return strings.TrimPrefix(id, "/hostedzone/")
}
