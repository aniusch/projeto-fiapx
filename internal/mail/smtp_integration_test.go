//go:build integration

// Integration test for the SMTP mailer against Mailpit (from docker-compose).
// Run with: go test -tags=integration ./internal/mail/...
// Overrides: TEST_SMTP_HOST, TEST_SMTP_PORT, TEST_MAILPIT_API.
package mail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"testing"
	"time"

	"github.com/aniusch/projeto-fiapx/internal/config"
)

func env(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func TestSMTPMailerSendsToMailpit(t *testing.T) {
	port, _ := strconv.Atoi(env("TEST_SMTP_PORT", "1025"))
	mailer := NewSMTPMailer(config.SMTPConfig{
		Host: env("TEST_SMTP_HOST", "localhost"),
		Port: port,
		From: "no-reply@fiapx.local",
	})

	// Unique recipient so we can find exactly this message.
	to := fmt.Sprintf("it-%d@example.com", time.Now().UnixNano())
	if err := mailer.Send(to, "Integration Test", "hello from the integration test"); err != nil {
		t.Fatalf("Send: %v", err)
	}

	api := env("TEST_MAILPIT_API", "http://localhost:8025")
	if !messageDelivered(t, api, to) {
		t.Fatalf("message to %s was not found in Mailpit", to)
	}
}

func messageDelivered(t *testing.T, api, to string) bool {
	t.Helper()
	url := api + "/api/v1/search?query=to:" + to
	// Give Mailpit a moment to index the message.
	for i := 0; i < 10; i++ {
		resp, err := http.Get(url)
		if err != nil {
			t.Fatalf("query mailpit: %v", err)
		}
		var body struct {
			Total int `json:"messages_count"`
			Msgs  []struct {
				Subject string `json:"Subject"`
			} `json:"messages"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&body)
		resp.Body.Close()
		if len(body.Msgs) > 0 {
			return true
		}
		time.Sleep(200 * time.Millisecond)
	}
	return false
}
