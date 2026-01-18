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

// RateLimitEntry represents a rate limit entry
type RateLimitEntry struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	Count     int    `dynamodbav:"count"`
	WindowEnd int64  `dynamodbav:"window_end"`
	TTL       int64  `dynamodbav:"ttl"`
}

// LoginAttempt represents a login attempt tracking entry
type LoginAttempt struct {
	PK           string    `dynamodbav:"PK"`
	SK           string    `dynamodbav:"SK"`
	FailedCount  int       `dynamodbav:"failed_count"`
	LastAttempt  time.Time `dynamodbav:"last_attempt"`
	LockedUntil  time.Time `dynamodbav:"locked_until"`
	TTL          int64     `dynamodbav:"ttl"`
}

// IncrementRateLimit increments the rate limit counter for a key
// Returns the current count and whether the limit is exceeded
func IncrementRateLimit(ctx context.Context, key string, limit int, windowSeconds int64) (int, bool, error) {
	now := time.Now().Unix()
	windowEnd := now + windowSeconds

	// Try to update existing entry
	result, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "RATELIMIT"},
			"SK": &types.AttributeValueMemberS{Value: key},
		},
		UpdateExpression: aws.String("SET #count = if_not_exists(#count, :zero) + :one, window_end = if_not_exists(window_end, :windowEnd), #ttl = :ttl"),
		ExpressionAttributeNames: map[string]string{
			"#count": "count",
			"#ttl":   "ttl",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":zero":      &types.AttributeValueMemberN{Value: "0"},
			":one":       &types.AttributeValueMemberN{Value: "1"},
			":windowEnd": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowEnd)},
			":ttl":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowEnd+60)},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return 0, false, fmt.Errorf("failed to increment rate limit: %w", err)
	}

	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Attributes, &entry); err != nil {
		return 0, false, fmt.Errorf("failed to unmarshal rate limit: %w", err)
	}

	// Check if window has expired
	if now > entry.WindowEnd {
		// Reset the counter
		_, err = client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: "RATELIMIT"},
				"SK": &types.AttributeValueMemberS{Value: key},
			},
			UpdateExpression: aws.String("SET #count = :one, window_end = :windowEnd, #ttl = :ttl"),
			ExpressionAttributeNames: map[string]string{
				"#count": "count",
				"#ttl":   "ttl",
			},
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":one":       &types.AttributeValueMemberN{Value: "1"},
				":windowEnd": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowEnd)},
				":ttl":       &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", windowEnd+60)},
			},
		})
		if err != nil {
			return 0, false, fmt.Errorf("failed to reset rate limit: %w", err)
		}
		return 1, false, nil
	}

	return entry.Count, entry.Count > limit, nil
}

// GetRateLimitCount returns the current rate limit count for a key
func GetRateLimitCount(ctx context.Context, key string) (int, error) {
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "RATELIMIT"},
			"SK": &types.AttributeValueMemberS{Value: key},
		},
	})
	if err != nil {
		return 0, fmt.Errorf("failed to get rate limit: %w", err)
	}

	if result.Item == nil {
		return 0, nil
	}

	var entry RateLimitEntry
	if err := attributevalue.UnmarshalMap(result.Item, &entry); err != nil {
		return 0, fmt.Errorf("failed to unmarshal rate limit: %w", err)
	}

	// Check if window has expired
	if time.Now().Unix() > entry.WindowEnd {
		return 0, nil
	}

	return entry.Count, nil
}

// RecordLoginAttempt records a login attempt and returns whether the account is locked
func RecordLoginAttempt(ctx context.Context, username string, success bool) (bool, time.Time, error) {
	now := time.Now().UTC()

	if success {
		// Clear failed attempts on success
		_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
			TableName: aws.String(tableName),
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: "LOGIN_ATTEMPT"},
				"SK": &types.AttributeValueMemberS{Value: username},
			},
		})
		return false, time.Time{}, err
	}

	// Get current attempts
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "LOGIN_ATTEMPT"},
			"SK": &types.AttributeValueMemberS{Value: username},
		},
	})
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to get login attempts: %w", err)
	}

	var attempt LoginAttempt
	if result.Item != nil {
		if err := attributevalue.UnmarshalMap(result.Item, &attempt); err != nil {
			return false, time.Time{}, fmt.Errorf("failed to unmarshal login attempt: %w", err)
		}
	}

	// Check if currently locked
	if !attempt.LockedUntil.IsZero() && now.Before(attempt.LockedUntil) {
		return true, attempt.LockedUntil, nil
	}

	// Increment failed count
	attempt.PK = "LOGIN_ATTEMPT"
	attempt.SK = username
	attempt.FailedCount++
	attempt.LastAttempt = now

	// Lock after 5 failed attempts
	if attempt.FailedCount >= 5 {
		attempt.LockedUntil = now.Add(15 * time.Minute)
		attempt.FailedCount = 0 // Reset count after locking
	}

	// Set TTL to 1 hour from now
	attempt.TTL = now.Add(1 * time.Hour).Unix()

	item, err := attributevalue.MarshalMap(attempt)
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to marshal login attempt: %w", err)
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to record login attempt: %w", err)
	}

	isLocked := !attempt.LockedUntil.IsZero() && now.Before(attempt.LockedUntil)
	return isLocked, attempt.LockedUntil, nil
}

// IsAccountLocked checks if an account is currently locked
func IsAccountLocked(ctx context.Context, username string) (bool, time.Time, error) {
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "LOGIN_ATTEMPT"},
			"SK": &types.AttributeValueMemberS{Value: username},
		},
	})
	if err != nil {
		return false, time.Time{}, fmt.Errorf("failed to get login attempts: %w", err)
	}

	if result.Item == nil {
		return false, time.Time{}, nil
	}

	var attempt LoginAttempt
	if err := attributevalue.UnmarshalMap(result.Item, &attempt); err != nil {
		return false, time.Time{}, fmt.Errorf("failed to unmarshal login attempt: %w", err)
	}

	if !attempt.LockedUntil.IsZero() && time.Now().UTC().Before(attempt.LockedUntil) {
		return true, attempt.LockedUntil, nil
	}

	return false, time.Time{}, nil
}
