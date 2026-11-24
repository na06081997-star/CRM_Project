package service

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"math/big"
	"net/http"
	"net/mail"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/mailer"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// Login flow (AUTH_MODE=password):
//
//	identifier + password ─▶ challenge (opaque handle, can only verify / resend)
//	                       ─▶ 6-digit code by e-mail ─▶ session token
//
// Invitation and "forgot password" links carry a random token whose SHA-256
// alone is stored; opening the page does not consume it, submitting does.

const (
	purposeInvite = "invite"
	purposeReset  = "reset"

	flowActivation = "activation"
	flowLogin      = "login"

	evLoginFail = "login_fail"
	evCodeFail  = "code_fail"
	evSend      = "mail_sent"
	evForgot    = "forgot"

	bcryptCost = 12
	stateBoot  = "bootstrap_super_admin"
)

// RequestInfo describes the caller of a public auth endpoint.
type RequestInfo struct{ IP string }

// ChallengeResult is returned once the password is accepted: the browser
// keeps Challenge to verify or resend the code, nothing more.
type ChallengeResult struct {
	Challenge string `json:"challenge"`
	Email     string `json:"email"` // masked
	Sent      bool   `json:"sent"`
	ResendIn  int    `json:"resendIn"` // seconds before "send again"
	CodeTTL   int    `json:"codeTtl"`  // seconds
	Flow      string `json:"flow"`
}

type SessionResult struct {
	Token       string          `json:"token"`
	ExpiresAt   time.Time       `json:"expiresAt"`
	User        domain.User     `json:"user"`
	Permissions []domain.Action `json:"permissions"`
}

// LinkInfo is what an invitation / reset page shows before submitting.
type LinkInfo struct {
	Status    string    `json:"status"` // valid | expired | used | replaced | disabled
	Username  string    `json:"username,omitempty"`
	FullName  string    `json:"fullName,omitempty"`
	Email     string    `json:"email,omitempty"` // masked
	ExpiresAt time.Time `json:"expiresAt"`
}

// ── Errors (public wording never reveals whether an account exists) ──

func authErr(status int, code, msg string) *domain.AuthError {
	return &domain.AuthError{Status: status, Code: code, Message: msg}
}

var (
	errBadCredentials = authErr(http.StatusUnauthorized, "invalid_credentials", "שם המשתמש או הסיסמה שגויים")
	errChallengeGone  = authErr(http.StatusGone, "challenge_expired", "תוקף הכניסה פג – התחברו מחדש")
	errWrongCode      = authErr(http.StatusUnauthorized, "wrong_code", "הקוד שגוי")
	errCodeExpired    = authErr(http.StatusGone, "code_expired", "תוקף הקוד פג – בקשו קוד חדש")
	errCodeLocked     = authErr(http.StatusTooManyRequests, "code_locked", "יותר מדי ניסיונות לקוד זה – בקשו קוד חדש")
	errNoCode         = authErr(http.StatusConflict, "no_code", "הקוד לא נשלח – לחצו על \"שליחה חוזרת\"")
	errMailDisabled   = authErr(http.StatusServiceUnavailable, "mail_disabled", "שליחת אימיילים אינה מוגדרת בשרת")
	errLinkInvalid    = authErr(http.StatusGone, "link_invalid", "הקישור אינו תקין – ודאו שהעתקתם אותו במלואו")
	errLinkExpired    = authErr(http.StatusGone, "link_expired", "תוקף הקישור פג")
	errLinkUsed       = authErr(http.StatusGone, "link_used", "הקישור כבר נוצל – התחברו עם שם המשתמש והסיסמה")
	errLinkReplaced   = authErr(http.StatusGone, "link_replaced", "נשלח קישור חדש יותר – השתמשו בקישור האחרון שקיבלתם")
	errAccountOff     = authErr(http.StatusForbidden, "account_disabled", "החשבון אינו פעיל – פנו למנהל המערכת")
)

func tooMany(retry time.Duration) *domain.AuthError {
	e := authErr(http.StatusTooManyRequests, "rate_limited", "יותר מדי ניסיונות. נסו שוב בעוד כמה דקות.")
	e.RetryAfter = int(retry.Seconds())
	return e
}

// ── Secrets ──────────────────────────────────────────────────

// newSecret returns 32 random bytes (base64url) and their SHA-256.
func newSecret() (string, []byte, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", nil, err
	}
	raw := base64.RawURLEncoding.EncodeToString(b)
	return raw, hashSecret(raw), nil
}

func hashSecret(raw string) []byte {
	h := sha256.Sum256([]byte(raw))
	return h[:]
}

// validSecret rejects anything that is not a token we could have issued.
func validSecret(raw string) bool {
	if len(raw) != 43 {
		return false
	}
	_, err := base64.RawURLEncoding.DecodeString(raw)
	return err == nil
}

// newCode is a uniform 6-digit code, leading zeros included.
func newCode() (string, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(1_000_000))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%06d", n.Int64()), nil
}

// codeMAC binds a code to its account, flow, challenge and generation, so a
// code is worthless anywhere else — and after any resend.
func (s *Service) codeMAC(ch store.Challenge, generation int, code string) []byte {
	m := hmac.New(sha256.New, []byte(s.cfg.AuthCodeSecret))
	fmt.Fprintf(m, "%s|%s|%s|%d|%s", ch.ID, ch.UserID, ch.Flow, generation, code)
	return m.Sum(nil)
}

