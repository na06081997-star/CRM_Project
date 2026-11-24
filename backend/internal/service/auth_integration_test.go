//go:build integration

package service

import (
	"context"
	"errors"
	"os"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/db"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/mailer"
	"github.com/na06081997-star/CRM_Project/backend/internal/mailer/mailtest"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

const testPublicURL = "https://crm.kehila-test.co.il"

type authEnv struct {
	svc   *Service
	st    *store.Store
	smtp  *mailtest.Server
	clock *testClock
	ip    RequestInfo
}

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *testClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// setupAuth builds a password-mode service over a fresh schema and a
// capturing SMTP server. seed adds the demo data (with its super admin).
func setupAuth(t *testing.T, seed bool, bootstrapEmail string) *authEnv {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
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
	cfg := &config.Config{Location: loc, ReminderLead: time.Hour, AppName: "קהילה+", AuthMode: "password",
		JWTSecret: "test-jwt-secret-0123456789abcdef0123456789", AuthCodeSecret: "test-code-secret-abcdef0123456789abcdef01",
		PublicURL: testPublicURL, BootstrapSuperAdminEmail: bootstrapEmail, Auth: config.DefaultAuthPolicy()}
	files, _ := storage.NewS3("", "", "", "", "")
	st := store.New(pool)
	svc := New(cfg, st, nil, &recNotifier{}, files)
	srv := mailtest.Start(t)
	m, err := mailer.NewSMTP(mailer.Config{Host: "127.0.0.1", Port: srv.Port(), TLS: mailer.TLSNone, From: "noreply@kehilaplus.com", Timeout: 5 * time.Second})
	if err != nil {
		t.Fatal(err)
	}
	svc.SetMailer(m)
	clock := &testClock{t: time.Now()}
	svc.now = clock.now
	if seed {
		if err := svc.SeedDemo(ctx); err != nil {
			t.Fatal(err)
		}
	}
	return &authEnv{svc: svc, st: st, smtp: srv, clock: clock, ip: RequestInfo{IP: "203.0.113.7"}}
}

var (
	linkRe = regexp.MustCompile(regexp.QuoteMeta(testPublicURL) + `/(activate|reset)#([A-Za-z0-9_-]{43})`)
	codeRe = regexp.MustCompile(`(?m)^\s*(\d{6})\s*$`)
)

// linkFrom follows the real link of the last e-mail sent to addr.
func (e *authEnv) linkFrom(t *testing.T, addr, kind string) string {
	t.Helper()
	m := e.smtp.Last(t, addr)
	got := linkRe.FindStringSubmatch(m.Text)
	if got == nil || got[1] != kind {
		t.Fatalf("no %s link in e-mail to %s: %q", kind, addr, m.Text)
	}
	if !regexp.MustCompile(regexp.QuoteMeta(got[0])).MatchString(m.HTML) {
		t.Fatal("HTML part must carry the same link")
	}
	return got[2]
}

func (e *authEnv) codeFrom(t *testing.T, addr string) string {
	t.Helper()
	m := e.smtp.Last(t, addr)
	got := codeRe.FindStringSubmatch(m.Text)
	if got == nil {
		t.Fatalf("no code in e-mail to %s: %q", addr, m.Text)
	}
	return got[1]
}

func authCode(err error) string {
	var ae *domain.AuthError
	if errors.As(err, &ae) {
		return ae.Code
	}
	if err == nil {
		return "nil"
	}
	return err.Error()
}

func (e *authEnv) superAdmin(t *testing.T) Actor {
	t.Helper()
	return actor(t, e.st, "matanya@matnas.local")
}

const goodPassword = "Kehila-2026-plus"

// invite creates an account through the admin flow and returns its link token.
func (e *authEnv) invite(t *testing.T, username, email string, role domain.Role) (domain.User, string) {
	t.Helper()
	u, err := e.svc.InviteUser(context.Background(), e.superAdmin(t), InviteInput{Username: username, Email: email, FullName: "משתמש בדיקה", Role: role})
	if err != nil {
		t.Fatal(err)
	}
	if u.Status != domain.AccountPending || u.Invitation == nil || u.Invitation.Status != "sent" {
		t.Fatalf("new account must be pending with a sent invitation: %+v", u)
	}
	return u, e.linkFrom(t, email, "activate")
}

// activate sets the password from the link and completes the e-mail code.
func (e *authEnv) activate(t *testing.T, token, email string) SessionResult {
	t.Helper()
	ctx := context.Background()
	ch, err := e.svc.Activate(ctx, e.ip, token, goodPassword, goodPassword)
	if err != nil || !ch.Sent {
		t.Fatalf("activate: %v %+v", err, ch)
	}
	s, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, e.codeFrom(t, email))
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	return s
}

