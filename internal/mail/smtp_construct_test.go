package mail

import (
	"testing"

	"github.com/aniusch/projeto-fiapx/internal/config"
)

func TestNewSMTPMailerNoAuth(t *testing.T) {
	m := NewSMTPMailer(config.SMTPConfig{Host: "localhost", Port: 1025, From: "no-reply@fiapx.local"})
	if m.auth != nil {
		t.Fatal("no credentials configured, so auth should be nil")
	}
	if m.addr != "localhost:1025" {
		t.Fatalf("addr = %q, want localhost:1025", m.addr)
	}
	if m.from != "no-reply@fiapx.local" {
		t.Fatalf("from = %q", m.from)
	}
}

func TestNewSMTPMailerWithAuth(t *testing.T) {
	m := NewSMTPMailer(config.SMTPConfig{
		Host: "smtp.example.com", Port: 587,
		Username: "user", Password: "pass", From: "a@b.com",
	})
	if m.auth == nil {
		t.Fatal("credentials configured, so auth should be set")
	}
	if m.addr != "smtp.example.com:587" {
		t.Fatalf("addr = %q", m.addr)
	}
}
