package route53

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/route53"
)

// Zone represents a Route 53 hosted zone
type Zone struct {
	ID          string
	Name        string
	RecordCount int64
	IsPrivate   bool
	Comment     string
}

// ListZones returns all hosted zones
func ListZones(ctx context.Context) ([]Zone, error) {
	// Check cache first
	if cached := getCachedZones(); cached != nil {
		return cached, nil
	}

	var zones []Zone
	var marker *string

	for {
		input := &route53.ListHostedZonesInput{
			Marker: marker,
		}

		result, err := client.ListHostedZones(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list hosted zones: %w", err)
		}

		for _, hz := range result.HostedZones {
			zone := Zone{
				ID:          strings.TrimPrefix(*hz.Id, "/hostedzone/"),
				Name:        strings.TrimSuffix(*hz.Name, "."),
				RecordCount: *hz.ResourceRecordSetCount,
				IsPrivate:   hz.Config != nil && hz.Config.PrivateZone,
			}
			if hz.Config != nil && hz.Config.Comment != nil {
				zone.Comment = *hz.Config.Comment
			}
			zones = append(zones, zone)
		}

		if !result.IsTruncated {
			break
		}
		marker = result.NextMarker
	}

	// Update cache
	setCachedZones(zones)

	return zones, nil
}

// GetZone returns a specific hosted zone by ID
func GetZone(ctx context.Context, zoneID string) (*Zone, error) {
	// Check cache first
	if cached := getCachedZones(); cached != nil {
		for _, z := range cached {
			if z.ID == zoneID {
				return &z, nil
			}
		}
	}

	result, err := client.GetHostedZone(ctx, &route53.GetHostedZoneInput{
		Id: &zoneID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get hosted zone: %w", err)
	}

	zone := &Zone{
		ID:          strings.TrimPrefix(*result.HostedZone.Id, "/hostedzone/"),
		Name:        strings.TrimSuffix(*result.HostedZone.Name, "."),
		RecordCount: *result.HostedZone.ResourceRecordSetCount,
		IsPrivate:   result.HostedZone.Config != nil && result.HostedZone.Config.PrivateZone,
	}
	if result.HostedZone.Config != nil && result.HostedZone.Config.Comment != nil {
		zone.Comment = *result.HostedZone.Config.Comment
	}

	return zone, nil
}
