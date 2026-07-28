// Package notifier consumes video-failure events and emails the affected user.
// It depends on a Mailer interface so the notification logic can be tested
// without a real SMTP server.
package notifier

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/aniusch/projeto-fiapx/internal/messaging"
)

// Mailer sends a plaintext email.
type Mailer interface {
	Send(to, subject, body string) error
}

// Notifier turns failure events into emails.
type Notifier struct {
	mailer Mailer
}

// New builds a Notifier.
func New(mailer Mailer) *Notifier {
	return &Notifier{mailer: mailer}
}

// Handle emails the owner about a failed video. An event without an address is
// nothing we can act on, so it is logged and treated as handled (acked).
func (n *Notifier) Handle(_ context.Context, event messaging.VideoFailedEvent) error {
	if event.Email == "" {
		slog.Warn("failure event has no recipient; skipping", "video_id", event.VideoID)
		return nil
	}

	subject := "Falha no processamento do seu vídeo"
	body := fmt.Sprintf(
		"Olá,\n\n"+
			"Não foi possível processar o seu vídeo \"%s\".\n\n"+
			"Motivo: %s\n"+
			"Identificador: %s\n\n"+
			"Por favor, verifique o arquivo e tente novamente.\n\n"+
			"Equipe FIAP X\n",
		event.OriginalName, event.Reason, event.VideoID,
	)

	if err := n.mailer.Send(event.Email, subject, body); err != nil {
		return fmt.Errorf("send notification: %w", err)
	}

	slog.Info("failure notification sent", "video_id", event.VideoID, "to", event.Email)
	return nil
}
