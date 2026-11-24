//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/api"
	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/db"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/mailer"
	"github.com/na06081997-star/CRM_Project/backend/internal/mailer/mailtest"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type syncBuf struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuf) Write(p []byte) (int, error) { s.mu.Lock(); defer s.mu.Unlock(); return s.b.Write(p) }
func (s *syncBuf) String() string              { s.mu.Lock(); defer s.mu.Unlock(); return s.b.String() }

func call(t *testing.T, srv *httptest.Server, method, path, bearer string, body any) (int, map[string]any, http.Header) {
	t.Helper()
	var rd *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = bytes.NewReader(b)
	} else {
		rd = bytes.NewReader(nil)
	}
	req, _ := http.NewRequest(method, srv.URL+path, rd)
	req.Header.Set("Content-Type", "application/json")
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	out := map[string]any{}
	json.NewDecoder(res.Body).Decode(&out) //nolint:errcheck
	return res.StatusCode, out, res.Header
}

// The whole HTTP flow: no protected route opens before the e-mail code, the
// demo routes do not exist, and nothing secret reaches the logs.
func TestHTTPLoginFlow(t *testing.T) {
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	logs := &syncBuf{}
	prev := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(logs, &slog.HandlerOptions{Level: slog.LevelDebug})))
	t.Cleanup(func() { slog.SetDefault(prev) })

	ctx := context.Background()
	pool, err := db.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Jerusalem")
	cfg := &config.Config{Location: loc, AppName: "קהילה+", AuthMode: "password", JWTSecret: "api-test-jwt-0123456789abcdef0123456789",
		AuthCodeSecret: "api-test-code-0123456789abcdef0123456789", PublicURL: "https://crm.kehila-test.co.il",
		BootstrapSuperAdminEmail: "boss@kehila-test.co.il", Auth: config.DefaultAuthPolicy()}
	files, _ := storage.NewS3("", "", "", "", "")
	st := store.New(pool)
	svc := service.New(cfg, st, nil, nil, files)
	smtp := mailtest.Start(t)
	m, _ := mailer.NewSMTP(mailer.Config{Host: "127.0.0.1", Port: smtp.Port(), TLS: mailer.TLSNone, From: "noreply@kehilaplus.com"})
	svc.SetMailer(m)
	if err := svc.BootstrapSuperAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(api.NewServer(cfg, svc, realtime.NewHub(nil, nil), files, "test").Routes())
	t.Cleanup(srv.Close)

	// Old session routes are gone in password mode.
	for _, p := range []string{"/api/auth/dev-users", "/api/auth/dev-login"} {
		if code, _, _ := call(t, srv, http.MethodPost, p, "", map[string]string{"email": "boss@kehila-test.co.il"}); code != http.StatusNotFound && code != http.StatusMethodNotAllowed {
			t.Fatalf("%s must not exist, got %d", p, code)
		}
		if code, _, _ := call(t, srv, http.MethodGet, p, "", nil); code != http.StatusNotFound && code != http.StatusMethodNotAllowed {
			t.Fatalf("GET %s must not exist, got %d", p, code)
		}
	}

	mail := smtp.Last(t, "boss@kehila-test.co.il")
	token := regexp.MustCompile(`/activate#([A-Za-z0-9_-]{43})`).FindStringSubmatch(mail.Text)[1]

	code, link, hdr := call(t, srv, http.MethodPost, "/api/auth/link", "", map[string]string{"purpose": "invite", "token": token})
	if code != 200 || link["status"] != "valid" || hdr.Get("Cache-Control") != "no-store" {
		t.Fatalf("link check: %d %v %v", code, link, hdr.Get("Cache-Control"))
	}
	const pw = "Strong-pass-2026"
	code, ch, _ := call(t, srv, http.MethodPost, "/api/auth/activate", "", map[string]string{"token": token, "password": pw, "passwordConfirm": pw})
	if code != 200 || ch["sent"] != true {
		t.Fatalf("activate: %d %v", code, ch)
	}
	if _, has := ch["token"]; has {
		t.Fatal("no session before the code")
	}
	handle := ch["challenge"].(string)

	// The intermediate state opens nothing.
	for _, p := range []string{"/api/me", "/api/users", "/api/bookings", "/api/dashboard"} {
		if code, _, _ := call(t, srv, http.MethodGet, p, handle, nil); code != http.StatusUnauthorized {
			t.Fatalf("%s with the challenge handle: %d", p, code)
		}
		if code, _, _ := call(t, srv, http.MethodGet, p, "", nil); code != http.StatusUnauthorized {
			t.Fatalf("%s without token: %d", p, code)
		}
	}
	if code, _, _ := call(t, srv, http.MethodGet, "/ws?token="+handle, "", nil); code != http.StatusUnauthorized {
		t.Fatalf("websocket with the challenge handle: %d", code)
	}

	codeMail := smtp.Last(t, "boss@kehila-test.co.il")
	otp := regexp.MustCompile(`(?m)^\s*(\d{6})\s*$`).FindStringSubmatch(codeMail.Text)[1]
	code, wrong, _ := call(t, srv, http.MethodPost, "/api/auth/verify", "", map[string]string{"challenge": handle, "code": "12345x"})
	if code != http.StatusUnprocessableEntity {
		t.Fatalf("malformed code: %d %v", code, wrong)
	}
	code, sess, _ := call(t, srv, http.MethodPost, "/api/auth/verify", "", map[string]string{"challenge": handle, "code": otp})
	if code != 200 || sess["token"] == nil {
		t.Fatalf("verify: %d %v", code, sess)
	}
	session := sess["token"].(string)
	code, me, _ := call(t, srv, http.MethodGet, "/api/me", session, nil)
	if code != 200 || me["user"].(map[string]any)["role"] != string(domain.RoleSuperAdmin) {
		t.Fatalf("me: %d %v", code, me)
	}

	// Admin metrics stay reachable for the super administrator.
	if code, _, _ := call(t, srv, http.MethodGet, "/api/dashboard", session, nil); code != 200 {
		t.Fatalf("dashboard: %d", code)
	}
	// Invite through the API and see the tracking columns.
	code, invited, _ := call(t, srv, http.MethodPost, "/api/users/invite", session, map[string]string{
		"username": "rakaz.one", "email": "rakaz.one@kehila-test.co.il", "fullName": "רכז אחד", "role": "coordinator"})
	if code != http.StatusCreated || invited["status"] != "pending" || invited["invitation"].(map[string]any)["status"] != "sent" {
		t.Fatalf("invite: %d %v", code, invited)
	}

	// Forgot password: identical answers.
	_, a, _ := call(t, srv, http.MethodPost, "/api/auth/forgot", "", map[string]string{"identifier": "nobody"})
	_, b, _ := call(t, srv, http.MethodPost, "/api/auth/forgot", "", map[string]string{"identifier": "boss@kehila-test.co.il"})
	if a["message"] != b["message"] {
		t.Fatal("forgot-password answers must not differ")
	}
	svc.WaitBackground()
	resetMail := smtp.Last(t, "boss@kehila-test.co.il")
	resetToken := regexp.MustCompile(`/reset#([A-Za-z0-9_-]{43})`).FindStringSubmatch(resetMail.Text)[1]

	// Secrets never reach the logs.
	out := logs.String()
	for name, secret := range map[string]string{"invitation token": token, "code": otp, "password": pw, "session": session,
		"challenge": handle, "reset token": resetToken} {
		if strings.Contains(out, secret) {
			t.Errorf("%s leaked into the logs", name)
		}
	}
}
