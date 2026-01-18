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

// Session represents a user session
type Session struct {
	PK        string    `dynamodbav:"PK"`
	SK        string    `dynamodbav:"SK"`
	SessionID string    `dynamodbav:"session_id"`
	Username  string    `dynamodbav:"username"`
	CreatedAt time.Time `dynamodbav:"created_at"`
	ExpiresAt time.Time `dynamodbav:"expires_at"`
	TTL       int64     `dynamodbav:"ttl"`
}

// CreateSession creates a new session
func CreateSession(ctx context.Context, session *Session) error {
	session.PK = "SESSION"
	session.SK = session.SessionID
	session.CreatedAt = time.Now().UTC()
	session.ExpiresAt = session.CreatedAt.Add(24 * time.Hour)
	session.TTL = session.ExpiresAt.Unix()

	item, err := attributevalue.MarshalMap(session)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	_, err = client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(tableName),
		Item:      item,
	})
	if err != nil {
		return fmt.Errorf("failed to create session: %w", err)
	}

	return nil
}

// GetSession retrieves a session by ID
func GetSession(ctx context.Context, sessionID string) (*Session, error) {
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "SESSION"},
			"SK": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	if result.Item == nil {
		return nil, nil
	}

	var session Session
	if err := attributevalue.UnmarshalMap(result.Item, &session); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	// Check if session is expired
	if time.Now().UTC().After(session.ExpiresAt) {
		return nil, nil
	}

	return &session, nil
}

// DeleteSession deletes a session
func DeleteSession(ctx context.Context, sessionID string) error {
	_, err := client.DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(tableName),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "SESSION"},
			"SK": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to delete session: %w", err)
	}

	return nil
}
