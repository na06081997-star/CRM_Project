package service

import (
	"context"
	"sort"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// MaxOccurrences caps a weekly series (one year).
const MaxOccurrences = 52

type SlotQuery struct {
	RoomID    uuid.UUID `json:"roomId"`
	StartsAt  time.Time `json:"startsAt"`
	EndsAt    time.Time `json:"endsAt"`
	ExcludeID uuid.UUID `json:"excludeId"`
	Attendees int       `json:"attendees"`
	// RepeatUntil (YYYY-MM-DD) checks every weekly occurrence up to that date.
	RepeatUntil string `json:"repeatUntil"`
}

type Occurrence struct {
	StartsAt time.Time `json:"startsAt"`
	EndsAt   time.Time `json:"endsAt"`
	Free     bool      `json:"free"`
}

type RoomOption struct {
	RoomID       uuid.UUID `json:"roomId"`
	RoomName     string    `json:"roomName"`
	BuildingName string    `json:"buildingName"`
	Capacity     int       `json:"capacity"`
}

// SlotCheck answers "can I have this room then?" and, when not, what else is free.
type SlotCheck struct {
	Free          bool              `json:"free"`
	Conflicts     []domain.Conflict `json:"conflicts"`
	OutsideHours  bool              `json:"outsideHours"`
	Hours         string            `json:"hours"` // HH:MM–HH:MM opening hours of the building
	InPast        bool              `json:"inPast"`
	BufferMinutes int               `json:"bufferMinutes"`
	// Alternatives, filled when the wanted slot is not free.
	SameRoom   []domain.Interval `json:"sameRoom"`
	OtherRooms []RoomOption      `json:"otherRooms"`
	// Weekly series (when RepeatUntil is set): every occurrence and whether it is free.
	Occurrences []Occurrence `json:"occurrences,omitempty"`
}

// CheckSlot validates a wanted slot against bookings (with turnover buffers),
// opening hours and the clock, and suggests free alternatives: other times the
// same day in that room, and other rooms free at the very same time.
func (s *Service) CheckSlot(ctx context.Context, actor Actor, q SlotQuery) (SlotCheck, error) {
	if !actor.Can(domain.ActViewAvailability) && !actor.Can(domain.ActViewCalendar) {
		return SlotCheck{}, domain.ErrForbidden
	}
	if err := domain.ValidateWindow(q.StartsAt, q.EndsAt); err != nil {
		return SlotCheck{}, err
	}
	room, err := s.store.RoomByID(ctx, q.RoomID)
	if err != nil {
		return SlotCheck{}, err
	}
	building, err := s.store.BuildingByID(ctx, room.BuildingID)
	if err != nil {
		return SlotCheck{}, err
	}
	res := SlotCheck{BufferMinutes: room.BufferMinutes, Hours: building.OpensAt + "–" + building.ClosesAt}
	if res.Conflicts, err = s.CheckConflicts(ctx, room.ID, q.StartsAt, q.EndsAt, q.ExcludeID); err != nil {
		return res, err
	}
	if !actor.Can(domain.ActViewCalendar) {
		// Others' bookings stay anonymous outside the managers' calendar.
		for i := range res.Conflicts {
			res.Conflicts[i].Title, res.Conflicts[i].BookingID = "", uuid.Nil
		}
	}
	res.OutsideHours = !domain.WithinOpeningHours(q.StartsAt, q.EndsAt, building.OpensAt, building.ClosesAt, s.loc)
	res.InPast = q.StartsAt.Before(s.now())
	res.Free = len(res.Conflicts) == 0 && !res.OutsideHours && !res.InPast

	if q.RepeatUntil != "" {
		until, err := s.parseUntil(q.RepeatUntil, q.StartsAt)
		if err != nil {
			return res, err
		}
		if res.Occurrences, err = s.occurrences(ctx, room, q.StartsAt, q.EndsAt, until, q.ExcludeID); err != nil {
			return res, err
		}
	}

	if !res.Free {
		if res.SameRoom, err = s.sameRoomAlternatives(ctx, room, building, q); err != nil {
			return res, err
		}
		if res.OtherRooms, err = s.otherRoomAlternatives(ctx, room, q); err != nil {
			return res, err
		}
	}
	return res, nil
}

// parseUntil reads a YYYY-MM-DD end date of a weekly series and returns the
// last moment of that day; it must not precede the first occurrence nor be
// more than a year after it.
func (s *Service) parseUntil(v string, first time.Time) (time.Time, error) {
	d, err := time.ParseInLocation("2006-01-02", v, s.loc)
	val := domain.NewValidation()
	switch {
	case err != nil:
		val.Add("repeatUntil", "תאריך סיום לא תקין")
	case d.Before(s.startOfDay(first)):
		val.Add("repeatUntil", "תאריך הסיום חייב להיות אחרי המפגש הראשון")
	case d.After(first.AddDate(1, 0, 0)):
		val.Add("repeatUntil", "ניתן לקבוע חוג חוזר לשנה לכל היותר")
	}
	if err := val.OrNil(); err != nil {
		return time.Time{}, err
	}
	return d.Add(24*time.Hour - time.Second), nil
}

// occurrences lists the weekly dates of a series and whether each is free.
func (s *Service) occurrences(ctx context.Context, room domain.Room, start, end, until time.Time, exclude uuid.UUID) ([]Occurrence, error) {
	dates := domain.RecurrenceDates(start, until, 0, MaxOccurrences)
	length := end.Sub(start)
	existing, err := s.store.ListBookings(ctx, store.BookingFilter{
		RoomIDs: []uuid.UUID{room.ID}, From: start.Add(-4 * time.Hour), To: dates[len(dates)-1].Add(length + 4*time.Hour),
		Statuses: []string{"pending", "confirmed"},
	})
	if err != nil {
		return nil, err
	}
	out := make([]Occurrence, 0, len(dates))
	for _, st := range dates {
		cand := domain.Booking{ID: exclude, RoomID: room.ID, StartsAt: st, EndsAt: st.Add(length), BufferMinutes: room.BufferMinutes, Status: domain.StatusPending}
		out = append(out, Occurrence{StartsAt: st, EndsAt: st.Add(length), Free: len(domain.FindConflicts(cand, existing)) == 0})
	}
	return out, nil
}

func (s *Service) sameRoomAlternatives(ctx context.Context, room domain.Room, b domain.Building, q SlotQuery) ([]domain.Interval, error) {
	day := s.startOfDay(q.StartsAt)
	open, err1 := clockOn(day, b.OpensAt, s.loc)
	closeAt, err2 := clockOn(day, b.ClosesAt, s.loc)
	if err1 != nil || err2 != nil {
		return nil, nil
	}
	if !closeAt.After(open) {
		closeAt = closeAt.Add(24 * time.Hour)
	}
	bookings, err := s.store.ListBookings(ctx, store.BookingFilter{
		RoomIDs: []uuid.UUID{room.ID}, From: open.Add(-4 * time.Hour), To: closeAt, Statuses: []string{"pending", "confirmed"},
	})
	if err != nil {
		return nil, err
	}
	kept := bookings[:0]
	for _, x := range bookings {
		if x.ID != q.ExcludeID {
			kept = append(kept, x)
		}
	}
	notBefore := s.now().Truncate(15 * time.Minute).Add(15 * time.Minute)
	want := domain.Interval{Start: q.StartsAt, End: q.EndsAt}
	return domain.SuggestSlots(want, time.Duration(room.BufferMinutes)*time.Minute, domain.BusyIntervals(kept), open, closeAt, notBefore, 15*time.Minute, 4), nil
}

// otherRoomAlternatives returns up to four other rooms free at the wanted
// time, large enough, same building first.
func (s *Service) otherRoomAlternatives(ctx context.Context, room domain.Room, q SlotQuery) ([]RoomOption, error) {
	if q.StartsAt.Before(s.now()) {
		return nil, nil
	}
	rooms, err := s.store.ListRooms(ctx, store.RoomFilter{OnlyActive: true})
	if err != nil {
		return nil, err
	}
	buildings, err := s.store.ListBuildings(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[uuid.UUID]domain.Building{}
	for _, b := range buildings {
		byID[b.ID] = b
	}
	sort.SliceStable(rooms, func(i, j int) bool {
		return rooms[i].BuildingID == room.BuildingID && rooms[j].BuildingID != room.BuildingID
	})
	out := []RoomOption{}
	for _, r := range rooms {
		if len(out) == 4 {
			break
		}
		if r.ID == room.ID || !r.IsBookable || (q.Attendees > 0 && r.Capacity > 0 && r.Capacity < q.Attendees) {
			continue
		}
		b := byID[r.BuildingID]
		if !domain.WithinOpeningHours(q.StartsAt, q.EndsAt, b.OpensAt, b.ClosesAt, s.loc) {
			continue
		}
		conflicts, err := s.CheckConflicts(ctx, r.ID, q.StartsAt, q.EndsAt, uuid.Nil)
		if err != nil {
			return nil, err
		}
		if len(conflicts) == 0 {
			out = append(out, RoomOption{RoomID: r.ID, RoomName: r.Name, BuildingName: b.Name, Capacity: r.Capacity})
		}
	}
	return out, nil
}
