// Package notify delivers reminders and alerts over pluggable channels.
// Free channels: console (logs), telegram, email (SMTP). Paid SMS: twilio, sms019.
package notify

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

type Recipient struct {
	UserID         *uuid.UUID
	Name           string
	Phone          string
	Email          string
	TelegramChatID string
}

type Message struct {
	To      Recipient
	Subject string
	Body    string
}

// Channel sends a message. It returns ErrNoAddress when the recipient has no
// address for this channel, so the dispatcher can silently skip it.
type Channel interface {
	Name() string
	Send(ctx context.Context, m Message) (address string, err error)
}

var ErrNoAddress = errors.New("recipient has no address for this channel")

// Recorder persists each delivery attempt (implemented by the store).
type Recorder interface {
	CreateNotification(ctx context.Context, n domain.Notification) (domain.Notification, error)
}

type Dispatcher struct {
	channels []Channel
	rec      Recorder
}

func NewDispatcher(cfg *config.Config, rec Recorder) *Dispatcher {
	d := &Dispatcher{rec: rec}
	for _, name := range cfg.NotifyChannels {
		switch strings.ToLower(name) {
		case "console":
			d.channels = append(d.channels, Console{})
		case "telegram":
			if cfg.TelegramBotToken != "" {
				d.channels = append(d.channels, NewTelegram(cfg.TelegramBotToken))
			}
		case "email":
			if cfg.SMTPHost != "" && cfg.SMTPFrom != "" {
				d.channels = append(d.channels, Email{Host: cfg.SMTPHost, Port: cfg.SMTPPort,
					User: cfg.SMTPUser, Password: cfg.SMTPPassword, From: cfg.SMTPFrom})
			}
		case "twilio":
			if cfg.TwilioAccountSID != "" {
				d.channels = append(d.channels, NewTwilio(cfg.TwilioAccountSID, cfg.TwilioAuthToken, cfg.TwilioFrom))
			}
		case "sms019":
			if cfg.SMS019Token != "" {
				d.channels = append(d.channels, NewSMS019(cfg.SMS019Username, cfg.SMS019Token, cfg.SMS019Source))
			}
		default:
			slog.Warn("notify: unknown channel ignored", "channel", name)
		}
	}
	if len(d.channels) == 0 {
		d.channels = []Channel{Console{}}
	}
	names := make([]string, len(d.channels))
	for i, c := range d.channels {
		names[i] = c.Name()
	}
	slog.Info("notify: channels enabled", "channels", names)
	return d
}

// Send delivers m over every enabled channel the recipient can be reached on
// and records each attempt. It returns an error only if no channel succeeded.
func (d *Dispatcher) Send(ctx context.Context, m Message) error {
	var delivered bool
	var errs []error
	for _, ch := range d.channels {
		addr, err := ch.Send(ctx, m)
		if errors.Is(err, ErrNoAddress) {
			continue
		}
		n := domain.Notification{UserID: m.To.UserID, Channel: ch.Name(), Recipient: addr, Subject: m.Subject, Body: m.Body}
		if err != nil {
			n.Status, n.Error = "failed", err.Error()
			errs = append(errs, fmt.Errorf("%s: %w", ch.Name(), err))
		} else {
			now := time.Now()
			n.Status, n.SentAt = "sent", &now
			delivered = true
		}
		if d.rec != nil {
			if _, rerr := d.rec.CreateNotification(ctx, n); rerr != nil {
				slog.Error("notify: record failed", "err", rerr)
			}
		}
	}
	if !delivered && len(errs) > 0 {
		return errors.Join(errs...)
	}
	return nil
}

// Console logs messages; always "delivers". Ideal for development.
type Console struct{}

func (Console) Name() string { return "console" }

func (Console) Send(_ context.Context, m Message) (string, error) {
	addr := m.To.Phone
	if addr == "" {
		addr = m.To.Email
	}
	if addr == "" {
		addr = m.To.Name
	}
	slog.Info("📨 notification", "to", addr, "subject", m.Subject, "body", m.Body)
	return addr, nil
}

// NormalizeIsraeliPhone converts local numbers (050-1234567) to E.164 (+972501234567).
func NormalizeIsraeliPhone(p string) string {
	var b strings.Builder
	for _, r := range p {
		if (r >= '0' && r <= '9') || r == '+' {
			b.WriteRune(r)
		}
	}
	s := b.String()
	switch {
	case s == "":
		return ""
	case strings.HasPrefix(s, "+"):
		return s
	case strings.HasPrefix(s, "972"):
		return "+" + s
	case strings.HasPrefix(s, "0"):
		return "+972" + s[1:]
	}
	return s
}
