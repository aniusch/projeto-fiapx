// Package mail sends email over SMTP. It drives the SMTP conversation manually
// (rather than using smtp.SendMail) so it works both against a plain dev server
// like Mailpit and against a real relay that needs STARTTLS and authentication.
package mail

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"time"

	"github.com/aniusch/projeto-fiapx/internal/config"
)

// SMTPMailer sends plaintext emails via an SMTP server.
type SMTPMailer struct {
	addr     string // host:port
	host     string // used for TLS server name and auth realm
	from     string
	auth     smtp.Auth // nil when no credentials are configured
	startTLS bool
}

// NewSMTPMailer builds a mailer from configuration. Credentials enable PLAIN
// auth; StartTLS upgrades the connection before authenticating.
func NewSMTPMailer(cfg config.SMTPConfig) *SMTPMailer {
	var auth smtp.Auth
	if cfg.Username != "" {
		auth = smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
	}
	return &SMTPMailer{
		addr:     net.JoinHostPort(cfg.Host, strconv.Itoa(cfg.Port)),
		host:     cfg.Host,
		from:     cfg.From,
		auth:     auth,
		startTLS: cfg.StartTLS,
	}
}

// Send delivers a single plaintext message. It returns an error on any SMTP
// failure so the caller can decide whether to retry.
func (m *SMTPMailer) Send(to, subject, body string) error {
	client, err := smtp.Dial(m.addr)
	if err != nil {
		return fmt.Errorf("dial smtp: %w", err)
	}
	defer client.Close()

	if m.startTLS {
		if err := client.StartTLS(&tls.Config{ServerName: m.host}); err != nil {
			return fmt.Errorf("starttls: %w", err)
		}
	}
	if m.auth != nil {
		if err := client.Auth(m.auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}

	if err := client.Mail(m.from); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("rcpt to: %w", err)
	}

	w, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := w.Write(buildMessage(m.from, to, subject, body)); err != nil {
		return fmt.Errorf("write body: %w", err)
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}

	return client.Quit()
}

// buildMessage assembles an RFC 5322 message. SMTP requires CRLF line endings.
func buildMessage(from, to, subject, body string) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "From: %s\r\n", from)
	fmt.Fprintf(&b, "To: %s\r\n", to)
	fmt.Fprintf(&b, "Subject: %s\r\n", subject)
	fmt.Fprintf(&b, "Date: %s\r\n", time.Now().Format(time.RFC1123Z))
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=\"UTF-8\"\r\n")
	b.WriteString("\r\n") // blank line separates headers from body
	b.WriteString(body)
	return b.Bytes()
}
