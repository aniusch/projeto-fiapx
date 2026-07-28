package mail

import (
	"strings"
	"testing"
)

func TestBuildMessage(t *testing.T) {
	msg := string(buildMessage("from@fiapx.local", "to@example.com", "Test Subject", "Line one\nLine two"))

	for _, want := range []string{
		"From: from@fiapx.local\r\n",
		"To: to@example.com\r\n",
		"Subject: Test Subject\r\n",
		"MIME-Version: 1.0\r\n",
		"Content-Type: text/plain; charset=\"UTF-8\"\r\n",
	} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing header %q", want)
		}
	}

	// Headers must be separated from the body by a blank line (CRLFCRLF).
	sep := "\r\n\r\n"
	idx := strings.Index(msg, sep)
	if idx < 0 {
		t.Fatal("message has no header/body separator")
	}
	body := msg[idx+len(sep):]
	if body != "Line one\nLine two" {
		t.Errorf("body = %q, want the original text", body)
	}
}
