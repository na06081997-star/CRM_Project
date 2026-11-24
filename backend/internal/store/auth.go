package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

// AuthToken is a single-use link (invitation or password reset); only its hash is stored.
type AuthToken struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Purpose       string
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
	RevokedAt     *time.Time
	SentAt        *time.Time
	DeliveryError *string
	CreatedAt     time.Time
}

const tokenCols = `id, user_id, purpose, expires_at, consumed_at, revoked_at, sent_at, delivery_error, created_at`

func scanToken(r pgx.Row) (AuthToken, error) {
	var t AuthToken
	err := r.Scan(&t.ID, &t.UserID, &t.Purpose, &t.ExpiresAt, &t.ConsumedAt, &t.RevokedAt, &t.SentAt, &t.DeliveryError, &t.CreatedAt)
	return t, mapErr(err)
}

// CreateToken revokes the account's previous unused links of the same purpose
// and stores the new one.
func (s *Store) CreateToken(ctx context.Context, userID uuid.UUID, purpose string, hash []byte, expires, now time.Time, by *uuid.UUID) (AuthToken, error) {
	if err := s.RevokeTokens(ctx, userID, purpose, now); err != nil {
		return AuthToken{}, err
	}
	return scanToken(s.q.QueryRow(ctx, `
		INSERT INTO auth_tokens (user_id, purpose, token_hash, expires_at, created_by, created_at)
		VALUES ($1, $2, $3, $4, $5, $6) RETURNING `+tokenCols, userID, purpose, hash, expires, by, now))
}

func (s *Store) RevokeTokens(ctx context.Context, userID uuid.UUID, purpose string, now time.Time) error {
	_, err := s.q.Exec(ctx, `UPDATE auth_tokens SET revoked_at = $3
		WHERE user_id = $1 AND purpose = $2 AND consumed_at IS NULL AND revoked_at IS NULL`, userID, purpose, now)
	return mapErr(err)
}

// MarkTokenDelivery records whether the e-mail carrying the link left.
func (s *Store) MarkTokenDelivery(ctx context.Context, id uuid.UUID, sentAt *time.Time, deliveryErr *string) error {
	_, err := s.q.Exec(ctx, `UPDATE auth_tokens SET sent_at = $2, delivery_error = $3 WHERE id = $1`, id, sentAt, deliveryErr)
	return mapErr(err)
}

// TokenByHash reads a link without consuming it. lock=true locks the row
// until the transaction ends, so two submissions cannot both consume it.
func (s *Store) TokenByHash(ctx context.Context, hash []byte, purpose string, lock bool) (AuthToken, error) {
	q := `SELECT ` + tokenCols + ` FROM auth_tokens WHERE token_hash = $1 AND purpose = $2`
	if lock {
		q += ` FOR UPDATE`
	}
	return scanToken(s.q.QueryRow(ctx, q, hash, purpose))
}

// ConsumeToken marks a link used; false when it already was.
func (s *Store) ConsumeToken(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	tag, err := s.q.Exec(ctx, `UPDATE auth_tokens SET consumed_at = $2 WHERE id = $1 AND consumed_at IS NULL AND revoked_at IS NULL`, id, now)
	return tag.RowsAffected() == 1, mapErr(err)
}

// LatestTokens returns the newest link of each account for a purpose.
func (s *Store) LatestTokens(ctx context.Context, purpose string) (map[uuid.UUID]AuthToken, error) {
	rows, err := s.q.Query(ctx, `SELECT DISTINCT ON (user_id) `+tokenCols+` FROM auth_tokens
		WHERE purpose = $1 ORDER BY user_id, created_at DESC`, purpose)
	list, err := collect(rows, err, scanToken)
	out := make(map[uuid.UUID]AuthToken, len(list))
	for _, t := range list {
		out[t.UserID] = t
	}
	return out, err
}

func (s *Store) LatestToken(ctx context.Context, userID uuid.UUID, purpose string) (AuthToken, error) {
	return scanToken(s.q.QueryRow(ctx, `SELECT `+tokenCols+` FROM auth_tokens WHERE user_id = $1 AND purpose = $2
		ORDER BY created_at DESC LIMIT 1`, userID, purpose))
}

// ── Code challenges ──────────────────────────────────────────

type Challenge struct {
	ID            uuid.UUID
	UserID        uuid.UUID
	Flow          string
	CodeExpiresAt *time.Time
	Generation    int
	Attempts      int
	Sends         int
	LastSentAt    *time.Time
	ExpiresAt     time.Time
	ConsumedAt    *time.Time
}

const challengeCols = `id, user_id, flow, code_expires_at, generation, attempts, sends, last_sent_at, expires_at, consumed_at`

func scanChallenge(r pgx.Row) (Challenge, error) {
	var c Challenge
	err := r.Scan(&c.ID, &c.UserID, &c.Flow, &c.CodeExpiresAt, &c.Generation, &c.Attempts, &c.Sends, &c.LastSentAt, &c.ExpiresAt, &c.ConsumedAt)
	return c, mapErr(err)
}

func (s *Store) CreateChallenge(ctx context.Context, userID uuid.UUID, flow string, handleHash []byte, expires, now time.Time) (Challenge, error) {
	return scanChallenge(s.q.QueryRow(ctx, `INSERT INTO auth_challenges (user_id, flow, handle_hash, expires_at, created_at)
		VALUES ($1, $2, $3, $4, $5) RETURNING `+challengeCols, userID, flow, handleHash, expires, now))
}

