package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/notify"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type RequestInput struct {
	RoomID    uuid.UUID `json:"roomId"`
	Title     string    `json:"title"`
	EventKind string    `json:"eventKind"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	Attendees int       `json:"attendees"`
	Equipment []string  `json:"equipment"`
	Notes     string    `json:"notes"`
	Phone     string    `json:"phone"`
	Name      string    `json:"name"` // שם המזמין (defaults to the profile name)
	// Route: "rental" (paid, default) or "activity" (פעילות באמצעות רכז).
	Route       string     `json:"route"`
	ActivityID  *uuid.UUID `json:"activityId"`
	TablesCount int        `json:"tablesCount"`
	ChairsCount int        `json:"chairsCount"`
}

func (s *Service) ListRequests(ctx context.Context, actor Actor, f store.RequestFilter) ([]domain.BookingRequest, error) {
	if !actor.Can(domain.ActReviewRequests) {
		uid := actor.User.ID
		f.UserID = &uid // regular users only see their own
	}
	list, err := s.store.ListRequests(ctx, f)
	if err != nil {
		return nil, err
	}
	if !actor.Can(domain.ActReviewRequests) {
		for i := range list {
			list[i].InternalNotes = ""
		}
	}
	return list, nil
}

func (s *Service) SubmitRequest(ctx context.Context, actor Actor, in RequestInput) (domain.BookingRequest, error) {
	if err := actor.Require(domain.ActSubmitRequest); err != nil {
		return domain.BookingRequest{}, err
	}
	in.Title = strings.TrimSpace(in.Title)
	v := domain.NewValidation()
	if in.Attendees < 0 || in.TablesCount < 0 || in.ChairsCount < 0 {
		v.Add("attendees", "מספר לא תקין")
	}
	switch in.Route {
	case "", "rental":
		in.Route = "rental"
	case "activity":
		if !actor.Can(domain.ActRequestActivity) {
			return domain.BookingRequest{}, domain.ErrForbidden
		}
		if in.ActivityID != nil {
			act, err := s.store.ActivityByID(ctx, *in.ActivityID)
			if err != nil || act.CoordinatorID == nil || *act.CoordinatorID != actor.User.ID {
				v.Add("activityId", "ניתן לבחור רק פעילות שבאחריותך")
			} else if in.Title == "" {
				in.Title = act.Name
			}
		}
	default:
		v.Add("route", "מסלול לא תקין")
	}
	if in.Title == "" {
		v.Add("title", "יש להזין שם לאירוע")
	}
	in.Name, in.Phone = strings.TrimSpace(in.Name), strings.TrimSpace(in.Phone)
	if in.Name == "" {
		in.Name = actor.User.FullName
	}
	if in.Phone == "" && actor.User.Phone != nil {
		in.Phone = *actor.User.Phone
	}
	if p := notify.NormalizeIsraeliPhone(in.Phone); len(p) < 12 || len(p) > 13 {
		v.Add("phone", "יש להזין מספר טלפון תקין")
	}
	if err := domain.ValidateWindow(in.StartsAt, in.EndsAt); err != nil {
		var ve *domain.ValidationError
		errors.As(err, &ve)
		for k, m := range ve.Fields {
			v.Add(k, m)
		}
	} else if in.StartsAt.Before(s.now()) {
		v.Add("startsAt", "לא ניתן להזמין מועד שעבר")
	}
	room, err := s.store.RoomByID(ctx, in.RoomID)
	if err != nil {
		v.Add("roomId", "יש לבחור חדר")
		return domain.BookingRequest{}, v
	}
	if !room.IsActive || !room.IsBookable {
		v.Add("roomId", "חדר זה אינו זמין להזמנה")
	}
	if room.Capacity > 0 && in.Attendees > room.Capacity {
		v.Add("attendees", fmt.Sprintf("קיבולת החדר היא %d משתתפים", room.Capacity))
	}
	if b, err := s.store.BuildingByID(ctx, room.BuildingID); err == nil && len(v.Fields) == 0 &&
		!domain.WithinOpeningHours(in.StartsAt, in.EndsAt, b.OpensAt, b.ClosesAt, s.loc) {
		v.Add("startsAt", fmt.Sprintf("שעות הפעילות הן %s–%s", b.OpensAt, b.ClosesAt))
	}
	if err := v.OrNil(); err != nil {
		return domain.BookingRequest{}, err
	}
	if conflicts, err := s.CheckConflicts(ctx, room.ID, in.StartsAt, in.EndsAt, uuid.Nil); err != nil {
		return domain.BookingRequest{}, err
	} else if len(conflicts) > 0 {
		v.Add("startsAt", "המועד שנבחר כבר תפוס – אנא בחרו מועד אחר")
		return domain.BookingRequest{}, v
	}

	// Activities run through a coordinator are not charged automatically.
	var quote domain.PriceQuote
	if in.Route == "rental" {
		if quote, err = s.QuoteBooking(ctx, room.ID, domain.BookingRental, in.StartsAt, in.EndsAt, in.Equipment); err != nil {
			return domain.BookingRequest{}, err
		}
	}
	var customerID *uuid.UUID
	if in.Route == "rental" {
		cust, err := s.customerForUser(ctx, actor.User, in.Phone)
		if err != nil {
			return domain.BookingRequest{}, err
		}
		customerID = &cust.ID
	}
	req, err := s.store.CreateRequest(ctx, domain.BookingRequest{
		UserID: actor.User.ID, CustomerID: customerID, RoomID: room.ID, Title: in.Title, EventKind: in.EventKind,
		StartsAt: in.StartsAt, EndsAt: in.EndsAt, Attendees: in.Attendees, Equipment: in.Equipment,
		Notes: in.Notes, QuotedPrice: quote.Total, Route: in.Route, ActivityID: in.ActivityID,
		TablesCount: in.TablesCount, ChairsCount: in.ChairsCount, ContactName: in.Name, ContactPhone: in.Phone,
	})
	if err != nil {
		return domain.BookingRequest{}, err
	}
	s.audit(ctx, actor, "request", req.ID, "submit", map[string]any{"title": req.Title, "room": req.RoomName})
	s.emit(ctx, EvRequestChanged, map[string]any{"action": "created", "id": req.ID}, realtime.TopicStaff)
	return req, nil
}

// customerForUser finds or creates the customer record linked to a portal user.
func (s *Service) customerForUser(ctx context.Context, u domain.User, phone string) (domain.Customer, error) {
	c, err := s.store.CustomerByUserID(ctx, u.ID)
	if err == nil {
		return c, nil
	}
	if !errors.Is(err, domain.ErrNotFound) {
		return c, err
	}
	p := u.Phone
	if phone = strings.TrimSpace(phone); phone != "" {
		p = &phone
	}
	email := u.Email
	return s.store.CreateCustomer(ctx, store.CustomerInput{
		Name: u.FullName, Kind: "private", ContactName: &u.FullName, Phone: p, Email: &email, UserID: &u.ID,
	})
}

type ReviewDecision struct {
	Approve         bool       `json:"approve"`
	FinalPrice      *float64   `json:"finalPrice"`
	InternalNotes   string     `json:"internalNotes"`
	RejectionReason string     `json:"rejectionReason"`
	CoordinatorID   *uuid.UUID `json:"coordinatorId"`
}

// ReviewRequest approves (creating a confirmed booking after a fresh
// availability check) or rejects a portal request.
func (s *Service) ReviewRequest(ctx context.Context, actor Actor, id uuid.UUID, d ReviewDecision) (domain.BookingRequest, error) {
	if err := actor.Require(domain.ActReviewRequests); err != nil {
		return domain.BookingRequest{}, err
	}
	if !d.Approve && strings.TrimSpace(d.RejectionReason) == "" {
		v := domain.NewValidation()
		v.Add("rejectionReason", "יש לציין סיבת דחייה")
		return domain.BookingRequest{}, v
	}
	if d.FinalPrice != nil && *d.FinalPrice < 0 {
		v := domain.NewValidation()
		v.Add("finalPrice", "מחיר לא תקין")
		return domain.BookingRequest{}, v
	}

	var booking *domain.Booking
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		req, err := tx.RequestByIDForUpdate(ctx, id)
		if err != nil {
			return err
		}
		if req.Status != domain.RequestPending {
			v := domain.NewValidation()
			v.Add("status", "הבקשה כבר טופלה")
			return v
		}
		in := store.ReviewInput{
			Status: domain.RequestRejected, InternalNotes: d.InternalNotes,
			RejectionReason: strings.TrimSpace(d.RejectionReason), ReviewedBy: actor.User.ID,
		}
		if d.Approve {
			room, err := tx.RoomByID(ctx, req.RoomID)
			if err != nil {
				return err
			}
			price := req.QuotedPrice
			if d.FinalPrice != nil {
				price = *d.FinalPrice
			}
			b := domain.Booking{
				RoomID: req.RoomID, Title: req.Title, Type: domain.BookingRental, Status: domain.StatusConfirmed,
				StartsAt: req.StartsAt, EndsAt: req.EndsAt, BufferMinutes: room.BufferMinutes,
				CustomerID: req.CustomerID, CoordinatorID: d.CoordinatorID, RequestID: &req.ID,
				Attendees: req.Attendees, Equipment: req.Equipment, Price: price,
				Notes: req.Notes, InternalNotes: d.InternalNotes, CreatedBy: actor.ID(),
				TablesCount: req.TablesCount, ChairsCount: req.ChairsCount,
			}
			if req.Route == "activity" {
				// The requester is the coordinator running the activity.
				b.Type, b.CoordinatorID, b.ActivityID = domain.BookingActivity, &req.UserID, req.ActivityID
			}
			existing, err := tx.BlockingBookings(ctx, b.RoomID, b.StartsAt.Add(-4*time.Hour), b.BlockedUntil())
			if err != nil {
				return err
			}
			if c := domain.FindConflicts(b, existing); len(c) > 0 {
				return &domain.ConflictError{Conflicts: c}
			}
			created, err := tx.CreateBooking(ctx, b)
			if errors.Is(err, store.ErrOverlap) {
				return &domain.ConflictError{}
			}
			if err != nil {
				return err
			}
			booking = &created
			in.Status, in.FinalPrice, in.BookingID, in.RejectionReason = domain.RequestApproved, &price, &created.ID, ""
		}
		return tx.ReviewRequest(ctx, id, in)
	})
	if err != nil {
		return domain.BookingRequest{}, err
	}

	req, err := s.store.RequestByID(ctx, id)
	if err != nil {
		return req, err
	}
	when := req.StartsAt.In(s.loc).Format("02/01/2006 15:04")
	if requester, err := s.store.UserByID(ctx, req.UserID); err == nil {
		if d.Approve {
			msg := fmt.Sprintf("שלום %s,\nבקשתך \"%s\" ב%s בתאריך %s אושרה.", requester.FullName, req.Title, req.RoomName, when)
			if req.FinalPrice != nil && *req.FinalPrice > 0 {
				msg += fmt.Sprintf("\nסכום לתשלום: ₪%.2f", *req.FinalPrice)
			}
			s.notifyUser(requester, "בקשתך אושרה ✅", msg)
		} else {
			s.notifyUser(requester, "עדכון לגבי בקשתך",
				fmt.Sprintf("שלום %s,\nלצערנו בקשתך \"%s\" בתאריך %s לא אושרה.\nסיבה: %s", requester.FullName, req.Title, when, req.RejectionReason))
		}
	}
	action := "reject"
	if d.Approve {
		action = "approve"
	}
	s.audit(ctx, actor, "request", id, action, map[string]any{"title": req.Title, "reason": req.RejectionReason, "price": req.FinalPrice})
	s.emit(ctx, EvRequestChanged, map[string]any{"action": action, "id": id}, realtime.TopicStaff, realtime.UserTopic(req.UserID.String()))
	if booking != nil {
		s.bookingChanged(ctx, "created", *booking)
	}
	return req, nil
}

func (s *Service) CancelOwnRequest(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := s.store.CancelRequest(ctx, id, actor.User.ID); err != nil {
		return err
	}
	s.audit(ctx, actor, "request", id, "cancel", nil)
	s.emit(ctx, EvRequestChanged, map[string]any{"action": "cancelled", "id": id}, realtime.TopicStaff)
	return nil
}

// ── Portal availability search ───────────────────────────────

type AvailabilityQuery struct {
	Date        time.Time
	From, To    string // optional HH:MM window wanted
	MinCapacity int
	Kind        string
	BuildingID  *uuid.UUID
}

type RoomAvailability struct {
	Room          domain.Room       `json:"room"`
	FreeSlots     []domain.Interval `json:"freeSlots"`
	WindowFree    *bool             `json:"windowFree"`
	EstimatedCost *float64          `json:"estimatedCost"`
}

func (s *Service) SearchAvailability(ctx context.Context, q AvailabilityQuery) ([]RoomAvailability, error) {
	day := s.startOfDay(q.Date)
	rooms, err := s.store.ListRooms(ctx, store.RoomFilter{BuildingID: q.BuildingID, OnlyActive: true})
	if err != nil {
		return nil, err
	}
	buildings, err := s.store.ListBuildings(ctx)
	if err != nil {
		return nil, err
	}
	hours := map[uuid.UUID]domain.Building{}
	for _, b := range buildings {
		hours[b.ID] = b
	}
	bookings, err := s.store.ListBookings(ctx, store.BookingFilter{From: day, To: day.AddDate(0, 0, 1), Statuses: []string{"pending", "confirmed"}})
	if err != nil {
		return nil, err
	}
	byRoom := map[uuid.UUID][]domain.Booking{}
	for _, b := range bookings {
		byRoom[b.RoomID] = append(byRoom[b.RoomID], b)
	}

	var wStart, wEnd time.Time
	if q.From != "" && q.To != "" {
		wStart, _ = clockOn(day, q.From, s.loc)
		wEnd, _ = clockOn(day, q.To, s.loc)
	}

	out := []RoomAvailability{}
	for _, r := range rooms {
		if !r.IsBookable || (q.MinCapacity > 0 && r.Capacity < q.MinCapacity) || (q.Kind != "" && r.Kind != q.Kind) {
			continue
		}
		b := hours[r.BuildingID]
		open, _ := clockOn(day, b.OpensAt, s.loc)
		closeAt, _ := clockOn(day, b.ClosesAt, s.loc)
		if !closeAt.After(open) {
			closeAt = closeAt.Add(24 * time.Hour)
		}
		if now := s.now(); open.Before(now) {
			open = now.Truncate(15 * time.Minute).Add(15 * time.Minute)
		}
		busy := domain.BusyIntervals(byRoom[r.ID])
		ra := RoomAvailability{Room: r, FreeSlots: domain.FreeSlots(open, closeAt, busy, 30*time.Minute)}
		if !wStart.IsZero() && wEnd.After(wStart) {
			cand := domain.Booking{RoomID: r.ID, StartsAt: wStart, EndsAt: wEnd, BufferMinutes: r.BufferMinutes, Status: domain.StatusPending}
			free := len(domain.FindConflicts(cand, byRoom[r.ID])) == 0 && domain.WithinOpeningHours(wStart, wEnd, b.OpensAt, b.ClosesAt, s.loc) && wStart.After(s.now())
			ra.WindowFree = &free
			if q, err := s.QuoteBooking(ctx, r.ID, domain.BookingRental, wStart, wEnd, nil); err == nil {
				ra.EstimatedCost = &q.Total
			}
		}
		out = append(out, ra)
	}
	return out, nil
}

func clockOn(day time.Time, hhmm string, loc *time.Location) (time.Time, error) {
	t, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(day.Year(), day.Month(), day.Day(), t.Hour(), t.Minute(), 0, 0, loc), nil
}
