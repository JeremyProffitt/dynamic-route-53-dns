package database

import (
	"context"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
)

const (
	// SessionPartitionKey is the partition key value for session records.
	SessionPartitionKey = "SESSION"
	// SessionTTLHours is the number of hours before sessions expire.
	SessionTTLHours = 24
)

// Session represents a user session in the database.
type Session struct {
	PK        string    `dynamodbav:"PK"`
	SK        string    `dynamodbav:"SK"`
	Username  string    `dynamodbav:"Username"`
	CreatedAt time.Time `dynamodbav:"CreatedAt"`
	TTL       int64     `dynamodbav:"TTL"`
}

// CreateSession creates a new session in the database.
func (c *Client) CreateSession(ctx context.Context, session *Session) error {
	session.PK = SessionPartitionKey
	session.SK = session.SK // session_id should already be set
	session.CreatedAt = time.Now()
	session.TTL = time.Now().Add(SessionTTLHours * time.Hour).Unix()

	item, err := attributevalue.MarshalMap(session)
	if err != nil {
		return err
	}

	_, err = c.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(c.tableName),
		Item:      item,
	})

	return err
}

// GetSession retrieves a session by session ID.
func (c *Client) GetSession(ctx context.Context, sessionID string) (*Session, error) {
	key, err := attributevalue.MarshalMap(map[string]string{
		"PK": SessionPartitionKey,
		"SK": sessionID,
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

	var session Session
	if err := attributevalue.UnmarshalMap(result.Item, &session); err != nil {
		return nil, err
	}

	// Check if session has expired (TTL might not have been enforced yet by DynamoDB)
	if session.TTL < time.Now().Unix() {
		return nil, ErrRecordNotFound
	}

	return &session, nil
}

// DeleteSession deletes a session by session ID.
func (c *Client) DeleteSession(ctx context.Context, sessionID string) error {
	key, err := attributevalue.MarshalMap(map[string]string{
		"PK": SessionPartitionKey,
		"SK": sessionID,
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

// NewSession creates a new Session with the partition key set correctly.
func NewSession(sessionID, username string) *Session {
	return &Session{
		PK:       SessionPartitionKey,
		SK:       sessionID,
		Username: username,
	}
}