// ── Rate limits (sliding window) ─────────────────────────────

func (s *Service) over(ctx context.Context, kind, key string, max int) bool {
	if key == "" || max <= 0 {
		return false
	}
	n, err := s.store.CountAuthEvents(ctx, kind, key, s.now().Add(-s.cfg.Auth.Window))
	if err != nil {
		slog.Error("auth: rate limit lookup failed", "err", err)
		return true // fail closed
	}
	return n >= max
}

func (s *Service) hit(ctx context.Context, kind string, keys ...string) {
	for _, k := range keys {
		if k == "" {
			continue
		}
		if err := s.store.AddAuthEvent(ctx, kind, k, s.now()); err != nil {
			slog.Error("auth: rate limit write failed", "err", err)
		}
	}
}

func ipKey(info RequestInfo) string {
	if info.IP == "" {
		return ""
	}
	return "ip:" + info.IP
}

func userKey(id uuid.UUID) string { return "u:" + id.String() }

func identKey(identifier string) string {
	h := sha256.Sum256([]byte(strings.ToLower(strings.TrimSpace(identifier))))
	return "id:" + hex.EncodeToString(h[:16])
}

// ── Passwords ────────────────────────────────────────────────

var common = map[string]bool{"1234567890": true, "12345678910": true, "password12": true, "password123": true, "qwerty1234": true,
	"qwertyuiop1": true, "1q2w3e4r5t": true, "abcd123456": true, "a123456789": true, "123456789a": true, "aa12345678": true}

// CheckPasswordPolicy is enforced on the server for every password set.
func CheckPasswordPolicy(pw, confirm string, u domain.User) error {
	v := domain.NewValidation()
	var letter, digit bool
	for _, r := range pw {
		letter = letter || unicode.IsLetter(r)
		digit = digit || unicode.IsDigit(r)
	}
	lower := strings.ToLower(pw)
	local := strings.ToLower(strings.SplitN(u.Email, "@", 2)[0])
	switch {
	case utf8.RuneCountInString(pw) < 10:
		v.Add("password", "הסיסמה צריכה לכלול לפחות 10 תווים")
	case len(pw) > 72:
		v.Add("password", "הסיסמה ארוכה מדי (עד 72 תווים באנגלית, פחות בעברית)")
	case !letter || !digit:
		v.Add("password", "יש לשלב אותיות וספרות")
	case common[lower]:
		v.Add("password", "הסיסמה נפוצה מדי – בחרו סיסמה אחרת")
	case len(u.Username) >= 3 && strings.Contains(lower, strings.ToLower(u.Username)),
		len(local) >= 3 && strings.Contains(lower, local):
		v.Add("password", "הסיסמה לא יכולה לכלול את שם המשתמש או האימייל")
	}
	if pw != confirm {
		v.Add("passwordConfirm", "הסיסמאות אינן זהות")
	}
	return v.OrNil()
}

// dummyHash keeps a failed lookup as slow as a wrong password.
var dummyHash, _ = bcrypt.GenerateFromPassword([]byte("not-a-real-password-0"), bcryptCost)

