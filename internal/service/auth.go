package service

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/dynamic-route-53-dns/internal/auth"
	"github.com/dynamic-route-53-dns/internal/database"
)

const (
	// loginAttemptPK is the partition key prefix for login attempt records.
	loginAttemptPK = "LOGIN_ATTEMPT"

	// maxFailedAttempts is the maximum number of failed login attempts before lockout.
	maxFailedAttempts = 5

	// lockoutDuration is the duration for which an IP is locked out after too many failed attempts.
	lockoutDuration = 15 * time.Minute

	// attemptWindowDuration is the time window for counting failed attempts.
	attemptWindowDuration = 15 * time.Minute
)

var (
	// ErrInvalidCredentials indicates invalid username or password.
	ErrInvalidCredentials = errors.New("invalid username or password")

	// ErrIPLocked indicates the IP address is temporarily locked due to too many failed attempts.
	ErrIPLocked = errors.New("too many failed login attempts, please try again later")
)

// LoginAttempt represents a login attempt stored in DynamoDB.
type LoginAttempt struct {
	PK          string `dynamodbav:"PK"`
	SK          string `dynamodbav:"SK"`
	ClientIP    string `dynamodbav:"client_ip"`
	Success     bool   `dynamodbav:"success"`
	AttemptedAt int64  `dynamodbav:"attempted_at"`
	FailedCount int    `dynamodbav:"failed_count"`
	LockedUntil int64  `dynamodbav:"locked_until,omitempty"`
	TTL         int64  `dynamodbav:"TTL"`
}

// AuthService handles authentication operations.
type AuthService struct {
	adminUsername  string
	adminPassword  string
	sessionManager *auth.SessionManager
	db             *database.Client
}

// NewAuthService creates a new AuthService with environment-based credentials.
func NewAuthService(db *database.Client) *AuthService {
	return &AuthService{
		adminUsername:  os.Getenv("ADMIN_USERNAME"),
		adminPassword:  os.Getenv("ADMIN_PASSWORD"),
		sessionManager: auth.NewSessionManager(db),
		db:             db,
	}
}

// Login authenticates a user and creates a session on success.
// Returns the session ID on success, or an error if authentication fails.
func (s *AuthService) Login(ctx context.Context, username, password, clientIP string) (string, error) {
	// Check if IP is locked
	locked, err := s.isIPLocked(ctx, clientIP)
	if err != nil {
		return "", err
	}
	if locked {
		return "", ErrIPLocked
	}

	// Validate credentials (plaintext comparison for env var passwords)
	if username != s.adminUsername || password != s.adminPassword {
		// Record failed attempt
		if err := s.recordLoginAttempt(ctx, clientIP, false); err != nil {
			// Log error but don't expose it to the user
			_ = err
		}
		return "", ErrInvalidCredentials
	}

	// Record successful attempt (resets failed count)
	if err := s.recordLoginAttempt(ctx, clientIP, true); err != nil {
		// Log error but continue with session creation
		_ = err
	}

	// Create session
	sessionID, err := s.sessionManager.CreateSession(ctx, username)
	if err != nil {
		return "", err
	}

	return sessionID, nil
}

// Logout destroys the session associated with the given session ID.
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	return s.sessionManager.DestroySession(ctx, sessionID)
}

// ValidateSession checks if a session is valid and returns the associated username.
func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (string, error) {
	return s.sessionManager.ValidateSession(ctx, sessionID)
}

// SessionManager returns the underlying session manager.
func (s *AuthService) SessionManager() *auth.SessionManager {
	return s.sessionManager
}

// isIPLocked checks if the given IP address is currently locked out.
func (s *AuthService) isIPLocked(ctx context.Context, clientIP string) (bool, error) {
	result, err := s.db.DB().GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.db.TableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: loginAttemptPK},
			"SK": &types.AttributeValueMemberS{Value: clientIP},
		},
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

	// Check if currently locked
	if attempt.LockedUntil > 0 && time.Now().Unix() < attempt.LockedUntil {
		return true, nil
	}

	return false, nil
}

// recordLoginAttempt records a login attempt and updates the lockout status if needed.
func (s *AuthService) recordLoginAttempt(ctx context.Context, clientIP string, success bool) error {
	now := time.Now()
	ttl := now.Add(attemptWindowDuration).Unix()

	// Get current attempt record
	result, err := s.db.DB().GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.db.TableName()),
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: loginAttemptPK},
			"SK": &types.AttributeValueMemberS{Value: clientIP},
		},
	})
	if err != nil {
		return err
	}

	var attempt LoginAttempt
	if result.Item != nil {
		if err := attributevalue.UnmarshalMap(result.Item, &attempt); err != nil {
			return err
		}
	}

	// Update attempt record
	attempt.PK = loginAttemptPK
	attempt.SK = clientIP
	attempt.ClientIP = clientIP
	attempt.Success = success
	attempt.AttemptedAt = now.Unix()
	attempt.TTL = ttl

	if success {
		// Reset failed count on successful login
		attempt.FailedCount = 0
		attempt.LockedUntil = 0
	} else {
		// Increment failed count
		// Reset count if window has passed
		windowStart := now.Add(-attemptWindowDuration).Unix()
		if attempt.AttemptedAt < windowStart {
			attempt.FailedCount = 1
		} else {
			attempt.FailedCount++
		}

		// Lock if too many failed attempts
		if attempt.FailedCount >= maxFailedAttempts {
			attempt.LockedUntil = now.Add(lockoutDuration).Unix()
			attempt.TTL = attempt.LockedUntil + 60 // Keep record a bit longer than lockout
		}
	}

	item, err := attributevalue.MarshalMap(attempt)
	if err != nil {
		return err
	}

	_, err = s.db.DB().PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.db.TableName()),
		Item:      item,
	})

	return err
}
