package auth

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

func TestPasswordHashAndCheck(t *testing.T) {
	hash, err := HashPassword("correct-horse-battery-staple")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "correct-horse-battery-staple" {
		t.Fatal("hash must not equal the plaintext")
	}
	if !CheckPassword(hash, "correct-horse-battery-staple") {
		t.Fatal("correct password should verify")
	}
	if CheckPassword(hash, "wrong") {
		t.Fatal("wrong password should not verify")
	}
}

func TestJWTRoundTrip(t *testing.T) {
	m := NewManager("test-secret", time.Hour)
	id := uuid.New()

	token, err := m.Issue(id)
	if err != nil {
		t.Fatalf("issue: %v", err)
	}

	got, err := m.Parse(token)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got != id {
		t.Fatalf("round-trip mismatch: got %s want %s", got, id)
	}
}

func TestJWTRejectsWrongSecret(t *testing.T) {
	issuer := NewManager("secret-a", time.Hour)
	verifier := NewManager("secret-b", time.Hour)

	token, err := issuer.Issue(uuid.New())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := verifier.Parse(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for wrong secret, got %v", err)
	}
}

func TestJWTRejectsExpired(t *testing.T) {
	m := NewManager("test-secret", -time.Minute) // already expired
	token, err := m.Issue(uuid.New())
	if err != nil {
		t.Fatalf("issue: %v", err)
	}
	if _, err := m.Parse(token); err != ErrInvalidToken {
		t.Fatalf("expected ErrInvalidToken for expired token, got %v", err)
	}
}
