package database

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// DDNSRecord represents a DDNS record in the database
type DDNSRecord struct {
	PK              string    `dynamodbav:"PK"`
	SK              string    `dynamodbav:"SK"`
	Hostname        string    `dynamodbav:"hostname"`
	ZoneID          string    `dynamodbav:"zone_id"`
	ZoneName        string    `dynamodbav:"zone_name"`
	TTL             int64     `dynamodbav:"ttl"`
	UpdateTokenHash string    `dynamodbav:"update_token_hash"`
	CurrentIP       string    `dynamodbav:"current_ip"`
	Enabled         bool      `dynamodbav:"enabled"`
	LastUpdated     time.Time `dynamodbav:"last_updated"`
	CreatedAt       time.Time `dynamodbav:"created_at"`
}

// UpdateLog represents an update log entry
type UpdateLog struct {
	PK         string    `dynamodbav:"PK"`
	SK         string    `dynamodbav:"SK"`
	PreviousIP string    `dynamodbav:"previous_ip"`
	NewIP      string    `dynamodbav:"new_ip"`
	SourceIP   string    `dynamodbav:"source_ip"`
	UserAgent  string    `dynamodbav:"user_agent"`
	Status     string    `dynamodbav:"status"`
	TTL        int64     `dynamodbav:"ttl"`
	Timestamp  time.Time `dynamodbav:"timestamp"`
}

// CreateDDNSRecord creates a new DDNS record
func CreateDDNSRecord(ctx context.Context, record *DDNSRecord) error {
	record.PK = "DDNS"
	record.SK = record.Hostname
	record.CreatedAt = time.Now().UTC()
	record.LastUpdated = record.CreatedAt

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		return fmt.Errorf("failed to create record: %w", err)
	}

	return nil
}

// GetDDNSRecord retrieves a DDNS record by hostname
func GetDDNSRecord(ctx context.Context, hostname string) (*DDNSRecord, error) {
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "DDNS"},
			"SK": &types.AttributeValueMemberS{Value: hostname},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get record: %w", err)
	}

	if result.Item == nil {
		return nil, nil
	}

	var record DDNSRecord
	if err := attributevalue.UnmarshalMap(result.Item, &record); err != nil {
		return nil, fmt.Errorf("failed to unmarshal record: %w", err)
	}

	return &record, nil
}

// ListDDNSRecords retrieves all DDNS records
func ListDDNSRecords(ctx context.Context) ([]DDNSRecord, error) {
	result, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "DDNS"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list records: %w", err)
	}

	var records []DDNSRecord
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &records); err != nil {
		return nil, fmt.Errorf("failed to unmarshal records: %w", err)
	}

	return records, nil
}

// UpdateDDNSRecord updates an existing DDNS record
func UpdateDDNSRecord(ctx context.Context, record *DDNSRecord) error {
	record.PK = "DDNS"
	record.SK = record.Hostname
	record.LastUpdated = time.Now().UTC()

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to update record: %w", err)
	}

	return nil
}

// DeleteDDNSRecord deletes a DDNS record
func DeleteDDNSRecord(ctx context.Context, hostname string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "DDNS"},
			"SK": &types.AttributeValueMemberS{Value: hostname},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete record: %w", err)
	}

	return nil
}

// CreateUpdateLog creates an update log entry
func CreateUpdateLog(ctx context.Context, log *UpdateLog) error {
	log.PK = fmt.Sprintf("LOG#%s", log.NewIP)
	log.SK = log.Timestamp.Format(time.RFC3339Nano)
	// Set TTL to 30 days from now
	log.TTL = time.Now().Add(30 * 24 * time.Hour).Unix()

	item, err := attributevalue.MarshalMap(log)
	if err != nil {
		return fmt.Errorf("failed to marshal log: %w", err)
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to create log: %w", err)
	}

	return nil
}

// GetUpdateLogs retrieves update logs for a hostname
func GetUpdateLogs(ctx context.Context, hostname string, limit int32) ([]UpdateLog, error) {
	result, err := client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: fmt.Sprintf("LOG#%s", hostname)},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(limit),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	var logs []UpdateLog
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &logs); err != nil {
		return nil, fmt.Errorf("failed to unmarshal logs: %w", err)
	}

	return logs, nil
}
