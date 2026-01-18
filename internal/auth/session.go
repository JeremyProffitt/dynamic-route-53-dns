package auth

import (
	"context"
	"errors"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/dynamic-route-53-dns/internal/database"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

const (
	// SessionCookieName is the name of the session cookie.
	SessionCookieName = "session_id"

	// SessionTTL is the duration for which a session is valid (24 hours).
	SessionTTL = 24 * time.Hour

	// sessionPK is the partition key prefix for session records.
	sessionPK = "SESSION"
)

var (
	// ErrSessionNotFound indicates the session does not exist or has expired.
	ErrSessionNotFound = errors.New("session not found")

	// ErrInvalidSession indicates the session is invalid.
	ErrInvalidSession = errors.New("invalid session")
)

// Session represents a user session stored in DynamoDB.
type Session struct {
	PK        string `dynamodbav:"PK"`
	SK        string `dynamodbav:"SK"`
	Username  string `dynamodbav:"username"`
	CreatedAt int64  `dynamodbav:"created_at"`
	ExpiresAt int64  `dynamodbav:"expires_at"`
	TTL       int64  `dynamodbav:"TTL"`
}

// SessionManager handles session creation, validation, and destruction.
type SessionManager struct {
	db *database.Client
}

// NewSessionManager creates a new SessionManager with the given database client.
func NewSessionManager(db *database.Client) *SessionManager {
	return &SessionManager{
		db: db,
	}
}

// CreateSession creates a new session for the given username.
// Returns the session ID (UUID) on success.
func (sm *SessionManager) CreateSession(ctx context.Context, username string) (string, error) {
	sessionID := uuid.New().String()
	now := time.Now()
	expiresAt := now.Add(SessionTTL)

	session := Session{
		PK:        sessionPK,
		SK:        sessionID,
		Username:  username,
		CreatedAt: now.Unix(),
		ExpiresAt: expiresAt.Unix(),
		TTL:       expiresAt.Unix(),
	}

	item, err := attributevalue.MarshalMap(session)
	if err != nil {
		return "", err
	}

	_, err = sm.db.DB().PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(sm.db.TableName()),
		Item:      item,
	})
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

// ValidateSession checks if a session is valid and returns the associated username.
// Returns ErrSessionNotFound if the session does not exist or has expired.
func (sm *SessionManager) ValidateSession(ctx context.Context, sessionID string) (string, error) {
	if sessionID == "" {
		return "", ErrInvalidSession
	}

	result, err := sm.db.DB().GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(sm.db.TableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: sessionPK},
			"SK": &types.AttributeValueMemberS{Value: sessionID},
		},
	})
	if err != nil {
		return "", err
	}

	if result.Item == nil {
		return "", ErrSessionNotFound
	}

	var session Session
	if err := attributevalue.UnmarshalMap(result.Item, &session); err != nil {
		return "", err
	}

	// Check if session has expired (belt and suspenders with DynamoDB TTL)
	if time.Now().Unix() > session.ExpiresAt {
		// Clean up expired session
		_ = sm.DestroySession(ctx, sessionID)
		return "", ErrSessionNotFound
	}

	return session.Username, nil
}

// DestroySession removes a session from the database.
func (sm *SessionManager) DestroySession(ctx context.Context, sessionID string) error {
	if sessionID == "" {
		return nil
	}

	_, err := sm.db.DB().DeleteItem(ctx, &dynamodb.DeleteItemInput{
		TableName: aws.String(sm.db.TableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: sessionPK},
			"SK": &types.AttributeValueMemberS{Value: sessionID},
		},
	})

	return err
}

// GetSessionFromCookie extracts the session ID from the request cookie.
// Returns an empty string if the cookie is not present.
func (sm *SessionManager) GetSessionFromCookie(c *fiber.Ctx) (string, error) {
	sessionID := c.Cookies(SessionCookieName)
	if sessionID == "" {
		return "", ErrSessionNotFound
	}
	return sessionID, nil
}

// SetSessionCookie sets the session cookie on the response.
func (sm *SessionManager) SetSessionCookie(c *fiber.Ctx, sessionID string) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    sessionID,
		Path:     "/",
		Expires:  time.Now().Add(SessionTTL),
		Secure:   true,
		HTTPOnly: true,
		SameSite: "Strict",
	})
}

// ClearSessionCookie removes the session cookie from the response.
func (sm *SessionManager) ClearSessionCookie(c *fiber.Ctx) {
	c.Cookie(&fiber.Cookie{
		Name:     SessionCookieName,
		Value:    "",
		Path:     "/",
		Expires:  time.Now().Add(-time.Hour),
		Secure:   true,
		HTTPOnly: true,
		SameSite: "Strict",
	})
}
