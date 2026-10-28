// Package store is the PostgreSQL repository layer.
package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

type querier interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

type Store struct {
	pool *pgxpool.Pool
	q    querier
}

func New(pool *pgxpool.Pool) *Store { return &Store{pool: pool, q: pool} }

func (s *Store) Ping(ctx context.Context) error { return s.pool.Ping(ctx) }

// InTx runs fn with a Store bound to a single transaction.
func (s *Store) InTx(ctx context.Context, fn func(tx *Store) error) error {
	if _, nested := s.q.(pgx.Tx); nested {
		return fn(s)
	}
	return pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		return fn(&Store{pool: s.pool, q: tx})
	})
}

// ErrOverlap is returned when the bookings_no_overlap constraint fires.
var ErrOverlap = errors.New("booking overlaps an existing booking")

func mapErr(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ErrNotFound
	}
	var pg *pgconn.PgError
	if errors.As(err, &pg) {
		switch pg.Code {
		case "23P01":
			return ErrOverlap
		case "23505":
			v := domain.NewValidation()
			v.Add(pg.ConstraintName, "ערך זה כבר קיים במערכת")
			return v
		case "23503":
			v := domain.NewValidation()
			v.Add(pg.ConstraintName, "לא ניתן לבצע – קיימות רשומות קשורות")
			return v
		case "23514", "22P02":
			v := domain.NewValidation()
			v.Add(pg.ConstraintName, "ערך לא תקין")
			return v
		}
	}
	return err
}

func collect[T any](rows pgx.Rows, err error, scan func(pgx.Row) (T, error)) ([]T, error) {
	if err != nil {
		return nil, mapErr(err)
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		v, err := scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, mapErr(rows.Err())
}

// where accumulates SQL conditions with positional arguments.
type where struct {
	conds []string
	args  []any
}

func (w *where) add(cond string, arg any) {
	w.args = append(w.args, arg)
	w.conds = append(w.conds, fmt.Sprintf(cond, len(w.args)))
}

func (w *where) raw(cond string) { w.conds = append(w.conds, cond) }

func (w *where) sql() string {
	if len(w.conds) == 0 {
		return ""
	}
	out := " WHERE " + w.conds[0]
	for _, c := range w.conds[1:] {
		out += " AND " + c
	}
	return out
}

func (w *where) next(arg any) string {
	w.args = append(w.args, arg)
	return fmt.Sprintf("$%d", len(w.args))
}