func (e *authEnv) resolve(token string) (domain.User, error) {
	id, err := e.svc.Sessions().Verify(context.Background(), token)
	if err != nil {
		return domain.User{}, err
	}
	return e.svc.ResolveIdentity(context.Background(), id)
}

// ── Initial super administrator ──────────────────────────────

func TestBootstrapSuperAdminRunsOnce(t *testing.T) {
	const boss = "boss@kehilaplus.com"
	e := setupAuth(t, false, boss)
	ctx := context.Background()

	if err := e.svc.BootstrapSuperAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	u, err := e.st.UserByEmail(ctx, boss)
	if err != nil || u.Role != domain.RoleSuperAdmin || u.Status != domain.AccountPending || u.HasPassword {
		t.Fatalf("pending super admin expected: %+v %v", u, err)
	}
	if e.smtp.Count(boss) != 1 {
		t.Fatalf("one invitation expected, got %d", e.smtp.Count(boss))
	}
	link := e.linkFrom(t, boss, "activate")

	// Re-running the seed: no duplicate, no new invitation.
	if err := e.svc.BootstrapSuperAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.st.CountRole(ctx, domain.RoleSuperAdmin); n != 1 || e.smtp.Count(boss) != 1 {
		t.Fatalf("rerun must change nothing: %d admins, %d mails", n, e.smtp.Count(boss))
	}

	// Activation: password, then the code, then the first session.
	sess := e.activate(t, link, boss)
	if sess.User.Status != domain.AccountActive || sess.User.FirstLoginAt == nil {
		t.Fatalf("first session must activate the account: %+v", sess.User)
	}
	hash, _ := e.st.PasswordHash(ctx, u.ID)

	// Rerun after activation: password untouched, nothing sent.
	if err := e.svc.BootstrapSuperAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	if h, _ := e.st.PasswordHash(ctx, u.ID); h != hash || e.smtp.Count(boss) != 2 { // invitation + code
		t.Fatalf("rerun must not touch the password nor send mail (mails=%d)", e.smtp.Count(boss))
	}

	// A disabled account is never re-enabled, and a changed variable grants nothing.
	if _, err := e.st.UpdateUser(ctx, u.ID, store.UserInput{Email: u.Email, FullName: u.FullName, Role: u.Role, IsActive: false}); err != nil {
		t.Fatal(err)
	}
	e.svc.cfg.BootstrapSuperAdminEmail = "intruder@kehilaplus.com"
	if err := e.svc.BootstrapSuperAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	if got, _ := e.st.UserByID(ctx, u.ID); got.IsActive {
		t.Fatal("bootstrap re-enabled a disabled account")
	}
	if _, err := e.st.UserByEmail(ctx, "intruder@kehilaplus.com"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatal("changing the variable must not create another super admin")
	}
	if e.smtp.Count("intruder@kehilaplus.com") != 0 {
		t.Fatal("no mail to the new address")
	}
	if _, _, err := e.svc.ResendBootstrapInvitation(ctx); err == nil {
		t.Fatal("resend must refuse a disabled / activated initial account")
	}
}