func passwordMatches(hash, pw string) bool {
	if hash == "" {
		bcrypt.CompareHashAndPassword(dummyHash, []byte(pw)) //nolint:errcheck
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// ── Brand & addresses ────────────────────────────────────────

func (s *Service) brand(ctx context.Context) mailer.Brand {
	org, _ := s.store.Organization(ctx)
	return mailer.Brand{AppName: s.cfg.AppName, OrgName: org.Name}
}

// MailBrand is used by the notification e-mails.
func (s *Service) MailBrand(ctx context.Context) mailer.Brand { return s.brand(ctx) }

func (s *Service) link(path, token string) string {
	return s.cfg.PublicURL + path + "#" + token
}

// MaskEmail shows enough to recognise the address: da***@ex***.com.
func MaskEmail(e string) string {
	at := strings.LastIndex(e, "@")
	if at < 1 {
		return "***"
	}
	local, domain := e[:at], e[at+1:]
	keep := func(s string, n int) string {
		r := []rune(s)
		if len(r) <= n {
			return string(r[:1]) + "***"
		}
		return string(r[:n]) + "***"
	}
	dot := strings.LastIndex(domain, ".")
	if dot < 1 {
		return keep(local, 2) + "@" + keep(domain, 1)
	}
	return keep(local, 2) + "@" + keep(domain[:dot], 2) + domain[dot:]
}

// UsableEmail tells whether mail can really be delivered to an address.
func UsableEmail(e string) bool {
	a, err := mail.ParseAddress(e)
	if err != nil || a.Address != e {
		return false
	}
	d := strings.ToLower(e[strings.LastIndex(e, "@")+1:])
	return strings.Contains(d, ".") && !strings.HasSuffix(d, ".local") && !strings.HasSuffix(d, ".invalid") &&
		!strings.HasSuffix(d, ".test") && d != "example.com" && d != "example.org"
}

var usernameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9._-]{2,31}$`)

// ValidUsername: 3-32 characters, latin letters, digits, dot, dash, underscore.
func ValidUsername(u string) bool { return usernameRe.MatchString(u) }

// ── Login ────────────────────────────────────────────────────

// lookupLogin finds an account by username, or by e-mail when the
// identifier contains "@". Ambiguous legacy e-mails never match.
func (s *Service) lookupLogin(ctx context.Context, identifier string) (domain.User, bool) {
	var u domain.User
	var err error
	if strings.Contains(identifier, "@") {
		u, err = s.store.UserByEmail(ctx, identifier)
	} else {
		u, err = s.store.UserByUsername(ctx, identifier)
	}
	if err != nil {
		if !errors.Is(err, domain.ErrNotFound) {
			slog.Error("auth: login lookup failed", "err", err)
		}
		return domain.User{}, false
	}
	return u, true
}

// Login checks the password and sends a code; no session yet.
func (s *Service) Login(ctx context.Context, info RequestInfo, identifier, password string) (ChallengeResult, error) {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" || password == "" {
		v := domain.NewValidation()
		if identifier == "" {
			v.Add("identifier", "יש להזין שם משתמש או אימייל")
		}
		if password == "" {
			v.Add("password", "יש להזין סיסמה")
		}
		return ChallengeResult{}, v
	}
	if s.mail == nil {
		return ChallengeResult{}, errMailDisabled
	}
	u, found := s.lookupLogin(ctx, identifier)
	// Same key whether or not the account exists, so the limit says nothing.
	key := identKey(identifier)
	if found {
		key = userKey(u.ID)
	}
	if s.over(ctx, evLoginFail, key, s.cfg.Auth.MaxLoginFailsPerAccount) || s.over(ctx, evLoginFail, ipKey(info), s.cfg.Auth.MaxLoginFailsPerIP) {
		return ChallengeResult{}, tooMany(s.cfg.Auth.Window)
	}
	hash := ""
	if found {
		var err error
		if hash, err = s.store.PasswordHash(ctx, u.ID); err != nil {
			return ChallengeResult{}, err
		}
	}
	if !passwordMatches(hash, password) || !found || !u.IsActive {
		s.hit(ctx, evLoginFail, key, ipKey(info))
		return ChallengeResult{}, errBadCredentials
	}
	flow := flowLogin
	if u.ActivatedAt == nil {
		flow = flowActivation
	}
	return s.startChallenge(ctx, info, u, flow)
}

func (s *Service) startChallenge(ctx context.Context, info RequestInfo, u domain.User, flow string) (ChallengeResult, error) {
	raw, hash, err := newSecret()
	if err != nil {
		return ChallengeResult{}, err
	}
	ch, err := s.store.CreateChallenge(ctx, u.ID, flow, hash, s.now().Add(s.cfg.Auth.ChallengeTTL), s.now())
	if err != nil {
		return ChallengeResult{}, err
	}
	return s.sendCode(ctx, info, u, ch, raw)
}

// sendCode issues a fresh code for the challenge (the previous one dies).
func (s *Service) sendCode(ctx context.Context, info RequestInfo, u domain.User, ch store.Challenge, handle string) (ChallengeResult, error) {
	res := ChallengeResult{Challenge: handle, Email: MaskEmail(u.Email), CodeTTL: int(s.cfg.Auth.CodeTTL.Seconds()), Flow: ch.Flow}
	now := s.now()
	if ch.LastSentAt != nil {
		if wait := ch.LastSentAt.Add(s.cfg.Auth.ResendCooldown).Sub(now); wait > 0 {
			e := authErr(http.StatusTooManyRequests, "resend_too_soon", "אפשר לבקש קוד חדש בעוד מספר שניות")
			e.RetryAfter = int(wait.Seconds()) + 1
			return res, e
		}
	}
	if ch.Sends >= s.cfg.Auth.MaxSendsPerFlow || s.over(ctx, evSend, userKey(u.ID), s.cfg.Auth.MaxSendsPerAccount) ||
		s.over(ctx, evSend, ipKey(info), s.cfg.Auth.MaxSendsPerIP) {
		return res, tooMany(s.cfg.Auth.Window)
	}
	code, err := newCode()
	if err != nil {
		return res, err
	}
	gen := ch.Generation + 1
	ok, err := s.store.SetChallengeCode(ctx, ch.ID, gen, s.codeMAC(ch, gen, code), now.Add(s.cfg.Auth.CodeTTL), now)
	if err != nil {
		return res, err
	}
	if !ok {
		return res, authErr(http.StatusConflict, "challenge_changed", "כבר נשלח קוד חדש – בדקו את תיבת הדואר")
	}
	s.hit(ctx, evSend, userKey(u.ID), ipKey(info))
	if err := s.mail.Send(ctx, mailer.Code(s.brand(ctx), u.Email, u.FullName, code, s.cfg.Auth.CodeTTL)); err != nil {
		slog.Warn("auth: code e-mail not sent", "user", u.ID, "temporary", mailer.IsTemporary(err), "err", err)
		if cerr := s.store.ClearChallengeCode(ctx, ch.ID, gen); cerr != nil {
			slog.Error("auth: clear unsent code", "err", cerr)
		}
		return res, nil // Sent=false: the page offers "send again" right away
	}
	res.Sent, res.ResendIn = true, int(s.cfg.Auth.ResendCooldown.Seconds())
	return res, nil
}

func (s *Service) openChallenge(ctx context.Context, handle string) (store.Challenge, domain.User, error) {
	if !validSecret(handle) {
		return store.Challenge{}, domain.User{}, errChallengeGone
	}
	ch, err := s.store.ChallengeByHandle(ctx, hashSecret(handle))
	if errors.Is(err, domain.ErrNotFound) {
		return ch, domain.User{}, errChallengeGone
	}
	if err != nil {
		return ch, domain.User{}, err
	}
	if ch.ConsumedAt != nil || !s.now().Before(ch.ExpiresAt) {
		return ch, domain.User{}, errChallengeGone
	}
	u, err := s.store.UserByID(ctx, ch.UserID)
	if err != nil {
		return ch, u, err
	}
	if !u.IsActive {
		return ch, u, errAccountOff
	}
	return ch, u, nil
}

// ResendCode sends a new code for an ongoing login.
func (s *Service) ResendCode(ctx context.Context, info RequestInfo, handle string) (ChallengeResult, error) {
	if s.mail == nil {
		return ChallengeResult{}, errMailDisabled
	}
	ch, u, err := s.openChallenge(ctx, handle)
	if err != nil {
		return ChallengeResult{}, err
	}
	return s.sendCode(ctx, info, u, ch, handle)
}

// VerifyCode completes a login: the only way to obtain a session.
func (s *Service) VerifyCode(ctx context.Context, info RequestInfo, handle, code string) (SessionResult, error) {
	code = strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		if unicode.IsSpace(r) || r == '-' {
			return -1
		}
		return 'x'
	}, code)
	if len(code) != 6 || strings.Contains(code, "x") {
		return SessionResult{}, authErr(http.StatusUnprocessableEntity, "invalid_code", "יש להזין את 6 הספרות שקיבלתם באימייל")
	}
	ch, u, err := s.openChallenge(ctx, handle)
	if err != nil {
		return SessionResult{}, err
	}
	if s.over(ctx, evCodeFail, userKey(u.ID), s.cfg.Auth.MaxCodeFailsPerAccount) || s.over(ctx, evCodeFail, ipKey(info), s.cfg.Auth.MaxCodeFailsPerIP) {
		return SessionResult{}, tooMany(s.cfg.Auth.Window)
	}
	mac, gen, ok, err := s.store.ReserveAttempt(ctx, ch.ID, s.cfg.Auth.CodeMaxAttempts, s.now())
	if err != nil {
		return SessionResult{}, err
	}
	if !ok {
		switch {
		case ch.CodeExpiresAt == nil || ch.LastSentAt == nil:
			return SessionResult{}, errNoCode
		case !s.now().Before(*ch.CodeExpiresAt):
			return SessionResult{}, errCodeExpired
		default:
			return SessionResult{}, errCodeLocked
		}
	}
	if !hmac.Equal(mac, s.codeMAC(ch, gen, code)) {
		s.hit(ctx, evCodeFail, userKey(u.ID), ipKey(info))
		e := *errWrongCode
		if left := s.cfg.Auth.CodeMaxAttempts - (ch.Attempts + 1); left > 0 {
			e.Message = fmt.Sprintf("הקוד שגוי (נותרו %d ניסיונות)", left)
		} else {
			e = *errCodeLocked
		}
		return SessionResult{}, &e
	}
	if won, err := s.store.ConsumeChallenge(ctx, ch.ID, s.now()); err != nil {
		return SessionResult{}, err
	} else if !won {
		return SessionResult{}, errChallengeGone
	}
	u, err = s.store.RecordLogin(ctx, u.ID, s.now())
	if err != nil {
		return SessionResult{}, err
	}
	s.audit(ctx, Actor{User: u}, "user", u.ID, "login", map[string]any{"flow": ch.Flow, "ip": info.IP})
	return s.issueSession(u)
}

func (s *Service) issueSession(u domain.User) (SessionResult, error) {
	tok, exp, err := s.sessions.Issue(u.ID, u.Email, u.TokenVersion)
	if err != nil {
		return SessionResult{}, err
	}
	return SessionResult{Token: tok, ExpiresAt: exp, User: u, Permissions: domain.Permissions(u.Role)}, nil
}

// DemoLogin opens a session without password (AUTH_MODE=dev only).
func (s *Service) DemoLogin(u domain.User) (SessionResult, error) {
	if s.cfg.PasswordAuth() {
		return SessionResult{}, domain.ErrForbidden
	}
	return s.issueSession(u)
}

// ── Invitations ──────────────────────────────────────────────

// InviteInput is what the super administrator types.
type InviteInput struct {
	Username  string      `json:"username"`
	Email     string      `json:"email"`
	FullName  string      `json:"fullName"`
	Role      domain.Role `json:"role"`
	Phone     *string     `json:"phone"`
	StaffKind *string     `json:"staffKind"`
}

// assignable lists the roles an actor may give.
func assignable(actor Actor, r domain.Role) bool {
	if !actor.Can(domain.ActManageUsers) || !r.Valid() {
		return false
	}
	return r != domain.RoleSuperAdmin || actor.User.Role == domain.RoleSuperAdmin
}

// InviteUser creates a pending account and e-mails its invitation.
func (s *Service) InviteUser(ctx context.Context, actor Actor, in InviteInput) (domain.User, error) {
	if err := actor.Require(domain.ActManageUsers); err != nil {
		return domain.User{}, err
	}
	if s.mail == nil {
		return domain.User{}, errMailDisabled
	}
	in.Username = strings.ToLower(strings.TrimSpace(in.Username))
	in.Email = strings.ToLower(strings.TrimSpace(in.Email))
	in.FullName = strings.TrimSpace(in.FullName)
	if in.FullName == "" {
		in.FullName = in.Username
	}
	v := domain.NewValidation()
	if !ValidUsername(in.Username) {
		v.Add("username", "3–32 תווים: אותיות לטיניות קטנות, ספרות, נקודה, מקף או קו תחתון")
	} else if taken, err := s.store.UsernameTaken(ctx, in.Username, uuid.Nil); err != nil {
		return domain.User{}, err
	} else if taken {
		v.Add("username", "שם המשתמש כבר תפוס")
	}
	if !UsableEmail(in.Email) {
		v.Add("email", "כתובת אימייל לא תקינה")
	} else if taken, err := s.store.EmailTaken(ctx, in.Email, uuid.Nil); err != nil {
		return domain.User{}, err
	} else if taken {
		v.Add("email", "כבר קיים משתמש עם אימייל זה")
	}
	if !in.Role.Valid() {
		v.Add("role", "תפקיד לא תקין")
	} else if !assignable(actor, in.Role) {
		v.Add("role", "אין לך הרשאה להעניק תפקיד זה")
	}
	if in.Role != domain.RoleStaff {
		in.StaffKind = nil
	}
	if err := v.OrNil(); err != nil {
		return domain.User{}, err
	}
	u, err := s.store.CreateUser(ctx, store.UserInput{Username: in.Username, Email: in.Email, FullName: in.FullName,
		Phone: in.Phone, Role: in.Role, StaffKind: in.StaffKind, IsActive: true})
	if err != nil {
		return u, err
	}
	s.audit(ctx, actor, "user", u.ID, "invite", map[string]any{"username": u.Username, "role": u.Role})
	inv, err := s.sendInvitation(ctx, actor.ID(), u)
	if err != nil {
		return u, err
	}
	u.Invitation = &inv
	s.emit(ctx, EvUserChanged, map[string]any{"id": u.ID})
	return u, nil
}

// ResendInvitation sends a new link (old ones stop working), even after expiry.
func (s *Service) ResendInvitation(ctx context.Context, actor Actor, id uuid.UUID) (domain.User, error) {
	if err := actor.Require(domain.ActManageUsers); err != nil {
		return domain.User{}, err
	}
	if s.mail == nil {
		return domain.User{}, errMailDisabled
	}
	u, err := s.store.UserByID(ctx, id)
	if err != nil {
		return u, err
	}
	v := domain.NewValidation()
	switch {
	case !u.IsActive:
		v.Add("status", "החשבון מושבת – הפעילו אותו לפני שליחת הזמנה")
	case u.Status == domain.AccountActive && u.HasPassword:
		v.Add("status", "החשבון כבר פעיל. אם המשתמש שכח את הסיסמה – \"שכחתי סיסמה\" במסך הכניסה")
	case !UsableEmail(u.Email):
		v.Add("email", "לחשבון אין כתובת אימייל תקינה – עדכנו אותה קודם")
	}
	if err := v.OrNil(); err != nil {
		return u, err
	}
	if last, err := s.store.LatestToken(ctx, u.ID, purposeInvite); err == nil {
		if wait := last.CreatedAt.Add(s.cfg.Auth.ResendCooldown).Sub(s.now()); wait > 0 {
			e := authErr(http.StatusTooManyRequests, "resend_too_soon", "הזמנה נשלחה הרגע – אפשר לשלוח שוב בעוד מספר שניות")
			e.RetryAfter = int(wait.Seconds()) + 1
			return u, e
		}
	}
	inv, err := s.sendInvitation(ctx, actor.ID(), u)
	if err != nil {
		return u, err
	}
	s.audit(ctx, actor, "user", u.ID, "resend_invitation", map[string]any{"status": inv.Status})
	u.Invitation = &inv
	return u, nil
}

// sendInvitation stores a fresh link and e-mails it. An SMTP refusal is
// recorded on the invitation (shown to the admin, who can resend), not returned.
func (s *Service) sendInvitation(ctx context.Context, by *uuid.UUID, u domain.User) (domain.Invitation, error) {
	if s.mail == nil {
		return domain.Invitation{}, errMailDisabled
	}
	raw, hash, err := newSecret()
	if err != nil {
		return domain.Invitation{}, err
	}
	now := s.now()
	tok, err := s.store.CreateToken(ctx, u.ID, purposeInvite, hash, now.Add(s.cfg.Auth.InviteTTL), now, by)
	if err != nil {
		return domain.Invitation{}, err
	}
	m := mailer.Invitation(s.brand(ctx), u.Email, u.FullName, u.Username, s.link("/activate", raw), s.cfg.Auth.InviteTTL)
	if err := s.mail.Send(ctx, m); err != nil {
		slog.Warn("auth: invitation not sent", "user", u.ID, "temporary", mailer.IsTemporary(err), "err", err)
		reason := err.Error()
		tok.DeliveryError = &reason
	} else {
		sent := s.now()
		tok.SentAt = &sent
	}
	if err := s.store.MarkTokenDelivery(ctx, tok.ID, tok.SentAt, tok.DeliveryError); err != nil {
		return domain.Invitation{}, err
	}
	return invitationOf(tok, s.now()), nil
}

func invitationOf(t store.AuthToken, now time.Time) domain.Invitation {
	inv := domain.Invitation{SentAt: t.SentAt, CreatedAt: t.CreatedAt, ExpiresAt: t.ExpiresAt, UsedAt: t.ConsumedAt, DeliveryError: t.DeliveryError}
	switch {
	case t.ConsumedAt != nil:
		inv.Status = "used"
	case t.RevokedAt != nil:
		inv.Status = "revoked"
	case t.SentAt == nil:
		inv.Status = "failed"
	case !now.Before(t.ExpiresAt):
		inv.Status = "expired"
	default:
		inv.Status = "sent"
	}
	return inv
}

// ── Links: invitation and password reset ─────────────────────

func (s *Service) linkState(ctx context.Context, purpose, raw string, lock bool, tx *store.Store) (store.AuthToken, domain.User, error) {
	if !validSecret(raw) {
		return store.AuthToken{}, domain.User{}, errLinkInvalid
	}
	t, err := tx.TokenByHash(ctx, hashSecret(raw), purpose, lock)
	if errors.Is(err, domain.ErrNotFound) {
		return t, domain.User{}, errLinkInvalid
	}
	if err != nil {
		return t, domain.User{}, err
	}
	u, err := tx.UserByID(ctx, t.UserID)
	if err != nil {
		return t, u, err
	}
	switch {
	case t.ConsumedAt != nil:
		return t, u, errLinkUsed
	case t.RevokedAt != nil:
		return t, u, errLinkReplaced
	case !s.now().Before(t.ExpiresAt):
		return t, u, errLinkExpired
	case !u.IsActive:
		return t, u, errAccountOff
	}
	return t, u, nil
}

// CheckLink tells the page whether a link still works. It never consumes it,
// so a mail scanner opening the link cannot spoil the invitation.
func (s *Service) CheckLink(ctx context.Context, purpose, raw string) (LinkInfo, error) {
	if purpose != purposeInvite && purpose != purposeReset {
		return LinkInfo{}, domain.ErrNotFound
	}
	t, u, err := s.linkState(ctx, purpose, raw, false, s.store)
	info := LinkInfo{Status: "valid", ExpiresAt: t.ExpiresAt}
	var ae *domain.AuthError
	switch {
	case err == nil:
	case errors.As(err, &ae) && ae != errLinkInvalid:
		info.Status = map[*domain.AuthError]string{errLinkUsed: "used", errLinkReplaced: "replaced", errLinkExpired: "expired", errAccountOff: "disabled"}[ae]
		return info, nil
	default:
		return info, err
	}
	info.Username, info.FullName, info.Email = u.Username, u.FullName, MaskEmail(u.Email)
	return info, nil
}

// setPasswordFromLink consumes a link (once, even under concurrent
// submissions), stores the password and ends every session and pending link.
func (s *Service) setPasswordFromLink(ctx context.Context, purpose, raw, pw, confirm string) (domain.User, error) {
	if !validSecret(raw) {
		return domain.User{}, errLinkInvalid
	}
	// Policy first: a typo must not burn the link.
	_, u, err := s.linkState(ctx, purpose, raw, false, s.store)
	if err != nil {
		return u, err
	}
	if err := CheckPasswordPolicy(pw, confirm, u); err != nil {
		return u, err
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(pw), bcryptCost)
	if err != nil {
		return u, err
	}
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		t, cur, err := s.linkState(ctx, purpose, raw, true, tx)
		if err != nil {
			return err
		}
		if won, err := tx.ConsumeToken(ctx, t.ID, s.now()); err != nil {
			return err
		} else if !won {
			return errLinkUsed
		}
		if err := tx.SetPassword(ctx, cur.ID, string(hash), s.now(), true); err != nil {
			return err
		}
		for _, p := range []string{purposeInvite, purposeReset} {
			if err := tx.RevokeTokens(ctx, cur.ID, p, s.now()); err != nil {
				return err
			}
		}
		if err := tx.EndChallenges(ctx, cur.ID, s.now()); err != nil {
			return err
		}
		u, err = tx.UserByID(ctx, cur.ID)
		return err
	})
	return u, err
}

// Activate sets the first password from an invitation, then sends the code.
// Closing the page afterwards is fine: a normal login finishes the activation.
func (s *Service) Activate(ctx context.Context, info RequestInfo, raw, pw, confirm string) (ChallengeResult, error) {
	if s.mail == nil {
		return ChallengeResult{}, errMailDisabled
	}
	u, err := s.setPasswordFromLink(ctx, purposeInvite, raw, pw, confirm)
	if err != nil {
		return ChallengeResult{}, err
	}
	s.audit(ctx, Actor{User: u}, "user", u.ID, "password_set", map[string]any{"via": "invitation", "ip": info.IP})
	flow := flowLogin
	if u.ActivatedAt == nil {
		flow = flowActivation
	}
	return s.startChallenge(ctx, info, u, flow)
}

// ResetPassword sets a new password from a reset link; every session ends
// and the next login asks for an e-mail code.
func (s *Service) ResetPassword(ctx context.Context, info RequestInfo, raw, pw, confirm string) error {
	u, err := s.setPasswordFromLink(ctx, purposeReset, raw, pw, confirm)
	if err != nil {
		return err
	}
	s.audit(ctx, Actor{User: u}, "user", u.ID, "password_reset", map[string]any{"ip": info.IP})
	return nil
}

// ForgotPassword always answers the same way and as fast; the e-mail (if the
// account exists and is active) leaves in the background.
func (s *Service) ForgotPassword(ctx context.Context, info RequestInfo, identifier string) error {
	identifier = strings.TrimSpace(identifier)
	if identifier == "" {
		v := domain.NewValidation()
		v.Add("identifier", "יש להזין שם משתמש או אימייל")
		return v
	}
	if s.over(ctx, evForgot, ipKey(info), s.cfg.Auth.MaxSendsPerIP) {
		return tooMany(s.cfg.Auth.Window)
	}
	s.hit(ctx, evForgot, ipKey(info))
	if s.mail == nil {
		return nil
	}
	s.bg.Add(1)
	go func() {
		defer s.bg.Done()
		bctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Minute)
		defer cancel()
		s.sendReset(bctx, info, identifier)
	}()
	return nil
}

func (s *Service) sendReset(ctx context.Context, info RequestInfo, identifier string) {
	u, found := s.lookupLogin(ctx, identifier)
	if !found || !u.IsActive || !UsableEmail(u.Email) {
		return
	}
	if s.over(ctx, evSend, userKey(u.ID), s.cfg.Auth.MaxSendsPerAccount) {
		return
	}
	if last, err := s.store.LatestToken(ctx, u.ID, purposeReset); err == nil && last.CreatedAt.Add(s.cfg.Auth.ResendCooldown).After(s.now()) {
		return
	}
	raw, hash, err := newSecret()
	if err != nil {
		return
	}
	now := s.now()
	tok, err := s.store.CreateToken(ctx, u.ID, purposeReset, hash, now.Add(s.cfg.Auth.ResetTTL), now, nil)
	if err != nil {
		slog.Error("auth: reset token", "err", err)
		return
	}
	s.hit(ctx, evSend, userKey(u.ID))
	m := mailer.PasswordReset(s.brand(ctx), u.Email, u.FullName, s.link("/reset", raw), s.cfg.Auth.ResetTTL)
	var sentAt *time.Time
	var reason *string
	if err := s.mail.Send(ctx, m); err != nil {
		slog.Warn("auth: reset e-mail not sent", "user", u.ID, "temporary", mailer.IsTemporary(err), "err", err)
		r := err.Error()
		reason = &r
	} else {
		t := s.now()
		sentAt = &t
	}
	if err := s.store.MarkTokenDelivery(ctx, tok.ID, sentAt, reason); err != nil {
		slog.Error("auth: reset delivery state", "err", err)
	}
}

// ── Initial super administrator ──────────────────────────────

// BootstrapSuperAdmin creates the instance's first, pending super
// administrator (BOOTSTRAP_SUPER_ADMIN_EMAIL) and e-mails the invitation.
// It runs once per instance: later runs never duplicate the account, touch
// its password, re-enable it, resend the invitation, or follow a changed
// variable to grant the role to another address.
func (s *Service) BootstrapSuperAdmin(ctx context.Context) error {
	if !s.cfg.PasswordAuth() {
		return nil
	}
	email := strings.ToLower(strings.TrimSpace(s.cfg.BootstrapSuperAdminEmail))
	var created *domain.User
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		if err := tx.LockBootstrap(ctx); err != nil {
			return err
		}
		if _, done, err := tx.State(ctx, stateBoot); err != nil || done {
			if done {
				slog.Info("bootstrap: initial super administrator already handled; BOOTSTRAP_SUPER_ADMIN_EMAIL is not used anymore")
			}
			return err
		}
		if n, err := tx.CountRole(ctx, domain.RoleSuperAdmin); err != nil {
			return err
		} else if n > 0 {
			slog.Info("bootstrap: a super administrator already exists, nothing to create")
			return tx.SetState(ctx, stateBoot, "existing")
		}
		if email == "" {
			slog.Warn("bootstrap: no super administrator yet – set BOOTSTRAP_SUPER_ADMIN_EMAIL and restart")
			return nil
		}
		if !UsableEmail(email) {
			return fmt.Errorf("bootstrap: BOOTSTRAP_SUPER_ADMIN_EMAIL is not a deliverable address")
		}
		if taken, err := tx.EmailTaken(ctx, email, uuid.Nil); err != nil {
			return err
		} else if taken {
			return fmt.Errorf("bootstrap: BOOTSTRAP_SUPER_ADMIN_EMAIL belongs to an existing account; refusing to grant it super administrator rights")
		}
		u, err := tx.CreateUser(ctx, store.UserInput{Email: email, FullName: "מנהל המערכת", Role: domain.RoleSuperAdmin, IsActive: true})
		if err != nil {
			return err
		}
		created = &u
		return tx.SetState(ctx, stateBoot, u.ID.String())
	})
	if err != nil || created == nil {
		return err
	}
	slog.Info("bootstrap: initial super administrator created", "username", created.Username)
	if s.mail == nil {
		slog.Error("bootstrap: SMTP not configured – invitation not sent; configure it, then run `server bootstrap-resend`")
		return nil
	}
	inv, err := s.sendInvitation(ctx, nil, *created)
	if err != nil {
		return err
	}
	if inv.Status == "failed" {
		slog.Error("bootstrap: invitation e-mail failed – fix SMTP settings, then run `server bootstrap-resend`", "reason", *inv.DeliveryError)
	} else {
		slog.Info("bootstrap: invitation e-mailed to the initial super administrator")
	}
	return nil
}

// ResendBootstrapInvitation is the operator command for a lost or refused
// first invitation. It only works while that account is still pending.
func (s *Service) ResendBootstrapInvitation(ctx context.Context) (domain.User, domain.Invitation, error) {
	v, done, err := s.store.State(ctx, stateBoot)
	if err != nil {
		return domain.User{}, domain.Invitation{}, err
	}
	id, perr := uuid.Parse(v)
	if !done || perr != nil {
		return domain.User{}, domain.Invitation{}, errors.New("no initial super administrator was created by this instance")
	}
	u, err := s.store.UserByID(ctx, id)
	if err != nil {
		return u, domain.Invitation{}, err
	}
	switch {
	case !u.IsActive:
		return u, domain.Invitation{}, errors.New("the initial super administrator is disabled; it will not be re-enabled")
	case u.Status != domain.AccountPending:
		return u, domain.Invitation{}, errors.New("the initial super administrator is already activated; use \"forgot password\" on the login page")
	case s.mail == nil:
		return u, domain.Invitation{}, errors.New("SMTP is not configured")
	}
	inv, err := s.sendInvitation(ctx, nil, u)
	return u, inv, err
}

// RecoverSuperAdmin is the operator procedure when the only super
// administrator lost their mailbox: new address, password cleared, every
// session and link revoked, and a new invitation sent to the new address.
// It is reachable only from the server's command line.
func (s *Service) RecoverSuperAdmin(ctx context.Context, who, newEmail string) (domain.User, domain.Invitation, error) {
	newEmail = strings.ToLower(strings.TrimSpace(newEmail))
	if !UsableEmail(newEmail) {
		return domain.User{}, domain.Invitation{}, errors.New("the new e-mail address is not valid")
	}
	if s.mail == nil {
		return domain.User{}, domain.Invitation{}, errors.New("SMTP is not configured")
	}
	u, found := s.lookupLogin(ctx, who)
	if !found || u.Role != domain.RoleSuperAdmin {
		return u, domain.Invitation{}, errors.New("no super administrator with this username / e-mail")
	}
	if !u.IsActive {
		return u, domain.Invitation{}, errors.New("this super administrator is disabled")
	}
	if taken, err := s.store.EmailTaken(ctx, newEmail, u.ID); err != nil || taken {
		if err == nil {
			err = errors.New("the new e-mail address already belongs to another account")
		}
		return u, domain.Invitation{}, err
	}
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		if err := tx.SetUserEmail(ctx, u.ID, newEmail); err != nil {
			return err
		}
		if err := tx.SetPassword(ctx, u.ID, "", s.now(), true); err != nil {
			return err
		}
		for _, p := range []string{purposeInvite, purposeReset} {
			if err := tx.RevokeTokens(ctx, u.ID, p, s.now()); err != nil {
				return err
			}
		}
		return tx.EndChallenges(ctx, u.ID, s.now())
	})
	if err != nil {
		return u, domain.Invitation{}, err
	}
	if err := s.store.Audit(ctx, nil, "user", &u.ID, "recover_super_admin", map[string]any{"via": "cli"}); err != nil {
		slog.Error("audit write failed", "err", err)
	}
	u, err = s.store.UserByID(ctx, u.ID)
	if err != nil {
		return u, domain.Invitation{}, err
	}
	inv, err := s.sendInvitation(ctx, nil, u)
	return u, inv, err
}

// ── Pre-deployment report ────────────────────────────────────

type AuthReport struct {
	Unusable   []domain.User   // no deliverable e-mail: cannot receive codes
	Duplicates [][]domain.User // same e-mail ignoring case: e-mail login refused
	NoPassword []domain.User   // active, must set a password (invitation / forgot password)
	Pending    []domain.User
}

func (s *Service) AuthReport(ctx context.Context) (AuthReport, error) {
	users, err := s.store.ListUsers(ctx, store.UserFilter{})
	if err != nil {
		return AuthReport{}, err
	}
	var r AuthReport
	groups := map[string][]domain.User{}
	var order []string
	for _, u := range users {
		k := strings.ToLower(strings.TrimSpace(u.Email))
		if _, seen := groups[k]; !seen {
			order = append(order, k)
		}
		groups[k] = append(groups[k], u)
		if !UsableEmail(u.Email) {
			r.Unusable = append(r.Unusable, u)
		}
		switch {
		case u.Status == domain.AccountPending:
			r.Pending = append(r.Pending, u)
		case u.IsActive && !u.HasPassword:
			r.NoPassword = append(r.NoPassword, u)
		}
	}
	for _, k := range order {
		if len(groups[k]) > 1 {
			r.Duplicates = append(r.Duplicates, groups[k])
		}
	}
	return r, nil
}

// PurgeAuth drops old rate-limit events and finished challenges.
func (s *Service) PurgeAuth(ctx context.Context) {
	if err := s.store.PurgeAuth(ctx, s.now().Add(-24*time.Hour)); err != nil {
		slog.Error("auth: purge failed", "err", err)
	}
}
