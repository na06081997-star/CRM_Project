package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/smtp"
	"net/url"
	"strconv"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

func doRequest(req *http.Request) error {
	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return nil
}

// ── Telegram (free) ──────────────────────────────────────────

type Telegram struct{ token, base string }

func NewTelegram(token string) *Telegram {
	return &Telegram{token: token, base: "https://api.telegram.org"}
}

func (t *Telegram) Name() string { return "telegram" }

func (t *Telegram) Send(ctx context.Context, m Message) (string, error) {
	if m.To.TelegramChatID == "" {
		return "", ErrNoAddress
	}
	text := m.Body
	if m.Subject != "" {
		text = "*" + m.Subject + "*\n" + m.Body
	}
	payload, _ := json.Marshal(map[string]string{"chat_id": m.To.TelegramChatID, "text": text, "parse_mode": "Markdown"})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, t.base+"/bot"+t.token+"/sendMessage", bytes.NewReader(payload))
	if err != nil {
		return m.To.TelegramChatID, err
	}
	req.Header.Set("Content-Type", "application/json")
	return m.To.TelegramChatID, doRequest(req)
}

// ── Email over SMTP (free with Gmail app passwords) ──────────

type Email struct {
	Host, User, Password, From string
	Port                       int
}

func (e Email) Name() string { return "email" }

func (e Email) Send(_ context.Context, m Message) (string, error) {
	if m.To.Email == "" {
		return "", ErrNoAddress
	}
	subject := m.Subject
	if subject == "" {
		subject = "תזכורת"
	}
	var msg bytes.Buffer
	fmt.Fprintf(&msg, "From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\n", e.From, m.To.Email, mime.BEncoding.Encode("UTF-8", subject))
	msg.WriteString("Content-Type: text/html; charset=UTF-8\r\n\r\n")
	fmt.Fprintf(&msg, `<div dir="rtl" style="font-family:Arial,sans-serif">%s</div>`, strings.ReplaceAll(htmlEscape(m.Body), "\n", "<br>"))

	var auth smtp.Auth
	if e.User != "" {
		auth = smtp.PlainAuth("", e.User, e.Password, e.Host)
	}
	return m.To.Email, smtp.SendMail(e.Host+":"+strconv.Itoa(e.Port), auth, e.From, []string{m.To.Email}, msg.Bytes())
}

func htmlEscape(s string) string {
	return strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;", `"`, "&quot;").Replace(s)
}

// ── Twilio SMS (paid) ────────────────────────────────────────

type Twilio struct{ sid, token, from, base string }

func NewTwilio(sid, token, from string) *Twilio {
	return &Twilio{sid: sid, token: token, from: from, base: "https://api.twilio.com"}
}

func (t *Twilio) Name() string { return "twilio" }

func (t *Twilio) Send(ctx context.Context, m Message) (string, error) {
	to := NormalizeIsraeliPhone(m.To.Phone)
	if to == "" {
		return "", ErrNoAddress
	}
	form := url.Values{"To": {to}, "From": {t.from}, "Body": {m.Body}}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		t.base+"/2010-04-01/Accounts/"+t.sid+"/Messages.json", strings.NewReader(form.Encode()))
	if err != nil {
		return to, err
	}
	req.SetBasicAuth(t.sid, t.token)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	return to, doRequest(req)
}

// ── 019 SMS (paid, Israeli provider) ─────────────────────────
// JSON API as documented by 019 (https://docs.019sms.co.il). Verify the
// payload against your account's API version before going live.

type SMS019 struct{ username, token, source, base string }

func NewSMS019(username, token, source string) *SMS019 {
	return &SMS019{username: username, token: token, source: source, base: "https://019sms.co.il/api"}
}

func (s *SMS019) Name() string { return "sms019" }

func (s *SMS019) Send(ctx context.Context, m Message) (string, error) {
	phone := NormalizeIsraeliPhone(m.To.Phone)
	if phone == "" {
		return "", ErrNoAddress
	}
	local := "0" + strings.TrimPrefix(phone, "+972")
	payload, _ := json.Marshal(map[string]any{
		"sms": map[string]any{
			"user":         map[string]string{"username": s.username},
			"source":       s.source,
			"destinations": map[string]any{"phone": []map[string]string{{"_": local}}},
			"message":      m.Body,
		},
	})
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base, bytes.NewReader(payload))
	if err != nil {
		return local, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+s.token)
	return local, doRequest(req)
}
