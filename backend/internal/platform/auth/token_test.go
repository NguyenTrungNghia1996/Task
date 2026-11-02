package auth

import (
	"testing"

	"github.com/google/uuid"
)

func TestPasswordHashing(t *testing.T) {
	pass := "mySecretPassword123!"
	hash, err := HashPassword(pass)
	if err != nil {
		t.Fatalf("failed to hash password: %v", err)
	}

	if !CheckPassword(hash, pass) {
		t.Errorf("expected password match, got false")
	}

	if CheckPassword(hash, "wrongPassword") {
		t.Errorf("expected password mismatch for wrong password, got true")
	}
}

func TestJWTAccessToken(t *testing.T) {
	secret := "test-secret-key-for-unit-testing-32-bytes"
	userID := uuid.New()
	orgID := uuid.New()
	username := "test_commander"
	roles := []string{"commander"}

	tokenStr, exp, err := GenerateAccessToken(secret, userID, orgID, username, roles, 15)
	if err != nil {
		t.Fatalf("failed to generate token: %v", err)
	}

	if tokenStr == "" {
		t.Errorf("expected non-empty token string")
	}
	if exp.IsZero() {
		t.Errorf("expected valid expiration time")
	}

	claims, err := ValidateAccessToken(secret, tokenStr)
	if err != nil {
		t.Fatalf("failed to validate token: %v", err)
	}

	if claims.UserID != userID {
		t.Errorf("expected userID %s, got %s", userID, claims.UserID)
	}
	if claims.OrganizationID != orgID {
		t.Errorf("expected orgID %s, got %s", orgID, claims.OrganizationID)
	}
	if claims.Username != username {
		t.Errorf("expected username %s, got %s", username, claims.Username)
	}
	if len(claims.Roles) != 1 || claims.Roles[0] != "commander" {
		t.Errorf("expected role 'commander', got %v", claims.Roles)
	}

	// Tampered secret test
	_, err = ValidateAccessToken("wrong-secret-key", tokenStr)
	if err == nil {
		t.Errorf("expected error when validating with wrong secret, got nil")
	}
}

func TestRefreshToken(t *testing.T) {
	raw, hash, err := GenerateRefreshToken()
	if err != nil {
		t.Fatalf("failed to generate refresh token: %v", err)
	}

	if len(raw) != 64 { // 32 bytes in hex = 64 characters
		t.Errorf("expected 64 chars raw token, got %d", len(raw))
	}

	recomputedHash := HashRefreshToken(raw)
	if recomputedHash != hash {
		t.Errorf("expected hash %s, got %s", hash, recomputedHash)
	}
}