func TestBootstrapResendAfterSMTPRefusal(t *testing.T) {
	const boss = "boss@kehilaplus.com"
	e := setupAuth(t, false, boss)
	ctx := context.Background()

	e.smtp.Reject(451, "4.7.1 try again later")
	if err := e.svc.BootstrapSuperAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	if e.smtp.Count(boss) != 0 {
		t.Fatal("the refused invitation must not be captured")
	}
	u, _ := e.st.UserByEmail(ctx, boss)
	tok, err := e.st.LatestToken(ctx, u.ID, purposeInvite)
	if err != nil || tok.SentAt != nil || tok.DeliveryError == nil {
		t.Fatalf("refusal must be recorded on the invitation: %+v %v", tok, err)
	}

	// The documented operator command, once SMTP works again.
	e.smtp.Reject(0, "")
	if _, inv, err := e.svc.ResendBootstrapInvitation(ctx); err != nil || inv.Status != "sent" {
		t.Fatalf("resend: %v %+v", err, inv)
	}
	sess := e.activate(t, e.linkFrom(t, boss, "activate"), boss)
	if sess.User.Role != domain.RoleSuperAdmin {
		t.Fatal("role must be kept")
	}
	if _, _, err := e.svc.ResendBootstrapInvitation(ctx); err == nil {
		t.Fatal("no resend once activated")
	}
}

func TestBootstrapSkipsWhenASuperAdminExists(t *testing.T) {
	e := setupAuth(t, true, "boss@kehilaplus.com") // demo seed already has one
	ctx := context.Background()
	if err := e.svc.BootstrapSuperAdmin(ctx); err != nil {
		t.Fatal(err)
	}
	if n, _ := e.st.CountRole(ctx, domain.RoleSuperAdmin); n != 1 || len(e.smtp.Messages()) != 0 {
		t.Fatalf("existing instance: nothing to create (admins=%d mails=%d)", n, len(e.smtp.Messages()))
	}
}

// ── Invitations and logins ───────────────────────────────────

func TestInvitationFirstLoginAndNextLogins(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	const email = "dana.k@kehila-test.co.il"
	u, link := e.invite(t, "dana.k", email, domain.RoleCoordinator)

	// A mail scanner opening the link does not use it up.
	for i := 0; i < 2; i++ {
		info, err := e.svc.CheckLink(ctx, purposeInvite, link)
		if err != nil || info.Status != "valid" || info.Username != "dana.k" {
			t.Fatalf("check %d: %+v %v", i, info, err)
		}
	}
	// A weak password is refused and the link stays valid.
	if _, err := e.svc.Activate(ctx, e.ip, link, "short1", "short1"); err == nil {
		t.Fatal("weak password accepted")
	}
	if _, err := e.svc.Activate(ctx, e.ip, link, goodPassword, goodPassword+"x"); err == nil {
		t.Fatal("mismatched confirmation accepted")
	}
	ch, err := e.svc.Activate(ctx, e.ip, link, goodPassword, goodPassword)
	if err != nil || !ch.Sent || ch.Flow != flowActivation || ch.Email == email {
		t.Fatalf("activate: %+v %v", ch, err)
	}
	// No session before the code.
	if got, _ := e.st.UserByID(ctx, u.ID); got.Status != domain.AccountPending {
		t.Fatal("account must stay pending until the code is verified")
	}
	code := e.codeFrom(t, email)
	bad := "000000"
	if code == bad {
		bad = "111111"
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, bad); authCode(err) != "wrong_code" {
		t.Fatalf("wrong code: %v", err)
	}
	sess, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, code)
	if err != nil {
		t.Fatal(err)
	}
	if got, err := e.resolve(sess.Token); err != nil || got.ID != u.ID || got.Role != domain.RoleCoordinator {
		t.Fatalf("session: %+v %v", got, err)
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, code); authCode(err) != "challenge_expired" {
		t.Fatalf("a used code must not open a second session: %v", err)
	}
	if _, err := e.svc.Activate(ctx, e.ip, link, goodPassword, goodPassword); authCode(err) != "link_used" {
		t.Fatalf("link reuse: %v", err)
	}

	// Admin view: invitation used, first and last login recorded.
	users, err := e.svc.ListUsers(ctx, e.superAdmin(t), store.UserFilter{})
	if err != nil {
		t.Fatal(err)
	}
	var row domain.User
	for _, x := range users {
		if x.ID == u.ID {
			row = x
		}
	}
	if row.Status != domain.AccountActive || row.Invitation == nil || row.Invitation.Status != "used" || row.FirstLoginAt == nil || !row.LoginHistoryKnown {
		t.Fatalf("admin row: %+v", row)
	}
	first := *row.FirstLoginAt

	// Next login (by e-mail this time): password, then a new code.
	e.clock.add(time.Hour)
	if _, err := e.svc.Login(ctx, e.ip, "dana.k", "wrong-password-1"); authCode(err) != "invalid_credentials" {
		t.Fatalf("wrong password: %v", err)
	}
	if _, err := e.svc.Login(ctx, e.ip, "nobody.here", "wrong-password-1"); authCode(err) != "invalid_credentials" {
		t.Fatalf("unknown account must look the same: %v", err)
	}
	ch2, err := e.svc.Login(ctx, e.ip, "DANA.K@kehila-test.co.il", goodPassword)
	if err != nil || ch2.Flow != flowLogin {
		t.Fatalf("login by e-mail: %+v %v", ch2, err)
	}
	if e.codeFrom(t, email) == code {
		t.Log("same digits by chance – allowed, the code is bound to its challenge")
	}
	sess2, err := e.svc.VerifyCode(ctx, e.ip, ch2.Challenge, e.codeFrom(t, email))
	if err != nil {
		t.Fatal(err)
	}
	if !sess2.User.FirstLoginAt.Equal(first) || !sess2.User.LastLoginAt.After(first) {
		t.Fatalf("first login must stay, last login must move: %+v", sess2.User)
	}
}

