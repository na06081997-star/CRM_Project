package store

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const userCols = `id, firebase_uid, username, email, full_name, phone, role, staff_kind, telegram_chat_id, avatar_color, is_active, created_at,
	password_hash IS NOT NULL, activated_at, first_login_at, last_login_at, login_history_known, token_version`

func scanUser(r pgx.Row) (domain.User, error) {
	var u domain.User
	err := r.Scan(&u.ID, &u.FirebaseUID, &u.Username, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.StaffKind,
		&u.TelegramChatID, &u.AvatarColor, &u.IsActive, &u.CreatedAt,
		&u.HasPassword, &u.ActivatedAt, &u.FirstLoginAt, &u.LastLoginAt, &u.LoginHistoryKnown, &u.TokenVersion)
	switch {
	case !u.IsActive:
		u.Status = domain.AccountDisabled
	case u.ActivatedAt == nil:
		u.Status = domain.AccountPending
	default:
		u.Status = domain.AccountActive
	}
	return u, mapErr(err)
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

// UserByEmail matches case-insensitively. Several accounts sharing an e-mail
// (legacy data, see `server auth-report`) give ErrNotFound: never guess.
func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	users, err := s.UsersByEmail(ctx, email)
	if err != nil {
		return domain.User{}, err
	}
	if len(users) != 1 {
		return domain.User{}, domain.ErrNotFound
	}
	return users[0], nil
}

func (s *Store) UsersByEmail(ctx context.Context, email string) ([]domain.User, error) {
	rows, err := s.q.Query(ctx, `SELECT `+userCols+` FROM users WHERE lower(email) = lower($1) ORDER BY created_at`, strings.TrimSpace(email))
	return collect(rows, err, scanUser)
}

func (s *Store) UserByUsername(ctx context.Context, username string) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE lower(username) = lower($1)`, strings.TrimSpace(username)))
}

type UserFilter struct {
	Role   string
	Search string
	Active *bool
}

func (s *Store) ListUsers(ctx context.Context, f UserFilter) ([]domain.User, error) {
	w := &where{}
	if f.Role != "" {
		w.add("role = $%d", f.Role)
	}
	if f.Active != nil {
		w.add("is_active = $%d", *f.Active)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		p := w.next("%" + q + "%")
		w.raw("(full_name ILIKE " + p + " OR email ILIKE " + p + " OR username ILIKE " + p + " OR phone ILIKE " + p + ")")
	}
	rows, err := s.q.Query(ctx, `SELECT `+userCols+` FROM users`+w.sql()+` ORDER BY role, full_name`, w.args...)
	return collect(rows, err, scanUser)
}

type UserInput struct {
	Username       string
	Email          string
	FullName       string
	Phone          *string
	Role           domain.Role
	StaffKind      *string
	TelegramChatID *string
	IsActive       bool
	FirebaseUID    *string
}

// CreateUser inserts an account; an empty Username is derived from the e-mail.
func (s *Store) CreateUser(ctx context.Context, in UserInput) (domain.User, error) {
	if in.Username == "" {
		name, err := s.FreeUsername(ctx, in.Email)
		if err != nil {
			return domain.User{}, err
		}
		in.Username = name
	}
	return scanUser(s.q.QueryRow(ctx, `
		INSERT INTO users (username, email, full_name, phone, role, staff_kind, telegram_chat_id, is_active, firebase_uid, avatar_color)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9,
		        (ARRAY['#0f766e','#7c3aed','#c2410c','#0369a1','#be123c','#4d7c0f','#a16207'])[1 + floor(random() * 7)::int])
		RETURNING `+userCols,
		in.Username, in.Email, in.FullName, in.Phone, in.Role, in.StaffKind, in.TelegramChatID, in.IsActive, in.FirebaseUID))
}

func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, in UserInput) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `
		UPDATE users SET username = coalesce(nullif($9, ''), username), email = $2, full_name = $3, phone = $4, role = $5,
		       staff_kind = $6, telegram_chat_id = $7, is_active = $8, updated_at = now()
		WHERE id = $1 RETURNING `+userCols,
		id, in.Email, in.FullName, in.Phone, in.Role, in.StaffKind, in.TelegramChatID, in.IsActive, in.Username))
}

var usernameStrip = regexp.MustCompile(`[^a-z0-9._-]`)

// FreeUsername derives an unused username from an e-mail's local part.
func (s *Store) FreeUsername(ctx context.Context, email string) (string, error) {
	base := usernameStrip.ReplaceAllString(strings.ToLower(strings.SplitN(email, "@", 2)[0]), "")
	base = strings.TrimLeft(base, "._-")
	if len(base) < 3 {
		base = "user" + base
	}
	if len(base) > 28 {
		base = base[:28]
	}
	cand := base
	for n := 2; ; n++ {
		taken, err := s.UsernameTaken(ctx, cand, uuid.Nil)
		if err != nil || !taken {
			return cand, err
		}
		cand = fmt.Sprintf("%s%d", base, n)
	}
}

// UsernameTaken / EmailTaken compare case-insensitively, ignoring account `except`.
func (s *Store) UsernameTaken(ctx context.Context, username string, except uuid.UUID) (bool, error) {
	var ok bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(username) = lower($1) AND id <> $2)`, username, except).Scan(&ok)
	return ok, mapErr(err)
}

