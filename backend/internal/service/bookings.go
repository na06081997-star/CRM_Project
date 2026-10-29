package service

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type BookingInput struct {
	RoomID        uuid.UUID            `json:"roomId"`
	Title         string               `json:"title"`
	Type          domain.BookingType   `json:"type"`
	Status        domain.BookingStatus `json:"status"`
	StartsAt      time.Time            `json:"startsAt"`
	EndsAt        time.Time            `json:"endsAt"`
	ActivityID    *uuid.UUID           `json:"activityId"`
	CoordinatorID *uuid.UUID           `json:"coordinatorId"`
	CustomerID    *uuid.UUID           `json:"customerId"`
	Attendees     int                  `json:"attendees"`
	Equipment     []string             `json:"equipment"`
	Price         *float64             `json:"price"` // nil → computed quote
	Notes         string               `json:"notes"`
	InternalNotes string               `json:"internalNotes"`
	// BufferMinutes overrides the room default when set.
	BufferMinutes *int `json:"bufferMinutes"`
	// Weekly recurrence (create only).
	RepeatWeeks int        `json:"repeatWeeks"`
	RepeatUntil *time.Time `json:"repeatUntil"`
	// SkipConflicts creates the non-conflicting occurrences of a series.
	SkipConflicts bool `json:"skipConflicts"`
}

func (in *BookingInput) normalize() error {
	in.Title = strings.TrimSpace(in.Title)
	v := domain.NewValidation()
	if in.Title == "" {
		v.Add("title", "יש להזין כותרת")
	}
	if in.RoomID == uuid.Nil {
		v.Add("roomId", "יש לבחור חדר")
	}
	if in.Type == "" {
		in.Type = domain.BookingInternal
	}
	if !in.Type.Valid() {
		v.Add("type", "סוג הזמנה לא תקין")
	}
	if in.Status == "" {
		in.Status = domain.StatusConfirmed
	}
	if !in.Status.Blocks() && in.Status != domain.StatusCancelled {
		v.Add("status", "סטטוס לא תקין")
	}
	if in.Attendees < 0 {
		v.Add("attendees", "מספר משתתפים לא תקין")
	}
	if in.Price != nil && *in.Price < 0 {
		v.Add("price", "מחיר לא תקין")
	}
	if in.RepeatWeeks < 0 || in.RepeatWeeks > 52 {
		v.Add("repeatWeeks", "ניתן לחזור עד 52 שבועות")
	}
	if err := domain.ValidateWindow(in.StartsAt, in.EndsAt); err != nil {
		var ve *domain.ValidationError
		if errors.As(err, &ve) {
			for k, m := range ve.Fields {
				v.Add(k, m)
			}
		}
	}
	return v.OrNil()
}

func (s *Service) ListBookings(ctx context.Context, actor Actor, f store.BookingFilter) ([]domain.Booking, error) {
	switch {
	case actor.Can(domain.ActViewCalendar):
	case actor.Can(domain.ActSubmitRequest):
		// Coordinators and portal users list only their own bookings.
		me := actor.User.ID
		f.OwnerUserID = &me
	default:
		return nil, domain.ErrForbidden
	}
	list, err := s.store.ListBookings(ctx, f)
	if err != nil {
		return nil, err
	}
	if !actor.Can(domain.ActManageBookings) { // non-managers never see the money or internal notes
		for i := range list {
			list[i].InternalNotes, list[i].Price, list[i].PaidAmount = "", 0, 0
		}
	}
	return list, nil
}

func (s *Service) GetBooking(ctx context.Context, actor Actor, id uuid.UUID) (domain.Booking, error) {
	if err := actor.Require(domain.ActViewCalendar); err != nil {
		return domain.Booking{}, err
	}
	return s.store.BookingByID(ctx, id)
}

// QuoteBooking prices a prospective booking.
func (s *Service) QuoteBooking(ctx context.Context, roomID uuid.UUID, typ domain.BookingType, start, end time.Time, equipment []string) (domain.PriceQuote, error) {
	room, err := s.store.RoomByID(ctx, roomID)
	if err != nil {
		return domain.PriceQuote{}, err
	}
	items, err := s.store.ListEquipment(ctx)
	if err != nil {
		return domain.PriceQuote{}, err
	}
	prices, labels := map[string]float64{}, map[string]string{}
	for _, it := range items {
		prices[it.Code], labels[it.Code] = it.FlatPrice, it.Label
	}
	return domain.Quote(domain.PriceInput{
		Type: typ, HourlyRate: room.HourlyRate, MinHours: room.MinHours,
		Start: start, End: end, Equipment: equipment, EquipmentPrices: prices, EquipmentLabels: labels,
	}), nil
}