func TestActivationResumesAfterClosingThePage(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	const email = "yael@kehila-test.co.il"
	u, link := e.invite(t, "yael", email, domain.RoleStaff)
	if _, err := e.svc.Activate(ctx, e.ip, link, goodPassword, goodPassword); err != nil {
		t.Fatal(err)
	}
	// Page closed before typing the code: a normal login finishes the activation.
	ch, err := e.svc.Login(ctx, e.ip, "yael", goodPassword)
	if err != nil || ch.Flow != flowActivation {
		t.Fatalf("login of a pending account: %+v %v", ch, err)
	}
	sess, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, e.codeFrom(t, email))
	if err != nil || sess.User.ID != u.ID || sess.User.Status != domain.AccountActive {
		t.Fatalf("resume: %+v %v", sess.User, err)
	}
}

func TestResendInvalidatesPreviousLinkAndCode(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	admin := e.superAdmin(t)
	const email = "omer@kehila-test.co.il"
	u, oldLink := e.invite(t, "omer", email, domain.RoleUser)

	if _, err := e.svc.ResendInvitation(ctx, admin, u.ID); authCode(err) != "resend_too_soon" {
		t.Fatalf("cooldown: %v", err)
	}
	e.clock.add(61 * time.Second)
	if _, err := e.svc.ResendInvitation(ctx, admin, u.ID); err != nil {
		t.Fatal(err)
	}
	newLink := e.linkFrom(t, email, "activate")
	if info, _ := e.svc.CheckLink(ctx, purposeInvite, oldLink); info.Status != "replaced" {
		t.Fatalf("old link must be replaced: %+v", info)
	}
	if _, err := e.svc.Activate(ctx, e.ip, oldLink, goodPassword, goodPassword); authCode(err) != "link_replaced" {
		t.Fatalf("old link accepted: %v", err)
	}
	ch, err := e.svc.Activate(ctx, e.ip, newLink, goodPassword, goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	oldCode := e.codeFrom(t, email)
	if _, err := e.svc.ResendCode(ctx, e.ip, ch.Challenge); authCode(err) != "resend_too_soon" {
		t.Fatalf("code cooldown: %v", err)
	}
	e.clock.add(61 * time.Second)
	if _, err := e.svc.ResendCode(ctx, e.ip, ch.Challenge); err != nil {
		t.Fatal(err)
	}
	newCode := e.codeFrom(t, email)
	if oldCode != newCode {
		if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, oldCode); authCode(err) != "wrong_code" {
			t.Fatalf("old code accepted after resend: %v", err)
		}
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, newCode); err != nil {
		t.Fatalf("new code: %v", err)
	}
}

