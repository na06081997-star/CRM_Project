package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const requestSelect = `
	SELECT q.id, q.user_id, q.customer_id, q.room_id, q.route, q.activity_id, q.tables_count, q.chairs_count, q.title, q.event_kind, q.starts_at, q.ends_at, q.attendees,
	       q.equipment, q.notes, q.status, q.quoted_price, q.final_price, q.internal_notes, q.rejection_reason,
	       q.reviewed_by, q.reviewed_at, q.booking_id, q.created_at,
	       r.name, u.full_name, u.email, u.phone, u.role, a.name, bk.status::text, (bk.cancel_requested_at IS NOT NULL),
	       (q.status = 'pending' AND EXISTS (
	            SELECT 1 FROM bookings b
	            WHERE b.room_id = q.room_id AND b.status IN ('pending', 'confirmed')
	              AND b.blocked_range && tstzrange(q.starts_at, q.ends_at + make_interval(mins => r.buffer_minutes), '[)')))
	FROM booking_requests q
	JOIN rooms r ON r.id = q.room_id
	JOIN users u ON u.id = q.user_id
	LEFT JOIN activities a ON a.id = q.activity_id
	LEFT JOIN bookings bk ON bk.id = q.booking_id`

func scanRequest(r pgx.Row) (domain.BookingRequest, error) {
	var q domain.BookingRequest
	err := r.Scan(&q.ID, &q.UserID, &q.CustomerID, &q.RoomID, &q.Route, &q.ActivityID, &q.TablesCount, &q.ChairsCount, &q.Title, &q.EventKind, &q.StartsAt, &q.EndsAt,
		&q.Attendees, &q.Equipment, &q.Notes, &q.Status, &q.QuotedPrice, &q.FinalPrice, &q.InternalNotes,
		&q.RejectionReason, &q.ReviewedBy, &q.ReviewedAt, &q.BookingID, &q.CreatedAt,
		&q.RoomName, &q.RequesterName, &q.RequesterEmail, &q.RequesterPhone, &q.RequesterRole, &q.ActivityName,
		&q.BookingStatus, &q.CancelRequested, &q.HasConflict)
	return q, mapErr(err)
}

type RequestFilter struct {
	Status string
	UserID *uuid.UUID
	Limit  int
}

func (s *Store) ListRequests(ctx context.Context, f RequestFilter) ([]domain.BookingRequest, error) {
	w := &where{}
	if f.Status != "" {
		w.add("q.status = $%d", f.Status)
	}
	if f.UserID != nil {
		w.add("q.user_id = $%d", *f.UserID)
	}
	sql := requestSelect + w.sql() + ` ORDER BY (q.status = 'pending') DESC, q.created_at DESC`
	if f.Limit > 0 {
		sql += " LIMIT " + w.next(f.Limit)
	}
	rows, err := s.q.Query(ctx, sql, w.args...)
	return collect(rows, err, scanRequest)
}

func (s *Store) RequestByID(ctx context.Context, id uuid.UUID) (domain.BookingRequest, error) {
	return scanRequest(s.q.QueryRow(ctx, requestSelect+` WHERE q.id = $1`, id))
}

func (s *Store) RequestByIDForUpdate(ctx context.Context, id uuid.UUID) (domain.BookingRequest, error) {
	if _, err := s.q.Exec(ctx, `SELECT 1 FROM booking_requests WHERE id = $1 FOR UPDATE`, id); err != nil {
		return domain.BookingRequest{}, mapErr(err)
	}
	return s.RequestByID(ctx, id)
}

func (s *Store) CreateRequest(ctx context.Context, q domain.BookingRequest) (domain.BookingRequest, error) {
	if q.Route == "" {
		q.Route = "rental"
	}
	var id uuid.UUID
	err := s.q.QueryRow(ctx, `
		INSERT INTO booking_requests (user_id, customer_id, room_id, title, event_kind, starts_at, ends_at,
		                              attendees, equipment, notes, quoted_price, route, activity_id, tables_count, chairs_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) RETURNING id`,
		q.UserID, q.CustomerID, q.RoomID, q.Title, q.EventKind, q.StartsAt, q.EndsAt, q.Attendees,
		nonNil(q.Equipment), q.Notes, q.QuotedPrice, q.Route, q.ActivityID, q.TablesCount, q.ChairsCount).Scan(&id)
	if err != nil {
		return domain.BookingRequest{}, mapErr(err)
	}
	return s.RequestByID(ctx, id)
}

type ReviewInput struct {
	Status          domain.RequestStatus
	FinalPrice      *float64
	InternalNotes   string
	RejectionReason string
	ReviewedBy      uuid.UUID
	BookingID       *uuid.UUID
}

func (s *Store) ReviewRequest(ctx context.Context, id uuid.UUID, in ReviewInput) error {
	_, err := s.q.Exec(ctx, `
		UPDATE booking_requests SET status = $2, final_price = $3, internal_notes = $4, rejection_reason = $5,
		       reviewed_by = $6, reviewed_at = $7, booking_id = $8, updated_at = now()
		WHERE id = $1`,
		id, in.Status, in.FinalPrice, in.InternalNotes, in.RejectionReason, in.ReviewedBy, time.Now(), in.BookingID)
	return mapErr(err)
}

func (s *Store) CancelRequest(ctx context.Context, id, userID uuid.UUID) error {
	tag, err := s.q.Exec(ctx, `
		UPDATE booking_requests SET status = 'cancelled', updated_at = now()
		WHERE id = $1 AND user_id = $2 AND status = 'pending'`, id, userID)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}