// CheckConflicts previews collisions for a prospective booking (excludeID = the booking being edited).
func (s *Service) CheckConflicts(ctx context.Context, roomID uuid.UUID, start, end time.Time, excludeID uuid.UUID) ([]domain.Conflict, error) {
	room, err := s.store.RoomByID(ctx, roomID)
	if err != nil {
		return nil, err
	}
	cand := domain.Booking{ID: excludeID, RoomID: roomID, StartsAt: start, EndsAt: end, BufferMinutes: room.BufferMinutes, Status: domain.StatusConfirmed}
	existing, err := s.store.ListBookings(ctx, store.BookingFilter{
		RoomIDs: []uuid.UUID{roomID}, From: start.Add(-4 * time.Hour), To: cand.BlockedUntil(),
		Statuses: []string{"pending", "confirmed"},
	})
	if err != nil {
		return nil, err
	}
	return domain.FindConflicts(cand, existing), nil
}

type CreateResult struct {
	Created []domain.Booking  `json:"created"`
	Skipped []domain.Conflict `json:"skipped"`
}

func (s *Service) CreateBooking(ctx context.Context, actor Actor, in BookingInput) (CreateResult, error) {
	if err := actor.Require(domain.ActManageBookings); err != nil {
		return CreateResult{}, err
	}
	if err := in.normalize(); err != nil {
		return CreateResult{}, err
	}
	room, err := s.store.RoomByID(ctx, in.RoomID)
	if err != nil {
		return CreateResult{}, err
	}

	until := time.Time{}
	if in.RepeatUntil != nil {
		until = *in.RepeatUntil
	}
	weeks := in.RepeatWeeks
	if weeks == 0 {
		weeks = 1
	}
	starts := domain.RecurrenceDates(in.StartsAt, until, weeks, 52)
	duration := in.EndsAt.Sub(in.StartsAt)

	var seriesID *uuid.UUID
	if len(starts) > 1 {
		id := uuid.New()
		seriesID = &id
	}

	price := 0.0
	if in.Price != nil {
		price = *in.Price
	} else if q, err := s.QuoteBooking(ctx, room.ID, in.Type, in.StartsAt, in.EndsAt, in.Equipment); err == nil {
		price = q.Total
	}
	buffer := room.BufferMinutes
	if in.BufferMinutes != nil {
		buffer = *in.BufferMinutes
	}

	var res CreateResult
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		for _, st := range starts {
			b := domain.Booking{
				RoomID: room.ID, Title: in.Title, Type: in.Type, Status: in.Status,
				StartsAt: st, EndsAt: st.Add(duration), BufferMinutes: buffer,
				ActivityID: in.ActivityID, CoordinatorID: in.CoordinatorID, CustomerID: in.CustomerID,
				SeriesID: seriesID, Attendees: in.Attendees, Equipment: in.Equipment, Price: price,
				Notes: in.Notes, InternalNotes: in.InternalNotes, CreatedBy: actor.ID(),
			}
			existing, err := tx.BlockingBookings(ctx, room.ID, b.StartsAt.Add(-4*time.Hour), b.BlockedUntil())
			if err != nil {
				return err
			}
			if conflicts := domain.FindConflicts(b, existing); len(conflicts) > 0 && b.Status.Blocks() {
				if in.SkipConflicts {
					res.Skipped = append(res.Skipped, conflicts...)
					continue
				}
				return &domain.ConflictError{Conflicts: conflicts}
			}
			created, err := tx.CreateBooking(ctx, b)
			if errors.Is(err, store.ErrOverlap) {
				return &domain.ConflictError{}
			}
			if err != nil {
				return err
			}
			res.Created = append(res.Created, created)
		}
		return nil
	})
	if err != nil {
		return CreateResult{}, err
	}
	for _, b := range res.Created {
		s.audit(ctx, actor, "booking", b.ID, "create", map[string]any{"title": b.Title, "room": b.RoomName, "startsAt": b.StartsAt})
	}
	s.bookingChanged(ctx, "created", res.Created...)
	return res, nil
}