func (s *Store) ChallengeByHandle(ctx context.Context, handleHash []byte) (Challenge, error) {
	return scanChallenge(s.q.QueryRow(ctx, `SELECT `+challengeCols+` FROM auth_challenges WHERE handle_hash = $1`, handleHash))
}

// SetChallengeCode replaces the code of a challenge (the previous one stops
// working) if nobody else did in the meantime. It returns false otherwise.
func (s *Store) SetChallengeCode(ctx context.Context, id uuid.UUID, generation int, hmac []byte, codeExpires, now time.Time) (bool, error) {
	tag, err := s.q.Exec(ctx, `UPDATE auth_challenges
		SET code_hmac = $3, code_expires_at = $4, generation = $2, attempts = 0, sends = sends + 1, last_sent_at = $5
		WHERE id = $1 AND generation = $2 - 1 AND consumed_at IS NULL`, id, generation, hmac, codeExpires, now)
	return tag.RowsAffected() == 1, mapErr(err)
}

// ClearChallengeCode voids a code whose e-mail could not be sent, so the
// user can ask again right away.
func (s *Store) ClearChallengeCode(ctx context.Context, id uuid.UUID, generation int) error {
	_, err := s.q.Exec(ctx, `UPDATE auth_challenges SET code_hmac = NULL, last_sent_at = NULL WHERE id = $1 AND generation = $2`, id, generation)
	return mapErr(err)
}

// ReserveAttempt atomically counts one try against the current code and
// returns it for comparison. ok is false when the code is missing, expired,
// used, or out of attempts — concurrent guesses can never exceed max.
func (s *Store) ReserveAttempt(ctx context.Context, id uuid.UUID, max int, now time.Time) (hmac []byte, generation int, ok bool, err error) {
	err = s.q.QueryRow(ctx, `UPDATE auth_challenges SET attempts = attempts + 1
		WHERE id = $1 AND consumed_at IS NULL AND code_hmac IS NOT NULL AND code_expires_at > $3 AND expires_at > $3 AND attempts < $2
		RETURNING code_hmac, generation`, id, max, now).Scan(&hmac, &generation)
	if err == pgx.ErrNoRows {
		return nil, 0, false, nil
	}
	return hmac, generation, err == nil, mapErr(err)
}

// ConsumeChallenge ends a challenge; only one caller ever gets true.
func (s *Store) ConsumeChallenge(ctx context.Context, id uuid.UUID, now time.Time) (bool, error) {
	tag, err := s.q.Exec(ctx, `UPDATE auth_challenges SET consumed_at = $2 WHERE id = $1 AND consumed_at IS NULL`, id, now)
	return tag.RowsAffected() == 1, mapErr(err)
}

// EndChallenges closes every open challenge of an account (password changed…).
func (s *Store) EndChallenges(ctx context.Context, userID uuid.UUID, now time.Time) error {
	_, err := s.q.Exec(ctx, `UPDATE auth_challenges SET consumed_at = $2 WHERE user_id = $1 AND consumed_at IS NULL`, userID, now)
	return mapErr(err)
}

// ── Rate limiting ────────────────────────────────────────────

func (s *Store) AddAuthEvent(ctx context.Context, kind, key string, now time.Time) error {
	_, err := s.q.Exec(ctx, `INSERT INTO auth_events (kind, key, created_at) VALUES ($1, $2, $3)`, kind, key, now)
	return mapErr(err)
}

func (s *Store) CountAuthEvents(ctx context.Context, kind, key string, since time.Time) (int, error) {
	var n int
	err := s.q.QueryRow(ctx, `SELECT count(*) FROM auth_events WHERE kind = $1 AND key = $2 AND created_at > $3`, kind, key, since).Scan(&n)
	return n, mapErr(err)
}

// PurgeAuth drops rate-limit events and finished challenges older than before.
func (s *Store) PurgeAuth(ctx context.Context, before time.Time) error {
	if _, err := s.q.Exec(ctx, `DELETE FROM auth_events WHERE created_at < $1`, before); err != nil {
		return mapErr(err)
	}
	_, err := s.q.Exec(ctx, `DELETE FROM auth_challenges WHERE expires_at < $1`, before)
	return mapErr(err)
}

// ── Instance state ───────────────────────────────────────────

func (s *Store) State(ctx context.Context, key string) (string, bool, error) {
	var v string
	err := s.q.QueryRow(ctx, `SELECT value FROM system_state WHERE key = $1`, key).Scan(&v)
	if err == pgx.ErrNoRows {
		return "", false, nil
	}
	return v, err == nil, mapErr(err)
}

func (s *Store) SetState(ctx context.Context, key, value string) error {
	_, err := s.q.Exec(ctx, `INSERT INTO system_state (key, value) VALUES ($1, $2)
		ON CONFLICT (key) DO UPDATE SET value = EXCLUDED.value, updated_at = now()`, key, value)
	return mapErr(err)
}

// LockBootstrap serialises instance initialisation across replicas (transaction-scoped).
func (s *Store) LockBootstrap(ctx context.Context) error {
	_, err := s.q.Exec(ctx, `SELECT pg_advisory_xact_lock(727272)`)
	return mapErr(err)
}