func (s *Store) EmailTaken(ctx context.Context, email string, except uuid.UUID) (bool, error) {
	var ok bool
	err := s.q.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE lower(email) = lower($1) AND id <> $2)`, email, except).Scan(&ok)
	return ok, mapErr(err)
}

func (s *Store) LinkFirebaseUID(ctx context.Context, id uuid.UUID, uid string) error {
	_, err := s.q.Exec(ctx, `UPDATE users SET firebase_uid = $2, updated_at = now() WHERE id = $1`, id, uid)
	return mapErr(err)
}

func (s *Store) CountUsers(ctx context.Context) (int, error) {
	var n int
	err := s.q.QueryRow(ctx, `SELECT count(*) FROM users`).Scan(&n)
	return n, mapErr(err)
}

func (s *Store) CountRole(ctx context.Context, role domain.Role) (int, error) {
	var n int
	err := s.q.QueryRow(ctx, `SELECT count(*) FROM users WHERE role = $1`, role).Scan(&n)
	return n, mapErr(err)
}

// ── Passwords and logins ─────────────────────────────────────

func (s *Store) PasswordHash(ctx context.Context, id uuid.UUID) (string, error) {
	var h *string
	if err := s.q.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, id).Scan(&h); err != nil {
		return "", mapErr(err)
	}
	if h == nil {
		return "", nil
	}
	return *h, nil
}

// SetPassword stores a new hash (empty clears it). revoke also ends every
// session of the account by bumping its token version.
func (s *Store) SetPassword(ctx context.Context, id uuid.UUID, hash string, now time.Time, revoke bool) error {
	var h *string
	if hash != "" {
		h = &hash
	}
	_, err := s.q.Exec(ctx, `
		UPDATE users SET password_hash = $2, password_changed_at = $3,
		       token_version = token_version + CASE WHEN $4 THEN 1 ELSE 0 END, updated_at = now()
		WHERE id = $1`, id, h, now, revoke)
	return mapErr(err)
}

func (s *Store) RevokeSessions(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.Exec(ctx, `UPDATE users SET token_version = token_version + 1, updated_at = now() WHERE id = $1`, id)
	return mapErr(err)
}

// RecordLogin marks a successful login with code (the first one activates the account).
func (s *Store) RecordLogin(ctx context.Context, id uuid.UUID, now time.Time) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `
		UPDATE users SET activated_at = coalesce(activated_at, $2), first_login_at = coalesce(first_login_at, $2),
		       last_login_at = $2, updated_at = now()
		WHERE id = $1 RETURNING `+userCols, id, now))
}

// SetUserEmail changes the address (operator recovery).
func (s *Store) SetUserEmail(ctx context.Context, id uuid.UUID, email string) error {
	_, err := s.q.Exec(ctx, `UPDATE users SET email = $2, updated_at = now() WHERE id = $1`, id, email)
	return mapErr(err)
}
