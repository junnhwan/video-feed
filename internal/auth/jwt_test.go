package auth

import "testing"

func TestGenerateAndParseToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")

	token, err := GenerateToken(7, "alice")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	claims, err := ParseToken(token)
	if err != nil {
		t.Fatalf("parse token: %v", err)
	}
	if claims.AccountID != 7 {
		t.Fatalf("expected account id 7, got %d", claims.AccountID)
	}
	if claims.Username != "alice" {
		t.Fatalf("expected username alice, got %q", claims.Username)
	}
}

func TestParseTokenRejectsTamperedToken(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-secret")
	token, err := GenerateToken(7, "alice")
	if err != nil {
		t.Fatalf("generate token: %v", err)
	}

	_, err = ParseToken(token + "tampered")

	if err == nil {
		t.Fatal("expected tampered token to be rejected")
	}
}
