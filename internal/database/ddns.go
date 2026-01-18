package database

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	// DDNSPartitionKey is the partition key value for DDNS records.
	DDNSPartitionKey = "DDNS"
	// LogPartitionKeyPrefix is the prefix for update log partition keys.
	LogPartitionKeyPrefix = "LOG#"
	// UpdateLogTTLDays is the number of days before update logs expire.
	UpdateLogTTLDays = 30
)

// DDNSRecord represents a Dynamic DNS record in the database.
type DDNSRecord struct {
	PK              string    `dynamodbav:"PK"`
	SK              string    `dynamodbav:"SK"`
	Hostname        string    `dynamodbav:"Hostname"`
	ZoneID          string    `dynamodbav:"ZoneID"`
	ZoneName        string    `dynamodbav:"ZoneName"`
	TTL             int       `dynamodbav:"TTL"`
	UpdateTokenHash string    `dynamodbav:"UpdateTokenHash"`
	CurrentIP       string    `dynamodbav:"CurrentIP"`
	Enabled         bool      `dynamodbav:"Enabled"`
	LastUpdated     time.Time `dynamodbav:"LastUpdated"`
	CreatedAt       time.Time `dynamodbav:"CreatedAt"`
}

// UpdateLog represents a log entry for IP address updates.
type UpdateLog struct {
	PK         string `dynamodbav:"PK"`
	SK         string `dynamodbav:"SK"`
	PreviousIP string `dynamodbav:"PreviousIP"`
	NewIP      string `dynamodbav:"NewIP"`
	SourceIP   string `dynamodbav:"SourceIP"`
	UserAgent  string `dynamodbav:"UserAgent"`
	Status     string `dynamodbav:"Status"`
	TTL        int64  `dynamodbav:"TTL"`
}

// ErrRecordNotFound is returned when a requested record does not exist.
var ErrRecordNotFound = errors.New("record not found")

// GetDDNSRecord retrieves a DDNS record by hostname.
func (c *Client) GetDDNSRecord(ctx context.Context, hostname string) (*DDNSRecord, error) {
	key, err := attributevalue.MarshalMap(map[string]string{
		"PK": DDNSPartitionKey,
		"SK": hostname,
	})
	if err != nil {
		return nil, err
	}

	result, err := c.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key:       key,
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, ErrRecordNotFound
	}

	var record DDNSRecord
	if err := attributevalue.UnmarshalMap(result.Item, &record); err != nil {
		return nil, err
	}

	return &record, nil
}

// GetDDNSRecordByHostname retrieves a DDNS record by hostname for the update endpoint.
// This is an alias for GetDDNSRecord but makes the intent clearer at call sites.
func (c *Client) GetDDNSRecordByHostname(ctx context.Context, hostname string) (*DDNSRecord, error) {
	return c.GetDDNSRecord(ctx, hostname)
}

// ListDDNSRecords retrieves all DDNS records.
func (c *Client) ListDDNSRecords(ctx context.Context) ([]DDNSRecord, error) {
	result, err := c.db.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: DDNSPartitionKey},
		},
	})
	if err != nil {
		return nil, err
	}

	records := make([]DDNSRecord, 0, len(result.Items))
	for _, item := range result.Items {
		var record DDNSRecord
		if err := attributevalue.UnmarshalMap(item, &record); err != nil {
			return nil, err
		}
		records = append(records, record)
	}

	return records, nil
}

// CreateDDNSRecord creates a new DDNS record.
func (c *Client) CreateDDNSRecord(ctx context.Context, record *DDNSRecord) error {
	record.PK = DDNSPartitionKey
	record.SK = record.Hostname

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return err
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(c.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_not_exists(PK) AND attribute_not_exists(SK)"),
	})

	return err
}

// UpdateDDNSRecord updates an existing DDNS record.
func (c *Client) UpdateDDNSRecord(ctx context.Context, record *DDNSRecord) error {
	record.PK = DDNSPartitionKey
	record.SK = record.Hostname
	record.LastUpdated = time.Now()

	item, err := attributevalue.MarshalMap(record)
	if err != nil {
		return err
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(c.tableName),
		Item:                item,
		ConditionExpression: aws.String("attribute_exists(PK) AND attribute_exists(SK)"),
	})

	return err
}

// DeleteDDNSRecord deletes a DDNS record by hostname.
func (c *Client) DeleteDDNSRecord(ctx context.Context, hostname string) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"PK": DDNSPartitionKey,
		"SK": hostname,
	})
	if err != nil {
		return err
	}

	_, err = c.db.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(c.tableName),
		Key:       key,
	})

	return err
}

// CreateUpdateLog creates a new update log entry.
func (c *Client) CreateUpdateLog(ctx context.Context, log *UpdateLog) error {
	// Set TTL to 30 days from now
	log.TTL = time.Now().Add(UpdateLogTTLDays * 24 * time.Hour).Unix()

	item, err := attributevalue.MarshalMap(log)
	if err != nil {
		return err
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})

	return err
}

// GetUpdateLogs retrieves update logs for a hostname, ordered by most recent first.
func (c *Client) GetUpdateLogs(ctx context.Context, hostname string, limit int) ([]UpdateLog, error) {
	pk := LogPartitionKeyPrefix + hostname

	input := &dynamodb.QueryInput{
		TableName:              aws.String(c.tableName),
		KeyConditionExpression: aws.String("PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: pk},
		},
		ScanIndexForward: aws.Bool(false), // Descending order (most recent first)
	}

	if limit > 0 {
		input.Limit = aws.Int32(int32(limit))
	}

	result, err := c.db.Query(ctx, input)
	if err != nil {
		return nil, err
	}

	logs := make([]UpdateLog, 0, len(result.Items))
	for _, item := range result.Items {
		var log UpdateLog
		if err := attributevalue.UnmarshalMap(item, &log); err != nil {
			return nil, err
		}
		logs = append(logs, log)
	}

	return logs, nil
}

// NewUpdateLog creates a new UpdateLog with the partition key set correctly.
func NewUpdateLog(hostname string, timestamp time.Time) *UpdateLog {
	return &UpdateLog{
		PK: LogPartitionKeyPrefix + hostname,
		SK: timestamp.Format(time.RFC3339Nano),
	}
}
