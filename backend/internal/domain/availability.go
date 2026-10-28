package domain

import (
	"sort"
	"time"
)

type Interval struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// FreeSlots returns the gaps of at least minLen inside [from,to) that are not
// covered by any busy interval. Busy intervals may overlap or be unsorted.
func FreeSlots(from, to time.Time, busy []Interval, minLen time.Duration) []Interval {
	sorted := make([]Interval, 0, len(busy))
	for _, b := range busy {
		if b.End.After(from) && b.Start.Before(to) {
			sorted = append(sorted, b)
		}
	}
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Start.Before(sorted[j].Start) })

	var out []Interval
	cursor := from
	for _, b := range sorted {
		if b.Start.After(cursor) && b.Start.Sub(cursor) >= minLen {
			out = append(out, Interval{Start: cursor, End: b.Start})
		}
		if b.End.After(cursor) {
			cursor = b.End
		}
	}
	if to.Sub(cursor) >= minLen {
		out = append(out, Interval{Start: cursor, End: to})
	}
	return out
}

// BusyIntervals converts blocking bookings into busy ranges (end includes buffer).
func BusyIntervals(bookings []Booking) []Interval {
	out := make([]Interval, 0, len(bookings))
	for _, b := range bookings {
		if b.Status.Blocks() {
			out = append(out, Interval{Start: b.StartsAt, End: b.BlockedUntil()})
		}
	}
	return out
}
