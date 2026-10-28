package domain

import (
	"testing"
	"time"

	"github.com/google/uuid"
)

var (
	roomA = uuid.MustParse("00000000-0000-0000-0000-00000000000a")
	roomB = uuid.MustParse("00000000-0000-0000-0000-00000000000b")
	day   = time.Date(2026, 10, 15, 0, 0, 0, 0, time.UTC)
)

func at(h, m int) time.Time {
	return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

func booking(room uuid.UUID, sh, sm, eh, em, buffer int) Booking {
	return Booking{
		ID: uuid.New(), RoomID: room, Title: "x", Type: BookingRental, Status: StatusConfirmed,
		StartsAt: at(sh, sm), EndsAt: at(eh, em), BufferMinutes: buffer,
	}
}

func TestFindConflicts(t *testing.T) {
	existing := booking(roomA, 10, 0, 12, 0, 15)
	maint := booking(roomA, 18, 0, 20, 0, 0)
	maint.Type = BookingMaintenance
	cancelled := booking(roomA, 13, 0, 14, 0, 0)
	cancelled.Status = StatusCancelled
	all := []Booking{existing, maint, cancelled}

	tests := []struct {
		name   string
		cand   Booking
		want   int
		reason ConflictReason
	}{
		{"clear overlap", booking(roomA, 11, 0, 13, 0, 0), 1, ConflictOverlap},
		{"contained", booking(roomA, 10, 30, 11, 0, 0), 1, ConflictOverlap},
		{"inside previous buffer", booking(roomA, 12, 10, 13, 0, 0), 1, ConflictBuffer},
		{"right after buffer", booking(roomA, 12, 15, 13, 0, 0), 0, ""},
		{"candidate buffer hits next", booking(roomA, 9, 0, 9, 50, 15), 1, ConflictBuffer},
		{"touching without buffer", booking(roomA, 9, 0, 10, 0, 0), 0, ""},
		{"cancelled ignored", booking(roomA, 13, 0, 14, 0, 0), 0, ""},
		{"other room", booking(roomB, 10, 0, 12, 0, 0), 0, ""},
		{"maintenance block", booking(roomA, 19, 0, 21, 0, 0), 1, ConflictMaintenance},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := FindConflicts(tt.cand, all)
			if len(got) != tt.want {
				t.Fatalf("got %d conflicts, want %d: %+v", len(got), tt.want, got)
			}
			if tt.want > 0 && got[0].Reason != tt.reason {
				t.Errorf("reason = %s, want %s", got[0].Reason, tt.reason)
			}
		})
	}
}

func TestFindConflictsIgnoresSelf(t *testing.T) {
	b := booking(roomA, 10, 0, 12, 0, 15)
	moved := b
	moved.StartsAt, moved.EndsAt = at(11, 0), at(13, 0)
	if got := FindConflicts(moved, []Booking{b}); len(got) != 0 {
		t.Fatalf("editing a booking must not conflict with itself, got %+v", got)
	}
}

func TestFindConflictsSorted(t *testing.T) {
	late := booking(roomA, 14, 0, 15, 0, 0)
	early := booking(roomA, 9, 0, 10, 0, 0)
	got := FindConflicts(booking(roomA, 8, 0, 16, 0, 0), []Booking{late, early})
	if len(got) != 2 || !got[0].StartsAt.Equal(early.StartsAt) {
		t.Fatalf("conflicts not sorted by start: %+v", got)
	}
}

func TestValidateWindow(t *testing.T) {
	cases := []struct {
		name  string
		s, e  time.Time
		valid bool
	}{
		{"ok", at(10, 0), at(11, 0), true},
		{"end before start", at(11, 0), at(10, 0), false},
		{"too short", at(10, 0), at(10, 10), false},
		{"too long", at(0, 0), at(0, 0).Add(25 * time.Hour), false},
		{"zero", time.Time{}, at(10, 0), false},
	}
	for _, c := range cases {
		if err := ValidateWindow(c.s, c.e); (err == nil) != c.valid {
			t.Errorf("%s: err=%v valid=%v", c.name, err, c.valid)
		}
	}
}

func TestWithinOpeningHours(t *testing.T) {
	loc := time.UTC
	if !WithinOpeningHours(at(8, 0), at(22, 0), "07:00", "23:00", loc) {
		t.Error("inside hours should pass")
	}
	if WithinOpeningHours(at(6, 30), at(8, 0), "07:00", "23:00", loc) {
		t.Error("before opening should fail")
	}
	if WithinOpeningHours(at(22, 0), at(23, 30), "07:00", "23:00", loc) {
		t.Error("after closing should fail")
	}
	if !WithinOpeningHours(at(22, 0), at(23, 59).Add(31*time.Minute), "08:00", "01:00", loc) {
		t.Error("closing after midnight should allow late events")
	}
}