func TestInvitationResendAfterExpiry(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	const email = "noa@kehila-test.co.il"
	u, link := e.invite(t, "noa", email, domain.RoleCoordinator)
	e.clock.add(25 * time.Hour)
	if info, _ := e.svc.CheckLink(ctx, purposeInvite, link); info.Status != "expired" {
		t.Fatalf("expired link: %+v", info)
	}
	if _, err := e.svc.Activate(ctx, e.ip, link, goodPassword, goodPassword); authCode(err) != "link_expired" {
		t.Fatalf("expired link accepted: %v", err)
	}
	users, _ := e.svc.ListUsers(ctx, e.superAdmin(t), store.UserFilter{})
	for _, x := range users {
		if x.ID == u.ID && (x.Invitation == nil || x.Invitation.Status != "expired") {
			t.Fatalf("admin must see the expiry: %+v", x.Invitation)
		}
	}
	if _, err := e.svc.ResendInvitation(ctx, e.superAdmin(t), u.ID); err != nil {
		t.Fatal(err)
	}
	e.activate(t, e.linkFrom(t, email, "activate"), email)
}

func TestCodeAttemptsAndRateLimits(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	const email = "ran@kehila-test.co.il"
	_, link := e.invite(t, "ran", email, domain.RoleUser)
	e.activate(t, link, email)

	wrong := func(code string) string {
		if code == "999999" {
			return "999998"
		}
		return "999999"
	}
	ch, err := e.svc.Login(ctx, e.ip, "ran", goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	code := e.codeFrom(t, email)
	for i := 1; i <= 5; i++ {
		_, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, wrong(code))
		want := "wrong_code"
		if i == 5 {
			want = "code_locked"
		}
		if authCode(err) != want {
			t.Fatalf("attempt %d: %v, want %s", i, err, want)
		}
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, code); authCode(err) != "code_locked" {
		t.Fatalf("the right code after 5 failures must be refused: %v", err)
	}
	// A new code gets 5 new tries, but the account-wide window still counts.
	e.clock.add(61 * time.Second)
	if _, err := e.svc.ResendCode(ctx, e.ip, ch.Challenge); err != nil {
		t.Fatal(err)
	}
	code = e.codeFrom(t, email)
	for i := 0; i < 5; i++ {
		e.svc.VerifyCode(ctx, e.ip, ch.Challenge, wrong(code)) //nolint:errcheck
	}
	e.clock.add(61 * time.Second)
	if _, err := e.svc.ResendCode(ctx, e.ip, ch.Challenge); err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, e.codeFrom(t, email)); authCode(err) != "rate_limited" {
		t.Fatalf("10 failures in the window must block the account, got %v", err)
	}
	// After the window, the account works again.
	e.clock.add(16 * time.Minute)
	ch, err = e.svc.Login(ctx, e.ip, "ran", goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, e.codeFrom(t, email)); err != nil {
		t.Fatalf("after the window: %v", err)
	}

	// Password guessing: blocked after 10 failures, even with the right password.
	for i := 0; i < 10; i++ {
		e.svc.Login(ctx, RequestInfo{IP: "198.51.100.9"}, "ran", "Not-the-password-1") //nolint:errcheck
	}
	if _, err := e.svc.Login(ctx, e.ip, "ran", goodPassword); authCode(err) != "rate_limited" {
		t.Fatalf("login must be blocked per account: %v", err)
	}
}

