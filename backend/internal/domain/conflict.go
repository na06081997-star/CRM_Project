package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

type ConflictReason string

const (
	ConflictOverlap     ConflictReason = "overlap"     // time ranges intersect
	ConflictBuffer      ConflictReason = "buffer"      // only the turnover buffer intersects
	ConflictMaintenance ConflictReason = "maintenance" // room blocked for maintenance / closed
)

type Conflict struct {
	BookingID uuid.UUID      `json:"bookingId"`
	Title     string         `json:"title"`
	StartsAt  time.Time      `json:"startsAt"`
	EndsAt    time.Time      `json:"endsAt"`
	Reason    ConflictReason `json:"reason"`
}

// BlockedUntil is the end of the period during which the room is unavailable
// because of b: its end time plus the turnover buffer.
func (b Booking) BlockedUntil() time.Time {
	return b.EndsAt.Add(time.Duration(b.BufferMinutes) * time.Minute)
}

// FindConflicts returns every existing booking that would collide with the
// candidate. The rule mirrors the database exclusion constraint: a booking
// occupies [start, end+buffer) and only pending/confirmed bookings block.
func FindConflicts(candidate Booking, existing []Booking) []Conflict {
	out := []Conflict{}
	for _, e := range existing {
		if e.ID == candidate.ID || e.RoomID != candidate.RoomID || !e.Status.Blocks() {
			continue
		}
		if !(candidate.StartsAt.Before(e.BlockedUntil()) && e.StartsAt.Before(candidate.BlockedUntil())) {
			continue
		}
		reason := ConflictBuffer
		switch {
		case e.Type == BookingMaintenance || e.Type == BookingClosed:
			reason = ConflictMaintenance
		case candidate.StartsAt.Before(e.EndsAt) && e.StartsAt.Before(candidate.EndsAt):
			reason = ConflictOverlap
		}
		out = append(out, Conflict{BookingID: e.ID, Title: e.Title, StartsAt: e.StartsAt, EndsAt: e.EndsAt, Reason: reason})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartsAt.Before(out[j].StartsAt) })
	return out
}

const (
	MinBookingDuration = 15 * time.Minute
	MaxBookingDuration = 24 * time.Hour
)

// ValidateWindow checks a booking's time range independently of other bookings.
func ValidateWindow(start, end time.Time) error {
	v := NewValidation()
	switch {
	case start.IsZero() || end.IsZero():
		v.Add("startsAt", "יש לבחור שעת התחלה וסיום")
	case !end.After(start):
		v.Add("endsAt", "שעת הסיום חייבת להיות אחרי שעת ההתחלה")
	case end.Sub(start) < MinBookingDuration:
		v.Add("endsAt", "משך הזמנה מינימלי הוא 15 דקות")
	case end.Sub(start) > MaxBookingDuration:
		v.Add("endsAt", "משך הזמנה מקסימלי הוא 24 שעות")
	}
	return v.OrNil()
}

// WithinOpeningHours reports whether [start,end) falls inside the building's
// daily opening hours (HH:MM strings) on the start day, in loc.
func WithinOpeningHours(start, end time.Time, opens, closes string, loc *time.Location) bool {
	o, err1 := time.Parse("15:04", opens)
	c, err2 := time.Parse("15:04", closes)
	if err1 != nil || err2 != nil {
		return true
	}
	s := start.In(loc)
	day := time.Date(s.Year(), s.Month(), s.Day(), 0, 0, 0, 0, loc)
	openAt := day.Add(time.Duration(o.Hour())*time.Hour + time.Duration(o.Minute())*time.Minute)
	closeAt := day.Add(time.Duration(c.Hour())*time.Hour + time.Duration(c.Minute())*time.Minute)
	if !closeAt.After(openAt) { // closes after midnight
		closeAt = closeAt.Add(24 * time.Hour)
	}
	return !start.Before(openAt) && !end.After(closeAt)
}
