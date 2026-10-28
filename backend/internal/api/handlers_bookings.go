package api

import (
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

func (s *Server) listBookings(w http.ResponseWriter, r *http.Request) {
	f := store.BookingFilter{
		From: qTime(r, "from"), To: qTime(r, "to"),
		RoomIDs: qUUIDs(r, "roomIds"), BuildingID: qUUID(r, "buildingId"),
		CoordinatorID: qUUID(r, "coordinatorId"), CustomerID: qUUID(r, "customerId"),
		ActivityID: qUUID(r, "activityId"),
		Types:      qList(r, "types"), Statuses: qList(r, "statuses"),
		Search: r.URL.Query().Get("q"), Limit: qInt(r, "limit", 0),
	}
	if f.From.IsZero() && f.To.IsZero() && f.Search == "" && f.CustomerID == nil {
		f.From = time.Now().AddDate(0, 0, -7)
		f.To = time.Now().AddDate(0, 0, 31)
	}
	list, err := s.svc.ListBookings(r.Context(), mustActor(r), f)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, list)
}

func (s *Server) getBooking(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	b, err := s.svc.GetBooking(r.Context(), mustActor(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, b)
}

func (s *Server) createBooking(w http.ResponseWriter, r *http.Request) {
	var in service.BookingInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CreateBooking(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, res)
}

func (s *Server) updateBooking(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in service.BookingInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	b, err := s.svc.UpdateBooking(r.Context(), mustActor(r), id, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, b)
}

func (s *Server) moveBooking(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in struct {
		RoomID   uuid.UUID `json:"roomId"`
		StartsAt time.Time `json:"startsAt"`
		EndsAt   time.Time `json:"endsAt"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	b, err := s.svc.MoveBooking(r.Context(), mustActor(r), id, in.RoomID, in.StartsAt, in.EndsAt)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, b)
}

func (s *Server) cancelBooking(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.CancelBooking(r.Context(), mustActor(r), id, r.URL.Query().Get("scope")); err != nil {
		fail(w, r, err)
		return
	}
	noContent(w)
}

func (s *Server) deleteBooking(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.DeleteBooking(r.Context(), mustActor(r), id); err != nil {
		fail(w, r, err)
		return
	}
	noContent(w)
}

// checkBooking previews conflicts and price while the user edits a booking.
func (s *Server) checkBooking(w http.ResponseWriter, r *http.Request) {
	var in struct {
		RoomID    uuid.UUID          `json:"roomId"`
		StartsAt  time.Time          `json:"startsAt"`
		EndsAt    time.Time          `json:"endsAt"`
		ExcludeID uuid.UUID          `json:"excludeId"`
		Type      domain.BookingType `json:"type"`
		Equipment []string           `json:"equipment"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := domain.ValidateWindow(in.StartsAt, in.EndsAt); err != nil {
		fail(w, r, err)
		return
	}
	conflicts, err := s.svc.CheckConflicts(r.Context(), in.RoomID, in.StartsAt, in.EndsAt, in.ExcludeID)
	if err != nil {
		fail(w, r, err)
		return
	}
	if in.Type == "" {
		in.Type = domain.BookingRental
	}
	quote, err := s.svc.QuoteBooking(r.Context(), in.RoomID, in.Type, in.StartsAt, in.EndsAt, in.Equipment)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"conflicts": conflicts, "quote": quote})
}

// ── Requests ─────────────────────────────────────────────────

func (s *Server) listRequests(w http.ResponseWriter, r *http.Request) {
	list, err := s.svc.ListRequests(r.Context(), mustActor(r), store.RequestFilter{
		Status: r.URL.Query().Get("status"), Limit: qInt(r, "limit", 0), UserID: qUUID(r, "userId"),
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, list)
}

func (s *Server) submitRequest(w http.ResponseWriter, r *http.Request) {
	var in service.RequestInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	req, err := s.svc.SubmitRequest(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, req)
}

func (s *Server) reviewRequest(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in service.ReviewDecision
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	req, err := s.svc.ReviewRequest(r.Context(), mustActor(r), id, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, req)
}

func (s *Server) cancelRequest(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.CancelOwnRequest(r.Context(), mustActor(r), id); err != nil {
		fail(w, r, err)
		return
	}
	noContent(w)
}

func (s *Server) availability(w http.ResponseWriter, r *http.Request) {
	date := qTime(r, "date")
	if date.IsZero() {
		date = time.Now()
	} else {
		// A bare YYYY-MM-DD parses as UTC midnight; reinterpret it in local time.
		date = time.Date(date.Year(), date.Month(), date.Day(), 12, 0, 0, 0, s.cfg.Location)
	}
	res, err := s.svc.SearchAvailability(r.Context(), service.AvailabilityQuery{
		Date: date, From: r.URL.Query().Get("from"), To: r.URL.Query().Get("to"),
		MinCapacity: qInt(r, "capacity", 0), Kind: r.URL.Query().Get("kind"), BuildingID: qUUID(r, "buildingId"),
	})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// ── Dashboard & reports ──────────────────────────────────────

func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	d, err := s.svc.Dashboard(r.Context(), mustActor(r), qUUID(r, "buildingId"))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, d)
}

func (s *Server) report(w http.ResponseWriter, r *http.Request) {
	from, to := qTime(r, "from"), qTime(r, "to")
	if from.IsZero() || to.IsZero() || !to.After(from) {
		now := time.Now().In(s.cfg.Location)
		from = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.cfg.Location).AddDate(0, -5, 0)
		to = time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.cfg.Location).AddDate(0, 1, 0)
	}
	rep, err := s.svc.Report(r.Context(), mustActor(r), from, to)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, rep)
}
