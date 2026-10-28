package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const userCols = `id, firebase_uid, email, full_name, phone, role, staff_kind, telegram_chat_id, avatar_color, is_active, created_at`

func scanUser(r pgx.Row) (domain.User, error) {
	var u domain.User
	err := r.Scan(&u.ID, &u.FirebaseUID, &u.Email, &u.FullName, &u.Phone, &u.Role, &u.StaffKind,
		&u.TelegramChatID, &u.AvatarColor, &u.IsActive, &u.CreatedAt)
	return u, mapErr(err)
}

func (s *Store) UserByID(ctx context.Context, id uuid.UUID) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE id = $1`, id))
}

func (s *Store) UserByEmail(ctx context.Context, email string) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE lower(email) = lower($1)`, email))
}

func (s *Store) UserByFirebaseUID(ctx context.Context, uid string) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `SELECT `+userCols+` FROM users WHERE firebase_uid = $1`, uid))
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
		w.raw("(full_name ILIKE " + p + " OR email ILIKE " + p + " OR phone ILIKE " + p + ")")
	}
	rows, err := s.q.Query(ctx, `SELECT `+userCols+` FROM users`+w.sql()+` ORDER BY role, full_name`, w.args...)
	return collect(rows, err, scanUser)
}

type UserInput struct {
	Email          string
	FullName       string
	Phone          *string
	Role           domain.Role
	StaffKind      *string
	TelegramChatID *string
	IsActive       bool
	FirebaseUID    *string
}

func (s *Store) CreateUser(ctx context.Context, in UserInput) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `
		INSERT INTO users (email, full_name, phone, role, staff_kind, telegram_chat_id, is_active, firebase_uid, avatar_color)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8,
		        (ARRAY['#0f766e','#7c3aed','#c2410c','#0369a1','#be123c','#4d7c0f','#a16207'])[1 + floor(random() * 7)::int])
		RETURNING `+userCols,
		in.Email, in.FullName, in.Phone, in.Role, in.StaffKind, in.TelegramChatID, in.IsActive, in.FirebaseUID))
}

func (s *Store) UpdateUser(ctx context.Context, id uuid.UUID, in UserInput) (domain.User, error) {
	return scanUser(s.q.QueryRow(ctx, `
		UPDATE users SET email = $2, full_name = $3, phone = $4, role = $5, staff_kind = $6,
		       telegram_chat_id = $7, is_active = $8, updated_at = now()
		WHERE id = $1 RETURNING `+userCols,
		id, in.Email, in.FullName, in.Phone, in.Role, in.StaffKind, in.TelegramChatID, in.IsActive))
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
