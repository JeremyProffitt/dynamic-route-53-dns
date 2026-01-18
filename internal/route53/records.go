package route53

import (
	"context"
	"fmt"
	"net"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// Record represents a DNS record
type Record struct {
	Name   string
	Type   string
	TTL    int64
	Values []string
}

// ListRecords returns all records for a zone
func ListRecords(ctx context.Context, zoneID string) ([]Record, error) {
	var records []Record
	var startName *string
	var startType types.RRType

	for {
		input := &route53.ListResourceRecordSetsInput{
			HostedZoneId:    aws.String(zoneID),
			StartRecordName: startName,
		}
		if startType != "" {
			input.StartRecordType = startType
		}

		result, err := client.ListResourceRecordSets(ctx, input)
		if err != nil {
			return nil, fmt.Errorf("failed to list records: %w", err)
		}

		for _, rrs := range result.ResourceRecordSets {
			record := Record{
				Name: strings.TrimSuffix(*rrs.Name, "."),
				Type: string(rrs.Type),
			}
			if rrs.TTL != nil {
				record.TTL = *rrs.TTL
			}

			// Handle alias records
			if rrs.AliasTarget != nil {
				record.Values = []string{fmt.Sprintf("ALIAS: %s", *rrs.AliasTarget.DNSName)}
			} else {
				for _, rr := range rrs.ResourceRecords {
					record.Values = append(record.Values, *rr.Value)
				}
			}
			records = append(records, record)
		}

		if !result.IsTruncated {
			break
		}
		startName = result.NextRecordName
		startType = result.NextRecordType
	}

	return records, nil
}

// UpdateRecord creates or updates a DNS record
func UpdateRecord(ctx context.Context, zoneID, hostname, ip string, ttl int64) error {
	// Determine record type based on IP version
	recordType := types.RRTypeA
	if net.ParseIP(ip).To4() == nil {
		recordType = types.RRTypeAaaa
	}

	// Ensure hostname ends with a dot
	fqdn := hostname
	if !strings.HasSuffix(fqdn, ".") {
		fqdn = fqdn + "."
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Comment: aws.String("DDNS update"),
			Changes: []types.Change{
				{
					Action: types.ChangeActionUpsert,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(fqdn),
						Type: recordType,
						TTL:  aws.Int64(ttl),
						ResourceRecords: []types.ResourceRecord{
							{
								Value: aws.String(ip),
							},
						},
					},
				},
			},
		},
	}

	_, err := client.ChangeResourceRecordSets(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	return nil
}

// DeleteRecord deletes a DNS record
func DeleteRecord(ctx context.Context, zoneID, hostname, ip string, ttl int64) error {
	// Determine record type based on IP version
	recordType := types.RRTypeA
	if net.ParseIP(ip).To4() == nil {
		recordType = types.RRTypeAaaa
	}

	// Ensure hostname ends with a dot
	fqdn := hostname
	if !strings.HasSuffix(fqdn, ".") {
		fqdn = fqdn + "."
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Comment: aws.String("DDNS record deletion"),
			Changes: []types.Change{
				{
					Action: types.ChangeActionDelete,
					ResourceRecordSet: &types.ResourceRecordSet{
						Name: aws.String(fqdn),
						Type: recordType,
						TTL:  aws.Int64(ttl),
						ResourceRecords: []types.ResourceRecord{
							{
								Value: aws.String(ip),
							},
						},
					},
				},
			},
		},
	}

	_, err := client.ChangeResourceRecordSets(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	return nil
}

// GetRecord retrieves a specific DNS record
func GetRecord(ctx context.Context, zoneID, hostname string, recordType types.RRType) (*Record, error) {
	fqdn := hostname
	if !strings.HasSuffix(fqdn, ".") {
		fqdn = fqdn + "."
	}

	input := &route53.ListResourceRecordSetsInput{
		HostedZoneId:    aws.String(zoneID),
		StartRecordName: aws.String(fqdn),
		StartRecordType: recordType,
		MaxItems:        aws.Int32(1),
	}

	result, err := client.ListResourceRecordSets(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to get record: %w", err)
	}

	for _, rrs := range result.ResourceRecordSets {
		name := strings.TrimSuffix(*rrs.Name, ".")
		if name == hostname && rrs.Type == recordType {
			record := &Record{
				Name: name,
				Type: string(rrs.Type),
			}
			if rrs.TTL != nil {
				record.TTL = *rrs.TTL
			}
			for _, rr := range rrs.ResourceRecords {
				record.Values = append(record.Values, *rr.Value)
			}
			return record, nil
		}
	}

	return nil, nil
}
