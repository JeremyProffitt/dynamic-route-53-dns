package database

import (
	"context"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	// RateLimitPartitionKey is the partition key value for rate limit entries.
	RateLimitPartitionKey = "RATELIMIT"
	// LoginAttemptPartitionKey is the partition key value for login attempt entries.
	LoginAttemptPartitionKey = "LOGIN_ATTEMPT"
	// RateLimitWindowMinutes is the duration of the rate limit window.
	RateLimitWindowMinutes = 1
	// RateLimitTTLMinutes is the TTL for rate limit entries.
	RateLimitTTLMinutes = 5
	// LoginLockoutMinutes is the duration of login lockout after too many failed attempts.
	LoginLockoutMinutes = 15
	// LoginAttemptTTLHours is the TTL for login attempt entries.
	LoginAttemptTTLHours = 24
	// MaxLoginAttempts is the maximum number of failed login attempts before lockout.
	MaxLoginAttempts = 5
)

// RateLimitEntry represents a rate limit counter in the database.
type RateLimitEntry struct {
	PK          string    `dynamodbav:"PK"`
	SK          string    `dynamodbav:"SK"`
	Count       int       `dynamodbav:"Count"`
	WindowStart time.Time `dynamodbav:"WindowStart"`
	TTL         int64     `dynamodbav:"TTL"`
}

// LoginAttempt represents a login attempt tracking entry in the database.
type LoginAttempt struct {
	PK             string    `dynamodbav:"PK"`
	SK             string    `dynamodbav:"SK"`
	FailedAttempts int       `dynamodbav:"FailedAttempts"`
	LastAttempt    time.Time `dynamodbav:"LastAttempt"`
	LockedUntil    time.Time `dynamodbav:"LockedUntil"`
	TTL            int64     `dynamodbav:"TTL"`
}

// IncrementRateLimit increments the rate limit counter for a key and returns the new count.
// The key can be a hostname or IP address.
func (c *Client) IncrementRateLimit(ctx context.Context, key string) (int, error) {
	now := time.Now()
	windowStart := now.Truncate(RateLimitWindowMinutes * time.Minute)
	ttl := now.Add(RateLimitTTLMinutes * time.Minute).Unix()

	// Use UpdateItem with atomic counter increment
	result, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: RateLimitPartitionKey},
			"SK": &types.AttributeValueMemberS{Value: key},
		},
		UpdateExpression: aws.String("SET #count = if_not_exists(#count, :zero) + :inc, WindowStart = :windowStart, #ttl = :ttl"),
		ExpressionAttributeNames: map[string]string{
			"#count": "Count",
			"#ttl":   "TTL",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero":        &types.AttributeValueMemberN{Value: "0"},
			":inc":         &types.AttributeValueMemberN{Value: "1"},
			":windowStart": &types.AttributeValueMemberS{Value: windowStart.Format(time.RFC3339)},
			":ttl":         &types.AttributeValueMemberN{Value: formatInt64(ttl)},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return 0, err
	}

	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
		return 0, err
	}

	// If the window has changed, reset the counter
	if !entry.WindowStart.Equal(windowStart) {
		// Window has expired, reset counter
		result, err = c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(c.tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: RateLimitPartitionKey},
				"SK": &types.AttributeValueMemberS{Value: key},
			},
			UpdateExpression: aws.String("SET #count = :one, WindowStart = :windowStart, #ttl = :ttl"),
			ExpressionAttributeNames: map[string]string{
				"#count": "Count",
				"#ttl":   "TTL",
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":one":         &types.AttributeValueMemberN{Value: "1"},
				":windowStart": &types.AttributeValueMemberS{Value: windowStart.Format(time.RFC3339)},
				":ttl":         &types.AttributeValueMemberN{Value: formatInt64(ttl)},
			},
			ReturnValues: types.ReturnValueAllNew,
		})
		if err != nil {
			return 0, err
		}

		if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
			return 0, err
		}
	}

	return entry.Count, nil
}

