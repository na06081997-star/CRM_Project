package domain

import (
	"testing"
	"time"
)

func TestFreeSlots(t *testing.T) {
	busy := []Interval{
		{at(12, 0), at(13, 0)},
		{at(9, 0), at(10, 0)},
		{at(9, 30), at(10, 30)}, // overlaps previous
		{at(16, 0), at(16, 10)},
	}
	got := FreeSlots(at(8, 0), at(18, 0), busy, 30*time.Minute)
	want := []Interval{
		{at(8, 0), at(9, 0)},
		{at(10, 30), at(12, 0)},
		{at(13, 0), at(16, 0)},
		{at(16, 10), at(18, 0)},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d slots, want %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		if !got[i].Start.Equal(want[i].Start) || !got[i].End.Equal(want[i].End) {
			t.Errorf("slot %d = %v–%v, want %v–%v", i, got[i].Start, got[i].End, want[i].Start, want[i].End)
		}
	}
}

func TestFreeSlotsDropsShortGaps(t *testing.T) {
	busy := []Interval{{at(8, 20), at(17, 45)}}
	got := FreeSlots(at(8, 0), at(18, 0), busy, 30*time.Minute)
	if len(got) != 0 {
		t.Fatalf("gaps shorter than 30m must be dropped, got %+v", got)
	}
}

func TestBusyIntervalsIncludesBuffer(t *testing.T) {
	b := booking(roomA, 10, 0, 11, 0, 15)
	c := booking(roomA, 12, 0, 13, 0, 0)
	c.Status = StatusRejected
	got := BusyIntervals([]Booking{b, c})
	if len(got) != 1 || !got[0].End.Equal(at(11, 15)) {
		t.Fatalf("unexpected busy intervals %+v", got)
	}
}

func TestSuggestSlotsAvoidsBuffers(t *testing.T) {
	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	at := func(h, m int) time.Time { return day.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute) }
	// Busy 10:00–12:00 plus its 15-minute cleaning slot.
	busy := []Interval{{Start: at(10, 0), End: at(12, 15)}}
	want := Interval{Start: at(11, 0), End: at(12, 0)}
	got := SuggestSlots(want, 15*time.Minute, busy, at(8, 0), at(14, 0), at(0, 0), 15*time.Minute, 3)
	if len(got) != 3 {
		t.Fatalf("want 3 suggestions, got %v", got)
	}
	for _, s := range got {
		// The slot itself and its own buffer must stay clear of 10:00–12:15.
		if s.Start.Before(at(12, 15)) && at(10, 0).Before(s.End.Add(15*time.Minute)) {
			t.Errorf("suggestion %v–%v overlaps the busy period or its buffer", s.Start, s.End)
		}
	}
	// Nearest to 11:00: right after the cleaning slot.
	if !got[0].Start.Equal(at(12, 15)) || !got[1].Start.Equal(at(12, 30)) || !got[2].Start.Equal(at(12, 45)) {
		t.Errorf("unexpected suggestions: %v", got)
	}
	// Before the booking, the latest option ends with its own buffer at 10:00.
	more := SuggestSlots(want, 15*time.Minute, busy, at(8, 0), at(14, 0), at(0, 0), 15*time.Minute, 20)
	if !more[0].Start.Equal(at(8, 0)) || !containsStart(more, at(8, 45)) || containsStart(more, at(9, 0)) {
		t.Errorf("morning suggestions should stop at 08:45: %v", more)
	}
}

func TestSuggestSlotsRespectsNotBefore(t *testing.T) {
	day := time.Date(2026, 3, 1, 0, 0, 0, 0, time.UTC)
	at := func(h int) time.Time { return day.Add(time.Duration(h) * time.Hour) }
	got := SuggestSlots(Interval{Start: at(9), End: at(10)}, 0, nil, at(8), at(12), at(10), time.Hour, 5)
	if len(got) != 2 || !got[0].Start.Equal(at(10)) || !got[1].Start.Equal(at(11)) {
		t.Errorf("got %v", got)
	}
}

func containsStart(list []Interval, t time.Time) bool {
	for _, iv := range list {
		if iv.Start.Equal(t) {
			return true
		}
	}
	return false
}