func (s *Service) UpdateBooking(ctx context.Context, actor Actor, id uuid.UUID, in BookingInput) (domain.Booking, error) {
	if err := actor.Require(domain.ActManageBookings); err != nil {
		return domain.Booking{}, err
	}
	if err := in.normalize(); err != nil {
		return domain.Booking{}, err
	}
	var updated domain.Booking
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		cur, err := tx.BookingByID(ctx, id)
		if err != nil {
			return err
		}
		room, err := tx.RoomByID(ctx, in.RoomID)
		if err != nil {
			return err
		}
		b := cur
		b.RoomID, b.Title, b.Type, b.Status = in.RoomID, in.Title, in.Type, in.Status
		b.StartsAt, b.EndsAt = in.StartsAt, in.EndsAt
		b.ActivityID, b.CoordinatorID, b.CustomerID = in.ActivityID, in.CoordinatorID, in.CustomerID
		b.Attendees, b.Equipment, b.Notes, b.InternalNotes = in.Attendees, in.Equipment, in.Notes, in.InternalNotes
		if in.Price != nil {
			b.Price = *in.Price
		}
		if in.BufferMinutes != nil {
			b.BufferMinutes = *in.BufferMinutes
		} else if b.RoomID != cur.RoomID {
			b.BufferMinutes = room.BufferMinutes
		}
		if b.Status.Blocks() {
			existing, err := tx.BlockingBookings(ctx, b.RoomID, b.StartsAt.Add(-4*time.Hour), b.BlockedUntil())
			if err != nil {
				return err
			}
			if c := domain.FindConflicts(b, existing); len(c) > 0 {
				return &domain.ConflictError{Conflicts: c}
			}
		}
		updated, err = tx.UpdateBooking(ctx, b)
		if errors.Is(err, store.ErrOverlap) {
			return &domain.ConflictError{}
		}
		return err
	})
	if err != nil {
		return domain.Booking{}, err
	}
	s.audit(ctx, actor, "booking", id, "update", map[string]any{"title": updated.Title, "startsAt": updated.StartsAt, "status": updated.Status})
	s.bookingChanged(ctx, "updated", updated)
	return updated, nil
}

// MoveBooking changes only time and room (drag & drop in the calendar).
func (s *Service) MoveBooking(ctx context.Context, actor Actor, id uuid.UUID, roomID uuid.UUID, start, end time.Time) (domain.Booking, error) {
	cur, err := s.store.BookingByID(ctx, id)
	if err != nil {
		return domain.Booking{}, err
	}
	price := cur.Price
	in := BookingInput{
		RoomID: roomID, Title: cur.Title, Type: cur.Type, Status: cur.Status, StartsAt: start, EndsAt: end,
		ActivityID: cur.ActivityID, CoordinatorID: cur.CoordinatorID, CustomerID: cur.CustomerID,
		Attendees: cur.Attendees, Equipment: cur.Equipment, Price: &price, Notes: cur.Notes, InternalNotes: cur.InternalNotes,
	}
	return s.UpdateBooking(ctx, actor, id, in)
}

func (s *Service) CancelBooking(ctx context.Context, actor Actor, id uuid.UUID, scope string) error {
	if err := actor.Require(domain.ActManageBookings); err != nil {
		return err
	}
	b, err := s.store.BookingByID(ctx, id)
	if err != nil {
		return err
	}
	targets := []domain.Booking{b}
	if scope == "series" && b.SeriesID != nil {
		targets, err = s.store.ListBookings(ctx, store.BookingFilter{SeriesID: b.SeriesID, From: b.StartsAt.Add(-time.Minute)})
		if err != nil {
			return err
		}
	}
	err = s.store.InTx(ctx, func(tx *store.Store) error {
		for _, t := range targets {
			if err := tx.SetBookingStatus(ctx, t.ID, domain.StatusCancelled); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	for i := range targets {
		targets[i].Status = domain.StatusCancelled
		s.audit(ctx, actor, "booking", targets[i].ID, "cancel", map[string]any{"title": targets[i].Title})
	}
	s.bookingChanged(ctx, "cancelled", targets...)
	return nil
}

func (s *Service) DeleteBooking(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := actor.Require(domain.ActManageBookings); err != nil {
		return err
	}
	b, err := s.store.BookingByID(ctx, id)
	if err != nil {
		return err
	}
	if b.PaidAmount > 0 {
		v := domain.NewValidation()
		v.Add("id", "לא ניתן למחוק הזמנה עם תשלומים – יש לבטל אותה")
		return v
	}
	if err := s.store.DeleteBooking(ctx, id); err != nil {
		return err
	}
	s.audit(ctx, actor, "booking", id, "delete", map[string]any{"title": b.Title})
	s.bookingChanged(ctx, "deleted", b)
	return nil
}

func (s *Service) bookingChanged(ctx context.Context, action string, bs ...domain.Booking) {
	if len(bs) == 0 {
		return
	}
	ids := make([]uuid.UUID, len(bs))
	for i, b := range bs {
		ids[i] = b.ID
	}
	s.emit(ctx, EvBookingChanged, map[string]any{"action": action, "ids": ids}, realtime.TopicStaff, realtime.TopicAvailability)
	s.emit(ctx, EvSignageChanged, nil, realtime.TopicSignage)
}
