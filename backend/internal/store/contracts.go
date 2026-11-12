package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const contractSelect = `
	SELECT c.id, c.user_id, c.title, c.body, c.method, c.status, c.token, c.expires_at, c.sent_at,
	       c.signed_name, c.signed_at, c.signed_ip, c.created_at, u.full_name, u.email, u.phone
	FROM contracts c
	JOIN users u ON u.id = c.user_id`

func scanContract(r pgx.Row) (domain.Contract, error) {
	var c domain.Contract
	err := r.Scan(&c.ID, &c.UserID, &c.Title, &c.Body, &c.Method, &c.Status, &c.Token, &c.ExpiresAt, &c.SentAt,
		&c.SignedName, &c.SignedAt, &c.SignedIP, &c.CreatedAt, &c.UserName, &c.UserEmail, &c.UserPhone)
	return c, mapErr(err)
}

// ListContracts returns contracts, newest first, optionally for one user.
func (s *Store) ListContracts(ctx context.Context, userID *uuid.UUID) ([]domain.Contract, error) {
	w := &where{}
	if userID != nil {
		w.add("c.user_id = $%d", *userID)
	}
	rows, err := s.q.Query(ctx, contractSelect+w.sql()+` ORDER BY c.created_at DESC`, w.args...)
	return collect(rows, err, scanContract)
}

func (s *Store) ContractByID(ctx context.Context, id uuid.UUID) (domain.Contract, error) {
	return scanContract(s.q.QueryRow(ctx, contractSelect+` WHERE c.id = $1`, id))
}

func (s *Store) ContractByToken(ctx context.Context, token string) (domain.Contract, error) {
	return scanContract(s.q.QueryRow(ctx, contractSelect+` WHERE c.token = $1`, token))
}

func (s *Store) CreateContract(ctx context.Context, c domain.Contract, createdBy *uuid.UUID) (domain.Contract, error) {
	var id uuid.UUID
	err := s.q.QueryRow(ctx, `
		INSERT INTO contracts (user_id, title, body, method, token, expires_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		c.UserID, c.Title, c.Body, c.Method, c.Token, c.ExpiresAt, createdBy).Scan(&id)
	if err != nil {
		return domain.Contract{}, mapErr(err)
	}
	return s.ContractByID(ctx, id)
}

// MarkContractSent records a (re)send and extends the link's validity.
func (s *Store) MarkContractSent(ctx context.Context, id uuid.UUID, expires time.Time) error {
	_, err := s.q.Exec(ctx, `UPDATE contracts SET sent_at = now(), expires_at = $2, updated_at = now() WHERE id = $1`, id, expires)
	return mapErr(err)
}

// SignContract marks a pending contract signed; false when it was not pending.
func (s *Store) SignContract(ctx context.Context, id uuid.UUID, name, ip string) (bool, error) {
	tag, err := s.q.Exec(ctx, `
		UPDATE contracts SET status = 'signed', signed_name = $2, signed_ip = $3, signed_at = now(), updated_at = now()
		WHERE id = $1 AND status = 'pending'`, id, name, ip)
	return tag.RowsAffected() > 0, mapErr(err)
}

func (s *Store) CancelContract(ctx context.Context, id uuid.UUID) error {
	tag, err := s.q.Exec(ctx, `UPDATE contracts SET status = 'cancelled', updated_at = now() WHERE id = $1 AND status = 'pending'`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}

// SetUserActive turns an account on or off (contract signature onboarding).
func (s *Store) SetUserActive(ctx context.Context, id uuid.UUID, active bool) error {
	_, err := s.q.Exec(ctx, `UPDATE users SET is_active = $2, updated_at = now() WHERE id = $1`, id, active)
	return mapErr(err)
}