func TestConcurrentConsumptionIsSingleUse(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	const email = "tal@kehila-test.co.il"
	_, link := e.invite(t, "tal", email, domain.RoleUser)

	var wg sync.WaitGroup
	results := make([]error, 2)
	for i := range results {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, results[i] = e.svc.Activate(ctx, e.ip, link, goodPassword, goodPassword)
		}(i)
	}
	wg.Wait()
	if (results[0] == nil) == (results[1] == nil) {
		t.Fatalf("exactly one activation must win: %v / %v", results[0], results[1])
	}

	ch, err := e.svc.Login(ctx, e.ip, "tal", goodPassword)
	if err != nil {
		t.Fatal(err)
	}
	code := e.codeFrom(t, email)
	sessions := make([]error, 2)
	for i := range sessions {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, sessions[i] = e.svc.VerifyCode(ctx, e.ip, ch.Challenge, code)
		}(i)
	}
	wg.Wait()
	if (sessions[0] == nil) == (sessions[1] == nil) {
		t.Fatalf("exactly one session must be issued: %v / %v", sessions[0], sessions[1])
	}
}

func TestForgotPasswordRevokesSessions(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	const email = "gil@kehila-test.co.il"
	_, link := e.invite(t, "gil", email, domain.RoleCoordinator)
	old := e.activate(t, link, email)
	before := e.smtp.Count(email)

	// Same answer whether or not the account exists.
	if err := e.svc.ForgotPassword(ctx, e.ip, "no-such-user"); err != nil {
		t.Fatal(err)
	}
	if err := e.svc.ForgotPassword(ctx, e.ip, "gil"); err != nil {
		t.Fatal(err)
	}
	e.svc.WaitBackground()
	if e.smtp.Count(email) != before+1 {
		t.Fatalf("one reset e-mail expected, got %d", e.smtp.Count(email)-before)
	}
	reset := e.linkFrom(t, email, "reset")
	if info, _ := e.svc.CheckLink(ctx, purposeReset, reset); info.Status != "valid" {
		t.Fatalf("reset link: %+v", info)
	}
	const newPassword = "Another-pass-77"
	if err := e.svc.ResetPassword(ctx, e.ip, reset, newPassword, newPassword); err != nil {
		t.Fatal(err)
	}
	if _, err := e.resolve(old.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("sessions opened before the reset must be revoked: %v", err)
	}
	if err := e.svc.ResetPassword(ctx, e.ip, reset, newPassword, newPassword); authCode(err) != "link_used" {
		t.Fatalf("reset link reuse: %v", err)
	}
	if _, err := e.svc.Login(ctx, e.ip, "gil", goodPassword); authCode(err) != "invalid_credentials" {
		t.Fatalf("old password: %v", err)
	}
	ch, err := e.svc.Login(ctx, e.ip, "gil", newPassword)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, e.codeFrom(t, email)); err != nil {
		t.Fatalf("login after reset needs and accepts the code: %v", err)
	}
}

func TestSMTPRefusalIsRecoverable(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	admin := e.superAdmin(t)
	const email = "maya.b@kehila-test.co.il"

	e.smtp.Reject(550, "5.1.1 mailbox unavailable")
	u, err := e.svc.InviteUser(ctx, admin, InviteInput{Username: "maya.b", Email: email, Role: domain.RoleCoordinator})
	if err != nil {
		t.Fatal(err)
	}
	if u.Invitation == nil || u.Invitation.Status != "failed" || u.Invitation.DeliveryError == nil {
		t.Fatalf("refusal must show on the invitation: %+v", u.Invitation)
	}
	e.smtp.Reject(0, "")
	e.clock.add(61 * time.Second)
	if u, err = e.svc.ResendInvitation(ctx, admin, u.ID); err != nil || u.Invitation.Status != "sent" {
		t.Fatalf("resend after recovery: %v %+v", err, u.Invitation)
	}
	link := e.linkFrom(t, email, "activate")

	// A refused code: the page offers an immediate resend.
	e.smtp.Reject(451, "4.3.0 temporary failure")
	ch, err := e.svc.Activate(ctx, e.ip, link, goodPassword, goodPassword)
	if err != nil || ch.Sent {
		t.Fatalf("activate with SMTP down: %+v %v", ch, err)
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, "123456"); authCode(err) != "no_code" {
		t.Fatalf("unsent code: %v", err)
	}
	e.smtp.Reject(0, "")
	again, err := e.svc.ResendCode(ctx, e.ip, ch.Challenge)
	if err != nil || !again.Sent {
		t.Fatalf("immediate resend after failure: %+v %v", again, err)
	}
	if _, err := e.svc.VerifyCode(ctx, e.ip, ch.Challenge, e.codeFrom(t, email)); err != nil {
		t.Fatal(err)
	}
}

