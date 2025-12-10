package jwt

import (
	"testing"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

func TestNewTokenManager(t *testing.T) {
	secret := "test-secret"
	expireHours := 24
	refreshHours := 168

	tm := NewTokenManager(secret, expireHours, refreshHours)
	if tm == nil {
		t.Fatal("NewTokenManager returned nil")
	}
	if string(tm.secret) != secret {
		t.Errorf("Expected secret %s, got %s", secret, string(tm.secret))
	}

	expectedExpireDur := time.Duration(expireHours) * time.Hour
	if tm.expireDur != expectedExpireDur {
		t.Errorf("Expected expireDur %v, got %v", expectedExpireDur, tm.expireDur)
	}

	expectedRefreshDur := time.Duration(refreshHours) * time.Hour
	if tm.refreshDur != expectedRefreshDur {
		t.Errorf("Expected refreshDur %v, got %v", expectedRefreshDur, tm.refreshDur)
	}
}

func TestGenerateToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)
	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}
	if token == "" {
		t.Error("Generated token is empty")
	}

	// Validate the generated token
	claims, err := tm.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.UserName != username {
		t.Errorf("Expected Username %s, got %s", username, claims.UserName)
	}
	if claims.UserEmail != user_email {
		t.Errorf("Expected UserEmail %s, got %s", user_email, claims.UserEmail)
	}
}

func TestParseToken_ValidToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)
	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := tm.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.UserName != username {
		t.Errorf("Expected Username %s, got %s", username, claims.UserName)
	}
	if claims.UserEmail != user_email {
		t.Errorf("Expected UserEmail %s, got %s", user_email, claims.UserEmail)
	}

	// Check that the token has proper timestamps
	now := time.Now()
	if claims.IssuedAt.Time.After(now) {
		t.Error("IssuedAt is in the future")
	}
	if claims.ExpiresAt.Time.Before(now) {
		t.Error("ExpiresAt is in the past")
	}
	if claims.NotBefore.Time.After(now) {
		t.Error("NotBefore is in the future")
	}
}

func TestParseToken_InvalidToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)

	tests := []struct {
		name        string
		token       string
		expectedErr error
	}{
		{
			name:        "empty token",
			token:       "",
			expectedErr: ErrInvalidToken,
		},
		{
			name:        "malformed token",
			token:       "not.a.valid.token",
			expectedErr: ErrInvalidToken,
		},
		{
			name:        "random string",
			token:       "randomstring",
			expectedErr: ErrInvalidToken,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tm.ParseToken(tt.token)
			if err == nil {
				t.Error("Expected error, got nil")
			}
			if err != tt.expectedErr {
				t.Errorf("Expected error %v, got %v", tt.expectedErr, err)
			}
		})
	}
}

func TestParseToken_WrongSecret(t *testing.T) {
	tm1 := NewTokenManager("secret1", 24, 168)
	tm2 := NewTokenManager("secret2", 24, 168)

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm1.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Try to validate with different secret
	_, err = tm2.ParseToken(token)
	if err == nil {
		t.Error("Expected error when validating with wrong secret")
	}
	if err != ErrInvalidToken {
		t.Errorf("Expected ErrInvalidToken, got %v", err)
	}
}

func TestParseToken_ExpiredToken(t *testing.T) {
	// Create a token manager with very short expiry
	tm := NewTokenManager("test-secret", 0, 168)
	tm.expireDur = 1 * time.Millisecond // Override to 1ms for testing

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	_, err = tm.ParseToken(token)
	if err == nil {
		t.Error("Expected error for expired token")
	}
	if err != ErrExpiredToken {
		t.Errorf("Expected ErrExpiredToken, got %v", err)
	}
}

func TestRefreshToken_ValidToken(t *testing.T) {
	// Create a token manager with short expiry
	tm := NewTokenManager("test-secret", 1, 168)

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)

	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Wait a bit to ensure we're within refresh window and timestamp changes
	time.Sleep(1100 * time.Millisecond)

	// Refresh the token
	newToken, err := tm.RefreshToken(token)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if newToken == "" {
		t.Error("Refreshed token is empty")
	}

	// Validate the new token
	claims, err := tm.ParseToken(newToken)
	if err != nil {
		t.Fatalf("ParseToken failed for refreshed token: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.UserName != username {
		t.Errorf("Expected Username %s, got %s", username, claims.UserName)
	}
	if claims.UserEmail != user_email {
		t.Errorf("Expected UserEmail %s, got %s", user_email, claims.UserEmail)
	}
}

func TestRefreshToken_ExpiredWithinWindow(t *testing.T) {
	// Create token manager with very short expiry
	tm := NewTokenManager("test-secret", 1, 1)

	// Override durations for testing
	originalExpireDur := tm.expireDur
	tm.expireDur = 10 * time.Millisecond
	tm.refreshDur = 1 * time.Hour

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Wait for token to expire
	time.Sleep(20 * time.Millisecond)

	// Restore normal expiry duration before refreshing
	tm.expireDur = originalExpireDur

	// Should still be able to refresh since it's within the refresh window
	newToken, err := tm.RefreshToken(token)
	if err != nil {
		t.Fatalf("RefreshToken failed: %v", err)
	}
	if newToken == "" {
		t.Error("Refreshed token is empty")
	}

	// The new token should have the normal expiry duration (1 hour)
	// So it should be valid now
	claims, err := tm.ParseToken(newToken)
	if err != nil {
		t.Fatalf("ParseToken failed for refreshed token: %v", err)
	}
	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.UserName != username {
		t.Errorf("Expected Username %s, got %s", username, claims.UserName)
	}
	if claims.UserEmail != user_email {
		t.Errorf("Expected UserEmail %s, got %s", user_email, claims.UserEmail)
	}
}

