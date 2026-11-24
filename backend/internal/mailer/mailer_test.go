package mailer_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/mailer"
	"github.com/na06081997-star/CRM_Project/backend/internal/mailer/mailtest"
)

func newSender(t *testing.T, srv *mailtest.Server) *mailer.SMTP {
	t.Helper()
	s, err := mailer.NewSMTP(mailer.Config{Host: "127.0.0.1", Port: srv.Port(), TLS: mailer.TLSNone, From: "קהילה+ <noreply@example.com>", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestSendCapturesTextAndHTML(t *testing.T) {
	srv := mailtest.Start(t)
	s := newSender(t, srv)
	b := mailer.Brand{AppName: "קהילה+", OrgName: "מינהל קהילתי"}
	m := mailer.Code(b, "dana@example.com", "דנה", "012345", 10*time.Minute)
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatal(err)
	}
	got := srv.Last(t, "dana@example.com")
	if !strings.Contains(got.Text, "012345") || !strings.Contains(got.HTML, "012345") {
		t.Fatalf("code missing from parts: %q", got.Text)
	}
	if !strings.Contains(got.Subject, "קוד האימות") || !strings.Contains(got.Text, "10 דקות") {
		t.Fatalf("subject / validity: %q %q", got.Subject, got.Text)
	}
	if !strings.Contains(got.HTML, `dir="rtl"`) || !strings.Contains(got.HTML, "viewport") {
		t.Fatal("html must be RTL and mobile friendly")
	}
}

func TestRefusalsAreClassified(t *testing.T) {
	srv := mailtest.Start(t)
	s := newSender(t, srv)
	m := mailer.Mail{To: "x@example.com", Subject: "s", Text: "t"}

	srv.Reject(550, "5.1.1 mailbox unavailable")
	err := s.Send(context.Background(), m)
	if err == nil || mailer.IsTemporary(err) {
		t.Fatalf("550 must be a permanent failure, got %v", err)
	}
	srv.Reject(451, "4.3.0 try again later")
	if err := s.Send(context.Background(), m); !mailer.IsTemporary(err) {
		t.Fatalf("451 must be temporary, got %v", err)
	}
	srv.Reject(0, "")
	if err := s.Send(context.Background(), m); err != nil {
		t.Fatalf("delivery after recovery: %v", err)
	}
	if srv.Count("x@example.com") != 1 {
		t.Fatalf("exactly one message must be accepted, got %d", srv.Count("x@example.com"))
	}
}

func TestUnreachableServerIsTemporary(t *testing.T) {
	s, _ := mailer.NewSMTP(mailer.Config{Host: "127.0.0.1", Port: 1, TLS: mailer.TLSNone, From: "a@b.c", Timeout: 2 * time.Second})
	if err := s.Send(context.Background(), mailer.Mail{To: "x@example.com", Subject: "s", Text: "t"}); !mailer.IsTemporary(err) {
		t.Fatalf("connection refused must be temporary, got %v", err)
	}
}

func TestDuration(t *testing.T) {
	for d, want := range map[time.Duration]string{24 * time.Hour: "24 שעות", 10 * time.Minute: "10 דקות", time.Hour: "שעה אחת", 72 * time.Hour: "3 ימים"} {
		if got := mailer.Duration(d); got != want {
			t.Errorf("%v: %q, want %q", d, got, want)
		}
	}
}
