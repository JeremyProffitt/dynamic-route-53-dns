package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"dynamic-route-53-dns/internal/auth"
	"dynamic-route-53-dns/internal/database"

	"golang.org/x/crypto/bcrypt"
)

// AuthService handles authentication logic
type AuthService struct {
	sessionManager *auth.SessionManager
	adminUsername  string
	adminPassword  string
}

// NewAuthService creates a new auth service
func NewAuthService() *AuthService {
	return &AuthService{
		sessionManager: auth.NewSessionManager(),
		adminUsername:  os.Getenv("ADMIN_USERNAME"),
		adminPassword:  os.Getenv("ADMIN_PASSWORD"),
	}
}

// LoginResult represents the result of a login attempt
type LoginResult struct {
	Success     bool
	SessionID   string
	Error       string
	IsLocked    bool
	LockedUntil time.Time
}

// Login attempts to authenticate a user
func (s *AuthService) Login(ctx context.Context, username, password string) *LoginResult {
	// Check if account is locked
	locked, lockedUntil, err := database.IsAccountLocked(ctx, username)
	if err != nil {
		return &LoginResult{
			Success: false,
			Error:   "Internal error",
		}
	}
	if locked {
		return &LoginResult{
			Success:     false,
			IsLocked:    true,
			LockedUntil: lockedUntil,
			Error:       fmt.Sprintf("Account locked until %s", lockedUntil.Format(time.RFC3339)),
		}
	}

	// Validate credentials
	if username != s.adminUsername || password != s.adminPassword {
		// Record failed attempt
		locked, lockedUntil, _ = database.RecordLoginAttempt(ctx, username, false)
		if locked {
			return &LoginResult{
				Success:     false,
				IsLocked:    true,
				LockedUntil: lockedUntil,
				Error:       fmt.Sprintf("Account locked until %s", lockedUntil.Format(time.RFC3339)),
			}
		}
		return &LoginResult{
			Success: false,
			Error:   "Invalid username or password",
		}
	}

	// Record successful login
	_, _, _ = database.RecordLoginAttempt(ctx, username, true)

	// Create session
	sessionID, err := s.sessionManager.CreateSession(ctx, username)
	if err != nil {
		return &LoginResult{
			Success: false,
			Error:   "Failed to create session",
		}
	}

	return &LoginResult{
		Success:   true,
		SessionID: sessionID,
	}
}

// Logout removes the session
func (s *AuthService) Logout(ctx context.Context, sessionID string) error {
	return s.sessionManager.DeleteSession(ctx, sessionID)
}

// ValidateSession validates a session and returns the username
func (s *AuthService) ValidateSession(ctx context.Context, sessionID string) (string, bool) {
	return s.sessionManager.ValidateSession(ctx, sessionID)
}

// HashToken hashes a token using bcrypt
func HashToken(token string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(token), 10)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// VerifyToken verifies a token against its hash
func VerifyToken(token, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(token))
	return err == nil
}
