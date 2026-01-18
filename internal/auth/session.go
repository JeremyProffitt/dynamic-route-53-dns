package auth

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"time"

	"dynamic-route-53-dns/internal/database"

	"github.com/google/uuid"
)

// SessionManager manages user sessions
type SessionManager struct{}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{}
}

// CreateSession creates a new session for a user
func (sm *SessionManager) CreateSession(ctx context.Context, username string) (string, error) {
	sessionID := uuid.New().String()

	session := &database.Session{
		SessionID: sessionID,
		Username:  username,
	}

	if err := database.CreateSession(ctx, session); err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return sessionID, nil
}

// ValidateSession validates a session and returns the username
func (sm *SessionManager) ValidateSession(ctx context.Context, sessionID string) (string, bool) {
	session, err := database.GetSession(ctx, sessionID)
	if err != nil || session == nil {
		return "", false
	}

	// Check expiration
	if time.Now().UTC().After(session.ExpiresAt) {
		_ = database.DeleteSession(ctx, sessionID)
		return "", false
	}

	return session.Username, true
}

// DeleteSession removes a session
func (sm *SessionManager) DeleteSession(ctx context.Context, sessionID string) error {
	return database.DeleteSession(ctx, sessionID)
}

// GenerateCSRFToken generates a new CSRF token
func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

// GenerateUpdateToken generates a new update token for DDNS
func GenerateUpdateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
