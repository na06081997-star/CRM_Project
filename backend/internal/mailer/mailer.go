// Package mailer sends transactional e-mail (invitations, login codes,
// password resets and notifications) over SMTP with go-mail.
package mailer

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	netmail "net/mail"
	"strings"
	"time"

	"github.com/wneessen/go-mail"
)

// Mail is one message, always with a plain-text and an HTML part.
type Mail struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

type Sender interface {
	Send(ctx context.Context, m Mail) error
}

// Error is a delivery failure. Temporary failures (4xx, network) are worth
// retrying later; permanent ones (5xx: unknown mailbox, refused sender…) are not.
type Error struct {
	Temporary bool
	Reason    string
}

func (e *Error) Error() string {
	kind := "permanent"
	if e.Temporary {
		kind = "temporary"
	}
	return fmt.Sprintf("smtp %s failure: %s", kind, e.Reason)
}

// IsTemporary reports whether err is a delivery failure that may succeed on retry.
func IsTemporary(err error) bool {
	var me *Error
	return errors.As(err, &me) && me.Temporary
}

// TLS modes.
const (
	TLSImplicit = "ssl"      // TLS from the first byte (port 465, Hostinger default)
	TLSStartTLS = "starttls" // plain connection upgraded with STARTTLS (port 587), mandatory
	TLSNone     = "none"     // no encryption: local test servers only
)

type Config struct {
	Host     string
	Port     int
	TLS      string
	Username string
	Password string
	From     string // "noreply@example.com" or "Name <noreply@example.com>"
	Timeout  time.Duration
}

// SMTP sends each message on its own connection, so a broken connection
// never poisons the next send.
type SMTP struct{ cfg Config }

func NewSMTP(cfg Config) (*SMTP, error) {
	if cfg.Host == "" || cfg.From == "" {
		return nil, errors.New("mailer: SMTP host and sender are required")
	}
	switch cfg.TLS {
	case TLSImplicit, TLSStartTLS, TLSNone:
	default:
		return nil, fmt.Errorf("mailer: unknown TLS mode %q (ssl | starttls | none)", cfg.TLS)
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 20 * time.Second
	}
	return &SMTP{cfg: cfg}, nil
}

func (s *SMTP) client() (*mail.Client, error) {
	opts := []mail.Option{mail.WithPort(s.cfg.Port), mail.WithTimeout(s.cfg.Timeout)}
	switch s.cfg.TLS {
	case TLSImplicit:
		opts = append(opts, mail.WithSSL())
	case TLSStartTLS:
		opts = append(opts, mail.WithTLSPolicy(mail.TLSMandatory))
	case TLSNone:
		opts = append(opts, mail.WithTLSPolicy(mail.NoTLS))
	}
	if s.cfg.Username != "" {
		opts = append(opts, mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(s.cfg.Username), mail.WithPassword(s.cfg.Password))
	}
	return mail.NewClient(s.cfg.Host, opts...)
}

func (s *SMTP) Send(ctx context.Context, m Mail) error {
	msg := mail.NewMsg()
	if err := msg.From(s.cfg.From); err != nil {
		return &Error{Reason: "invalid sender address"}
	}
	if err := msg.To(m.To); err != nil {
		return &Error{Reason: "invalid recipient address"}
	}
	msg.Subject(m.Subject)
	msg.SetMessageIDWithValue(messageID(s.cfg.From))
	msg.SetDate()
	msg.SetBodyString(mail.TypeTextPlain, m.Text)
	if m.HTML != "" {
		msg.AddAlternativeString(mail.TypeTextHTML, m.HTML)
	}
	c, err := s.client()
	if err != nil {
		return &Error{Reason: "client setup: " + err.Error()}
	}
	if err := c.DialAndSendWithContext(ctx, msg); err != nil {
		return classify(err)
	}
	return nil
}

// classify keeps the server's answer (useful to the operator) but never the
// message itself: it holds links and codes.
func classify(err error) error {
	var se *mail.SendError
	if errors.As(err, &se) {
		reason := se.Error()
		if code := se.ErrorCode(); code > 0 {
			reason = fmt.Sprintf("%d %s", code, reason)
		}
		return &Error{Temporary: se.IsTemp() || (se.ErrorCode() >= 400 && se.ErrorCode() < 500), Reason: oneLine(reason)}
	}
	// Dial, TLS and timeout errors: the server may come back.
	return &Error{Temporary: true, Reason: oneLine(err.Error())}
}

// messageID uses the sender's domain rather than the server's hostname.
func messageID(from string) string {
	domain := "localhost"
	if a, err := netmail.ParseAddress(from); err == nil {
		if i := strings.LastIndex(a.Address, "@"); i >= 0 {
			domain = a.Address[i+1:]
		}
	}
	b := make([]byte, 16)
	rand.Read(b) //nolint:errcheck
	return hex.EncodeToString(b) + "@" + domain
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