// GetRateLimit retrieves the current rate limit entry for a key.
func (c *Client) GetRateLimit(ctx context.Context, key string) (*RateLimitEntry, error) {
	keyMap, err := attributevalue.MarshalMap(map[string]string{
		"PK": RateLimitPartitionKey,
		"SK": key,
	})
	if err != nil {
		return nil, err
	}

	result, err := c.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key:       keyMap,
	})
	if err != nil {
		return nil, err
	}

	if result.Item == nil {
		return nil, ErrRecordNotFound
	}

	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Item, &entry); err != nil {
		return nil, err
	}

	return &entry, nil
}

// RecordLoginAttempt records a login attempt and manages lockout state.
func (c *Client) RecordLoginAttempt(ctx context.Context, ip string, success bool) error {
	now := time.Now()
	ttl := now.Add(LoginAttemptTTLHours * time.Hour).Unix()

	if success {
		// Reset failed attempts on successful login
		_, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(c.tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: LoginAttemptPartitionKey},
				"SK": &types.AttributeValueMemberS{Value: ip},
			},
			UpdateExpression: aws.String("SET FailedAttempts = :zero, LastAttempt = :lastAttempt, LockedUntil = :lockedUntil, #ttl = :ttl"),
			ExpressionAttributeNames: map[string]string{
				"#ttl": "TTL",
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":zero":        &types.AttributeValueMemberN{Value: "0"},
				":lastAttempt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
				":lockedUntil": &types.AttributeValueMemberS{Value: time.Time{}.Format(time.RFC3339)},
				":ttl":         &types.AttributeValueMemberN{Value: formatInt64(ttl)},
			},
		})
		return err
	}

	// Increment failed attempts
	result, err := c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(c.tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: LoginAttemptPartitionKey},
			"SK": &types.AttributeValueMemberS{Value: ip},
		},
		UpdateExpression: aws.String("SET FailedAttempts = if_not_exists(FailedAttempts, :zero) + :inc, LastAttempt = :lastAttempt, #ttl = :ttl"),
		ExpressionAttributeNames: map[string]string{
			"#ttl": "TTL",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero":        &types.AttributeValueMemberN{Value: "0"},
			":inc":         &types.AttributeValueMemberN{Value: "1"},
			":lastAttempt": &types.AttributeValueMemberS{Value: now.Format(time.RFC3339)},
			":ttl":         &types.AttributeValueMemberN{Value: formatInt64(ttl)},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return err
	}

	var attempt LoginAttempt
	if err := attributevalue.UnmarshalMap(result.Attributes, &attempt); err != nil {
		return err
	}

	// If failed attempts exceed threshold, set lockout
	if attempt.FailedAttempts >= MaxLoginAttempts {
		lockedUntil := now.Add(LoginLockoutMinutes * time.Minute)
		_, err = c.db.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(c.tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: LoginAttemptPartitionKey},
				"SK": &types.AttributeValueMemberS{Value: ip},
			},
			UpdateExpression: aws.String("SET LockedUntil = :lockedUntil"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":lockedUntil": &types.AttributeValueMemberS{Value: lockedUntil.Format(time.RFC3339)},
			},
		})
		return err
	}

	return nil
}

// IsLoginLocked checks if an IP address is currently locked out from login attempts.
func (c *Client) IsLoginLocked(ctx context.Context, ip string) (bool, error) {
	keyMap, err := attributevalue.MarshalMap(map[string]string{
		"PK": LoginAttemptPartitionKey,
		"SK": ip,
	})
	if err != nil {
		return false, err
	}

	result, err := c.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(c.tableName),
		Key:       keyMap,
	})
	if err != nil {
		return false, err
	}

	if result.Item == nil {
		return false, nil
	}

	var attempt LoginAttempt
	if err := attributevalue.UnmarshalMap(result.Item, &attempt); err != nil {
		return false, err
	}

	// Check if still locked out
	if !attempt.LockedUntil.IsZero() && attempt.LockedUntil.After(time.Now()) {
		return true, nil
	}

	return false, nil
}

// formatInt64 formats an int64 as a string for DynamoDB number attributes.
func formatInt64(n int64) string {
	return strconv.FormatInt(n, 10)
}