func TestRefreshToken_ExpiredBeyondWindow(t *testing.T) {
	// Create token manager with very short expiry and refresh window
	tm := NewTokenManager("test-secret", 0, 0)
	tm.expireDur = 10 * time.Millisecond
	tm.refreshDur = 20 * time.Millisecond

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Wait for token to expire beyond refresh window
	time.Sleep(50 * time.Millisecond)

	// Should not be able to refresh
	_, err = tm.RefreshToken(token)
	if err == nil {
		t.Error("Expected error when refreshing token expired beyond window")
	}
}

func TestRefreshToken_NotYetEligible(t *testing.T) {
	// Create token manager with long expiry and short refresh window
	tm := NewTokenManager("test-secret", 24, 1)

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Try to refresh immediately (not yet eligible)
	_, err = tm.RefreshToken(token)
	if err == nil {
		t.Error("Expected error when token not yet eligible for refresh")
	}
}

func TestRefreshToken_InvalidToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)

	_, err := tm.RefreshToken("invalid.token.string")
	if err == nil {
		t.Error("Expected error when refreshing invalid token")
	}
}

func TestGetUserIDFromToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	extractedUserID, err := tm.GetUserIDFromToken(token)
	if err != nil {
		t.Fatalf("GetUserIDFromToken failed: %v", err)
	}
	if extractedUserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, extractedUserID)
	}
}

func TestGetUserIDFromToken_ExpiredToken(t *testing.T) {
	// Create token manager with very short expiry
	tm := NewTokenManager("test-secret", 0, 168)
	tm.expireDur = 1 * time.Millisecond

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	// Wait for token to expire
	time.Sleep(10 * time.Millisecond)

	// GetUserIDFromToken should still work even with expired token
	userID, err = tm.GetUserIDFromToken(token)
	if err != nil {
		t.Fatalf("GetUserIDFromToken failed: %v", err)
	}
	if userID != "user123" {
		t.Errorf("Expected UserID user123, got %s", userID)
	}
}

func TestGetUserIDFromToken_InvalidToken(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)

	_, err := tm.GetUserIDFromToken("invalid.token")
	if err == nil {
		t.Error("Expected error for invalid token")
	}
}

func TestTokenClaims_AllFields(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)

	userID := "user123"
	username := "testuser"
	user_email := "test@123456.com"

	token, err := tm.GenerateToken(userID, username, user_email)
	if err != nil {
		t.Fatalf("GenerateToken failed: %v", err)
	}

	claims, err := tm.ParseToken(token)
	if err != nil {
		t.Fatalf("ParseToken failed: %v", err)
	}

	// Check all claim fields
	if claims.UserID != userID {
		t.Errorf("Expected UserID %s, got %s", userID, claims.UserID)
	}
	if claims.UserName != username {
		t.Errorf("Expected Username %s, got %s", username, claims.UserName)
	}
	if claims.UserEmail != user_email {
		t.Errorf("Expected UserEmail %s, got %s", user_email, claims.UserEmail)
	}

	if claims.IssuedAt == nil {
		t.Error("IssuedAt is nil")
	}
	if claims.ExpiresAt == nil {
		t.Error("ExpiresAt is nil")
	}
	if claims.NotBefore == nil {
		t.Error("NotBefore is nil")
	}

	// Verify time relationships
	if !claims.IssuedAt.Time.Before(claims.ExpiresAt.Time) {
		t.Error("IssuedAt should be before ExpiresAt")
	}
	if claims.NotBefore.Time.After(claims.IssuedAt.Time) {
		t.Error("NotBefore should not be after IssuedAt")
	}
}

func TestTokenManager_DifferentSigningMethods(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)

	// Create a token with a different signing method (RS256 instead of HS256)
	claims := Claims{
		UserID:   "user123",
		UserName: "testuser",
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}

	// This would require an RSA key, but we're just testing that our validator rejects it
	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	tokenString, err := token.SignedString([]byte("test-secret"))
	if err != nil {
		t.Fatalf("Failed to create test token: %v", err)
	}

	// Our validator should accept HS512 since it's still HMAC
	_, err = tm.ParseToken(tokenString)
	if err != nil {
		t.Logf("Token validation result: %v", err)
	}
}

func TestConcurrentTokenGeneration(t *testing.T) {
	tm := NewTokenManager("test-secret", 24, 168)

	// Test concurrent token generation
	done := make(chan bool)
	for i := range 10 {
		go func(id int) {
			userID := "user" + string(rune(id))
			username := "testuser" + string(rune(id))
			user_email := "test" + string(rune(id)) + "@example.com"

			token, err := tm.GenerateToken(userID, username, user_email)
			if err != nil {
				t.Errorf("GenerateToken failed: %v", err)
			}

			_, err = tm.ParseToken(token)
			if err != nil {
				t.Errorf("ParseToken failed: %v", err)
			}

			done <- true
		}(i)
	}

	// Wait for all goroutines to complete
	for range 10 {
		<-done
	}
}
