package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// Block is one occupied period as seen by a coordinator. Other people's
// bookings only expose their type (and coordinator for activities) — never
// customers, prices or notes. The caller's own bookings and pending
// requests are returned in full.
type Block struct {
	ID              string               `json:"id"`
	RoomID          uuid.UUID            `json:"roomId"`
	RoomName        string               `json:"roomName"`
	BuildingID      uuid.UUID            `json:"buildingId"`
	StartsAt        time.Time            `json:"startsAt"`
	EndsAt          time.Time            `json:"endsAt"`
	Type            domain.BookingType   `json:"type"`
	Status          domain.BookingStatus `json:"status"`
	Title           string               `json:"title"`
	CoordinatorName *string              `json:"coordinatorName"`
	Mine            bool                 `json:"mine"`
	Request         bool                 `json:"request"` // a pending request of mine (does not block others)
	SeriesID        *uuid.UUID           `json:"seriesId"`
	CancelRequested bool                 `json:"cancelRequested"`
}

var publicTypeLabel = map[domain.BookingType]string{
	domain.BookingActivity:    "פעילות",
	domain.BookingRental:      "שכירות",
	domain.BookingInternal:    "פעילות פנימית",
	domain.BookingMaintenance: "לא זמין",
	domain.BookingClosed:      "לא זמין",
}

// blockLabel is what a coordinator sees on someone else's booking.
func blockLabel(b domain.Booking) string {
	label := publicTypeLabel[b.Type]
	if b.Type != domain.BookingMaintenance && b.Type != domain.BookingClosed && b.CoordinatorName != nil {
		label += " · רכז: " + *b.CoordinatorName
	}
	return label
}

func (s *Service) Availability(ctx context.Context, actor Actor, from, to time.Time, buildingID *uuid.UUID, roomIDs []uuid.UUID) ([]Block, error) {
	if !actor.Can(domain.ActViewAvailability) && !actor.Can(domain.ActViewCalendar) {
		return nil, domain.ErrForbidden
	}
	if to.Sub(from) > 62*24*time.Hour || !to.After(from) {
		v := domain.NewValidation()
		v.Add("to", "טווח תאריכים לא תקין (עד חודשיים)")
		return nil, v
	}
	bookings, err := s.store.ListBookings(ctx, store.BookingFilter{
		From: from, To: to, BuildingID: buildingID, RoomIDs: roomIDs, Statuses: []string{"pending", "confirmed"},
	})
	if err != nil {
		return nil, err
	}
	me := actor.User.ID
	out := make([]Block, 0, len(bookings))
	for _, b := range bookings {
		mine := b.CoordinatorID != nil && *b.CoordinatorID == me
		blk := Block{
			ID: b.ID.String(), RoomID: b.RoomID, RoomName: b.RoomName, BuildingID: b.BuildingID,
			StartsAt: b.StartsAt, EndsAt: b.EndsAt, Type: b.Type, Status: b.Status, Mine: mine,
			CoordinatorName: b.CoordinatorName, SeriesID: b.SeriesID, CancelRequested: b.CancelRequestedAt != nil,
		}
		if mine || actor.Can(domain.ActViewCalendar) {
			blk.Title = b.Title
		} else {
			blk.Title = blockLabel(b)
			blk.SeriesID = nil
			if b.Type == domain.BookingMaintenance || b.Type == domain.BookingClosed {
				blk.CoordinatorName = nil
			}
		}
		out = append(out, blk)
	}

	// My own pending requests, so I can see what I already asked for.
	reqs, err := s.store.ListRequests(ctx, store.RequestFilter{Status: "pending", UserID: &me})
	if err != nil {
		return nil, err
	}
	for _, r := range reqs {
		if !r.EndsAt.After(from) || !r.StartsAt.Before(to) || (len(roomIDs) > 0 && !containsID(roomIDs, r.RoomID)) {
			continue
		}
		typ := domain.BookingRental
		if r.Route == "activity" {
			typ = domain.BookingActivity
		}
		out = append(out, Block{
			ID: "request:" + r.ID.String(), RoomID: r.RoomID, RoomName: r.RoomName, StartsAt: r.StartsAt, EndsAt: r.EndsAt,
			Type: typ, Status: domain.StatusPending, Title: "בקשה ממתינה: " + r.Title, Mine: true, Request: true,
		})
	}
	return out, nil
}

