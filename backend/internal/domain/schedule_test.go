package domain

import (
	"testing"
	"time"
)

func TestOccurrences(t *testing.T) {
	loc, err := time.LoadLocation("Asia/Jerusalem")
	if err != nil {
		t.Skip("tzdata unavailable")
	}
	s := TaskSchedule{TimeOfDay: "07:30", Weekdays: []int{0, 3}, IsActive: true} // Sun, Wed
	from := time.Date(2026, 10, 25, 0, 0, 0, 0, loc)                             // Sunday
	to := from.AddDate(0, 0, 14)
	got := Occurrences(s, from, to, loc)
	if len(got) != 4 {
		t.Fatalf("want 4 occurrences in two weeks, got %d: %v", len(got), got)
	}
	for _, g := range got {
		if g.Hour() != 7 || g.Minute() != 30 {
			t.Errorf("wrong time of day %v", g)
		}
		if wd := g.Weekday(); wd != time.Sunday && wd != time.Wednesday {
			t.Errorf("wrong weekday %v", wd)
		}
	}
}

func TestOccurrencesInactiveOrInvalid(t *testing.T) {
	from := at(0, 0)
	to := from.AddDate(0, 0, 7)
	if got := Occurrences(TaskSchedule{TimeOfDay: "07:30", Weekdays: []int{1}}, from, to, time.UTC); got != nil {
		t.Error("inactive schedule must not produce occurrences")
	}
	if got := Occurrences(TaskSchedule{TimeOfDay: "bad", Weekdays: []int{1}, IsActive: true}, from, to, time.UTC); got != nil {
		t.Error("invalid time must not produce occurrences")
	}
}

func TestOccurrencesExcludesPastTimeOnFirstDay(t *testing.T) {
	s := TaskSchedule{TimeOfDay: "07:00", Weekdays: []int{0, 1, 2, 3, 4, 5, 6}, IsActive: true}
	got := Occurrences(s, at(9, 0), at(9, 0).Add(24*time.Hour), time.UTC)
	if len(got) != 1 || got[0].Day() != at(9, 0).Day()+1 {
		t.Fatalf("today's 07:00 is already past at 09:00: %v", got)
	}
}

func TestRecurrenceDates(t *testing.T) {
	first := at(17, 0)
	if got := RecurrenceDates(first, time.Time{}, 4, 52); len(got) != 4 {
		t.Errorf("4 weeks → %d dates", len(got))
	}
	until := first.AddDate(0, 0, 21)
	got := RecurrenceDates(first, until, 0, 52)
	if len(got) != 4 || !got[3].Equal(until) {
		t.Errorf("until inclusive → %v", got)
	}
	if got := RecurrenceDates(first, first.AddDate(5, 0, 0), 0, 10); len(got) != 10 {
		t.Errorf("max cap → %d", len(got))
	}
}
