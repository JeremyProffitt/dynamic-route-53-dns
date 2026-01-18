package route53

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/route53"
	"github.com/aws/aws-sdk-go-v2/service/route53/types"
)

// DNSRecord represents a DNS record in a hosted zone
type DNSRecord struct {
	Name   string
	Type   string
	TTL    int64
	Values []string
}

// ListRecords retrieves all DNS records for a hosted zone
func (r *Route53Client) ListRecords(ctx context.Context, zoneID string) ([]DNSRecord, error) {
	var records []DNSRecord
	var startRecordName *string
	var startRecordType types.RRType

	for {
		input := &route53.ListResourceRecordSetsInput{
			HostedZoneId:    aws.String(zoneID),
			StartRecordName: startRecordName,
		}

		if startRecordName != nil {
			input.StartRecordType = startRecordType
		}

		output, err := r.client.ListResourceRecordSets(ctx, input)
		if err != nil {
			return nil, err
		}

		for _, rrs := range output.ResourceRecordSets {
			record := DNSRecord{
				Name: strings.TrimSuffix(*rrs.Name, "."),
				Type: string(rrs.Type),
			}

			if rrs.TTL != nil {
				record.TTL = *rrs.TTL
			}

			// Extract values from resource records
			for _, rr := range rrs.ResourceRecords {
				if rr.Value != nil {
					record.Values = append(record.Values, *rr.Value)
				}
			}

			// Handle alias records
			if rrs.AliasTarget != nil {
				record.Values = append(record.Values, fmt.Sprintf("ALIAS %s", *rrs.AliasTarget.DNSName))
			}

			records = append(records, record)
		}

		if !output.IsTruncated {
			break
		}

		startRecordName = output.NextRecordName
		startRecordType = output.NextRecordType
	}

	return records, nil
}

// UpsertRecord creates or updates a DNS record in the specified hosted zone
// Supports A (IPv4) and AAAA (IPv6) record types
func (r *Route53Client) UpsertRecord(ctx context.Context, zoneID, name, recordType, value string, ttl int64) error {
	// Validate record type
	rrType := types.RRType(recordType)
	if rrType != types.RRTypeA && rrType != types.RRTypeAaaa {
		return fmt.Errorf("unsupported record type: %s (only A and AAAA are supported)", recordType)
	}

	// Ensure the name has a trailing dot for Route53
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	change := types.Change{
		Action: types.ChangeActionUpsert,
		ResourceRecordSet: &types.ResourceRecordSet{
			Name: aws.String(name),
			Type: rrType,
			TTL:  aws.Int64(ttl),
			ResourceRecords: []types.ResourceRecord{
				{
					Value: aws.String(value),
				},
			},
		},
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{change},
			Comment: aws.String("Upserted by dynamic-route-53-dns"),
		},
	}

	_, err := r.client.ChangeResourceRecordSets(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to upsert record: %w", err)
	}

	return nil
}

// DeleteRecord removes a DNS record from the specified hosted zone
func (r *Route53Client) DeleteRecord(ctx context.Context, zoneID, name, recordType string) error {
	// First, we need to get the current record to know its TTL and values
	records, err := r.ListRecords(ctx, zoneID)
	if err != nil {
		return fmt.Errorf("failed to list records: %w", err)
	}

	// Find the matching record
	var targetRecord *DNSRecord
	for _, record := range records {
		if record.Name == strings.TrimSuffix(name, ".") && record.Type == recordType {
			targetRecord = &record
			break
		}
	}

	if targetRecord == nil {
		return fmt.Errorf("record not found: %s %s", name, recordType)
	}

	// Ensure the name has a trailing dot for Route53
	if !strings.HasSuffix(name, ".") {
		name = name + "."
	}

	// Build resource records from values
	var resourceRecords []types.ResourceRecord
	for _, value := range targetRecord.Values {
		// Skip alias records as they need special handling
		if strings.HasPrefix(value, "ALIAS ") {
			return fmt.Errorf("cannot delete alias records with this method")
		}
		resourceRecords = append(resourceRecords, types.ResourceRecord{
			Value: aws.String(value),
		})
	}

	change := types.Change{
		Action: types.ChangeActionDelete,
		ResourceRecordSet: &types.ResourceRecordSet{
			Name:            aws.String(name),
			Type:            types.RRType(recordType),
			TTL:             aws.Int64(targetRecord.TTL),
			ResourceRecords: resourceRecords,
		},
	}

	input := &route53.ChangeResourceRecordSetsInput{
		HostedZoneId: aws.String(zoneID),
		ChangeBatch: &types.ChangeBatch{
			Changes: []types.Change{change},
			Comment: aws.String("Deleted by dynamic-route-53-dns"),
		},
	}

	_, err = r.client.ChangeResourceRecordSets(ctx, input)
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	return nil
}
