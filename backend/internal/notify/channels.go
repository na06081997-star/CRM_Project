package notify

import (
	"context"

	"github.com/na06081997-star/CRM_Project/backend/internal/mailer"
)

// ── Email (shared SMTP mailer) ───────────────────────────────

type Email struct {
	Mailer mailer.Sender
	Brand  func(ctx context.Context) mailer.Brand
}

func (e Email) Name() string { return "email" }

func (e Email) Send(ctx context.Context, m Message) (string, error) {
	if m.To.Email == "" {
		return "", ErrNoAddress
	}
	subject := m.Subject
	if subject == "" {
		subject = "תזכורת"
	}
	return m.To.Email, e.Mailer.Send(ctx, mailer.Notice(e.Brand(ctx), m.To.Email, subject, m.Body))
}
