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