func TestInviteValidationAndRoles(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	admin := e.superAdmin(t)
	cases := []InviteInput{
		{Username: "x", Email: "a@kehila-test.co.il", Role: domain.RoleUser},             // too short
		{Username: "shiri", Email: "new@kehila-test.co.il", Role: domain.RoleUser},       // username taken
		{Username: "newone", Email: "SHIRI@matnas.local", Role: domain.RoleUser},         // e-mail taken (any case)
		{Username: "newtwo", Email: "not-an-email", Role: domain.RoleUser},               // bad e-mail
		{Username: "newthree", Email: "n3@kehila-test.co.il", Role: domain.Role("boss")}, // bad role
	}
	for i, in := range cases {
		if _, err := e.svc.InviteUser(ctx, admin, in); err == nil {
			t.Fatalf("case %d accepted: %+v", i, in)
		}
	}
	// מנהל/ת מינהל keeps her business rights but does not manage users.
	if _, err := e.svc.InviteUser(ctx, actor(t, e.st, "shiri@matnas.local"), InviteInput{Username: "abc", Email: "abc@kehila-test.co.il", Role: domain.RoleUser}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("admin must not invite: %v", err)
	}
}

func TestRecoverSuperAdmin(t *testing.T) {
	e := setupAuth(t, true, "")
	ctx := context.Background()
	const email = "chief@kehila-test.co.il"
	u, link := e.invite(t, "chief", email, domain.RoleSuperAdmin)
	old := e.activate(t, link, email)

	const fresh = "chief.new@kehila-test.co.il"
	got, inv, err := e.svc.RecoverSuperAdmin(ctx, "chief", fresh)
	if err != nil || inv.Status != "sent" || got.Email != fresh || got.HasPassword {
		t.Fatalf("recover: %+v %+v %v", got, inv, err)
	}
	if _, err := e.resolve(old.Token); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatal("old sessions must be revoked")
	}
	if _, err := e.svc.Login(ctx, e.ip, "chief", goodPassword); authCode(err) != "invalid_credentials" {
		t.Fatalf("old password must be gone: %v", err)
	}
	sess := e.activate(t, e.linkFrom(t, fresh, "activate"), fresh)
	if sess.User.ID != u.ID || sess.User.Role != domain.RoleSuperAdmin {
		t.Fatalf("same account, same role: %+v", sess.User)
	}
	if _, _, err := e.svc.RecoverSuperAdmin(ctx, "shiri", "x@kehila-test.co.il"); err == nil {
		t.Fatal("recovery is for super administrators only")
	}
}

func TestDemoLoginRefusedInPasswordMode(t *testing.T) {
	e := setupAuth(t, true, "")
	u := e.superAdmin(t).User
	if _, err := e.svc.DemoLogin(u); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("demo login must be off: %v", err)
	}
	// A token without the right version (e.g. forged after a revocation) is refused.
	tok, _, _ := e.svc.Sessions().Issue(u.ID, u.Email, u.TokenVersion+1)
	if _, err := e.resolve(tok); !errors.Is(err, domain.ErrUnauthorized) {
		t.Fatalf("stale version accepted: %v", err)
	}
}