func containsID(ids []uuid.UUID, id uuid.UUID) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

// ── Cancellation requests (owner asks, manager decides) ───────

// ownsBooking reports whether the actor is the coordinator of the booking or
// the portal customer it was made for.
func (s *Service) ownsBooking(ctx context.Context, actor Actor, b domain.Booking) bool {
	if b.CoordinatorID != nil && *b.CoordinatorID == actor.User.ID {
		return true
	}
	if b.CustomerID != nil {
		if c, err := s.store.CustomerByUserID(ctx, actor.User.ID); err == nil && c.ID == *b.CustomerID {
			return true
		}
	}
	return false
}

func (s *Service) RequestCancellation(ctx context.Context, actor Actor, id uuid.UUID, reason string) error {
	b, err := s.store.BookingByID(ctx, id)
	if err != nil {
		return err
	}
	if !s.ownsBooking(ctx, actor, b) {
		return domain.ErrForbidden
	}
	v := domain.NewValidation()
	switch {
	case !b.Status.Blocks():
		v.Add("id", "ההזמנה כבר בוטלה")
	case b.CancelRequestedAt != nil:
		v.Add("id", "כבר נשלחה בקשת ביטול להזמנה זו")
	case strings.TrimSpace(reason) == "":
		v.Add("reason", "יש לציין סיבת ביטול")
	}
	if err := v.OrNil(); err != nil {
		return err
	}
	if err := s.store.SetCancelRequest(ctx, id, &actor.User.ID, strings.TrimSpace(reason)); err != nil {
		return err
	}
	s.audit(ctx, actor, "booking", id, "cancel_request", map[string]any{"title": b.Title, "reason": reason})
	s.emit(ctx, EvRequestChanged, map[string]any{"action": "cancel_requested", "bookingId": id}, realtime.TopicStaff)
	s.emit(ctx, EvBookingChanged, map[string]any{"action": "cancel_requested", "ids": []uuid.UUID{id}}, realtime.TopicStaff)
	return nil
}

// ResolveCancellation approves (cancels the booking, freeing the room) or
// declines an owner's cancellation request. Until then the room stays blocked.
func (s *Service) ResolveCancellation(ctx context.Context, actor Actor, id uuid.UUID, approve bool, note string) error {
	if err := actor.Require(domain.ActReviewRequests); err != nil {
		return err
	}
	b, err := s.store.BookingByID(ctx, id)
	if err != nil {
		return err
	}
	if b.CancelRequestedAt == nil {
		v := domain.NewValidation()
		v.Add("id", "אין בקשת ביטול פתוחה להזמנה זו")
		return v
	}
	requester := b.CancelRequestedBy
	if approve {
		err = s.store.SetBookingStatus(ctx, id, domain.StatusCancelled)
	} else {
		err = s.store.SetCancelRequest(ctx, id, nil, "")
	}
	if err != nil {
		return err
	}
	action := "cancel_declined"
	if approve {
		action = "cancel_approved"
	}
	s.audit(ctx, actor, "booking", id, action, map[string]any{"title": b.Title, "note": note})
	if requester != nil {
		if u, err := s.store.UserByID(ctx, *requester); err == nil {
			when := b.StartsAt.In(s.loc).Format("02/01/2006 15:04")
			if approve {
				s.notifyUser(u, "בקשת הביטול אושרה", fmt.Sprintf("ההזמנה \"%s\" (%s) בוטלה והחדר שוחרר.", b.Title, when))
			} else {
				msg := fmt.Sprintf("בקשת הביטול להזמנה \"%s\" (%s) לא אושרה – ההזמנה בתוקף.", b.Title, when)
				if note = strings.TrimSpace(note); note != "" {
					msg += "\n" + note
				}
				s.notifyUser(u, "עדכון לגבי בקשת הביטול", msg)
			}
		}
	}
	b.Status = domain.StatusCancelled
	s.bookingChanged(ctx, action, b)
	s.emit(ctx, EvRequestChanged, map[string]any{"action": action, "bookingId": id}, realtime.TopicStaff)
	return nil
}
