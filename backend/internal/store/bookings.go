package store

import (
	"context"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const bookingSelect = `
	SELECT b.id, b.room_id, b.title, b.type, b.status, b.starts_at, b.ends_at, b.buffer_minutes,
	       b.activity_id, b.coordinator_id, b.customer_id, b.request_id, b.series_id, b.attendees,
	       b.equipment, b.tables_count, b.chairs_count, b.price, b.notes, b.internal_notes, b.created_by, b.created_at,
	       b.cancel_requested_at, b.cancel_requested_by, b.cancel_request_reason,
	       r.name, r.building_id, co.full_name, cu.name,
	       COALESCE((SELECT sum(p.amount) FROM payments p WHERE p.booking_id = b.id), 0)
	FROM bookings b
	JOIN rooms r ON r.id = b.room_id
	LEFT JOIN users co ON co.id = b.coordinator_id
	LEFT JOIN customers cu ON cu.id = b.customer_id`

func scanBooking(r pgx.Row) (domain.Booking, error) {
	var b domain.Booking
	err := r.Scan(&b.ID, &b.RoomID, &b.Title, &b.Type, &b.Status, &b.StartsAt, &b.EndsAt, &b.BufferMinutes,
		&b.ActivityID, &b.CoordinatorID, &b.CustomerID, &b.RequestID, &b.SeriesID, &b.Attendees,
		&b.Equipment, &b.TablesCount, &b.ChairsCount, &b.Price, &b.Notes, &b.InternalNotes, &b.CreatedBy, &b.CreatedAt,
		&b.CancelRequestedAt, &b.CancelRequestedBy, &b.CancelRequestReason,
		&b.RoomName, &b.BuildingID, &b.CoordinatorName, &b.CustomerName, &b.PaidAmount)
	return b, mapErr(err)
}

type BookingFilter struct {
	From, To        time.Time
	RoomIDs         []uuid.UUID
	BuildingID      *uuid.UUID
	CoordinatorID   *uuid.UUID
	CustomerID      *uuid.UUID
	ActivityID      *uuid.UUID
	SeriesID        *uuid.UUID
	Types           []string
	Statuses        []string
	Search          string
	OnlyWithBalance bool
	CancelRequested bool
	// OwnerUserID limits to bookings the user runs (coordinator) or booked (portal customer).
	OwnerUserID *uuid.UUID
	Limit       int
}

func (s *Store) ListBookings(ctx context.Context, f BookingFilter) ([]domain.Booking, error) {
	w := &where{}
	if !f.From.IsZero() {
		w.add("b.ends_at > $%d", f.From)
	}
	if !f.To.IsZero() {
		w.add("b.starts_at < $%d", f.To)
	}
	if len(f.RoomIDs) > 0 {
		w.add("b.room_id = ANY($%d)", f.RoomIDs)
	}
	if f.BuildingID != nil {
		w.add("r.building_id = $%d", *f.BuildingID)
	}
	if f.CoordinatorID != nil {
		w.add("b.coordinator_id = $%d", *f.CoordinatorID)
	}
	if f.CustomerID != nil {
		w.add("b.customer_id = $%d", *f.CustomerID)
	}
	if f.OwnerUserID != nil {
		p := w.next(*f.OwnerUserID)
		w.raw("(b.coordinator_id = " + p + " OR cu.user_id = " + p + ")")
	}
	if f.ActivityID != nil {
		w.add("b.activity_id = $%d", *f.ActivityID)
	}
	if f.SeriesID != nil {
		w.add("b.series_id = $%d", *f.SeriesID)
	}
	if len(f.Types) > 0 {
		w.add("b.type::text = ANY($%d)", f.Types)
	}
	if len(f.Statuses) > 0 {
		w.add("b.status::text = ANY($%d)", f.Statuses)
	}
	if q := strings.TrimSpace(f.Search); q != "" {
		p := w.next("%" + q + "%")
		w.raw("(b.title ILIKE " + p + " OR cu.name ILIKE " + p + " OR co.full_name ILIKE " + p + " OR r.name ILIKE " + p + ")")
	}
	if f.CancelRequested {
		w.raw("b.cancel_requested_at IS NOT NULL AND b.status IN ('pending', 'confirmed')")
	}
	if f.OnlyWithBalance {
		w.raw("b.price > COALESCE((SELECT sum(p.amount) FROM payments p WHERE p.booking_id = b.id), 0)")
		w.raw("b.status IN ('pending', 'confirmed')")
	}
	sql := bookingSelect + w.sql() + ` ORDER BY b.starts_at`
	if f.Limit > 0 {
		sql += " LIMIT " + w.next(f.Limit)
	}
	rows, err := s.q.Query(ctx, sql, w.args...)
	return collect(rows, err, scanBooking)
}

func (s *Store) BookingByID(ctx context.Context, id uuid.UUID) (domain.Booking, error) {
	return scanBooking(s.q.QueryRow(ctx, bookingSelect+` WHERE b.id = $1`, id))
}

// BlockingBookings returns the room's pending/confirmed bookings whose blocked
// range intersects [from, to), locking them so concurrent writers serialise.
func (s *Store) BlockingBookings(ctx context.Context, roomID uuid.UUID, from, to time.Time) ([]domain.Booking, error) {
	rows, err := s.q.Query(ctx, bookingSelect+`
		WHERE b.room_id = $1 AND b.status IN ('pending', 'confirmed')
		  AND b.blocked_range && tstzrange($2, $3, '[)')
		ORDER BY b.starts_at
		FOR UPDATE OF b`, roomID, from, to)
	return collect(rows, err, scanBooking)
}

func (s *Store) CreateBooking(ctx context.Context, b domain.Booking) (domain.Booking, error) {
	var id uuid.UUID
	err := s.q.QueryRow(ctx, `
		INSERT INTO bookings (room_id, title, type, status, starts_at, ends_at, buffer_minutes, activity_id,
		                      coordinator_id, customer_id, request_id, series_id, attendees, equipment, price,
		                      notes, internal_notes, created_by, tables_count, chairs_count)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20)
		RETURNING id`,
		b.RoomID, b.Title, b.Type, b.Status, b.StartsAt, b.EndsAt, b.BufferMinutes, b.ActivityID,
		b.CoordinatorID, b.CustomerID, b.RequestID, b.SeriesID, b.Attendees, nonNil(b.Equipment), b.Price,
		b.Notes, b.InternalNotes, b.CreatedBy, b.TablesCount, b.ChairsCount).Scan(&id)
	if err != nil {
		return domain.Booking{}, mapErr(err)
	}
	return s.BookingByID(ctx, id)
}

func (s *Store) UpdateBooking(ctx context.Context, b domain.Booking) (domain.Booking, error) {
	tag, err := s.q.Exec(ctx, `
		UPDATE bookings SET room_id = $2, title = $3, type = $4, status = $5, starts_at = $6, ends_at = $7,
		       buffer_minutes = $8, activity_id = $9, coordinator_id = $10, customer_id = $11, attendees = $12,
		       equipment = $13, price = $14, notes = $15, internal_notes = $16, tables_count = $17, chairs_count = $18
		WHERE id = $1`,
		b.ID, b.RoomID, b.Title, b.Type, b.Status, b.StartsAt, b.EndsAt, b.BufferMinutes, b.ActivityID,
		b.CoordinatorID, b.CustomerID, b.Attendees, nonNil(b.Equipment), b.Price, b.Notes, b.InternalNotes, b.TablesCount, b.ChairsCount)
	if err != nil {
		return domain.Booking{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Booking{}, domain.ErrNotFound
	}
	return s.BookingByID(ctx, b.ID)
}

func (s *Store) SetBookingStatus(ctx context.Context, id uuid.UUID, status domain.BookingStatus) error {
	tag, err := s.q.Exec(ctx, `UPDATE bookings SET status = $2,
		cancel_requested_at = CASE WHEN $2::booking_status = 'cancelled' THEN NULL ELSE cancel_requested_at END
		WHERE id = $1`, id, status)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}

func (s *Store) DeleteBooking(ctx context.Context, id uuid.UUID) error {
	tag, err := s.q.Exec(ctx, `DELETE FROM bookings WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}

// SetCancelRequest records (by != nil) or clears (by == nil) an owner's
// request to cancel a booking.
func (s *Store) SetCancelRequest(ctx context.Context, id uuid.UUID, by *uuid.UUID, reason string) error {
	tag, err := s.q.Exec(ctx, `
		UPDATE bookings SET
		       cancel_requested_at = CASE WHEN $2::uuid IS NULL THEN NULL ELSE now() END,
		       cancel_requested_by = $2, cancel_request_reason = $3
		WHERE id = $1`, id, by, reason)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}
