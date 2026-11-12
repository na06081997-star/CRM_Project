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

// SuggestSlots proposes up to n windows of the wanted length in [open, close)
// that the room can host, nearest to the wanted start first, then returned in
// chronological order. A window [s, s+len) also needs its own turnover buffer
// free after it, and busy intervals already include other bookings' buffers,
// so a suggestion never lands in a cleaning slot. Starts are aligned to step
// and never before notBefore.
func SuggestSlots(want Interval, buffer time.Duration, busy []Interval, open, close, notBefore time.Time, step time.Duration, n int) []Interval {
	length := want.End.Sub(want.Start)
	if length <= 0 || step <= 0 || n <= 0 {
		return nil
	}
	var cands []Interval
	for s := open; !s.Add(length).After(close); s = s.Add(step) {
		if s.Before(notBefore) || s.Equal(want.Start) {
			continue
		}
		blockedEnd := s.Add(length + buffer)
		free := true
		for _, b := range busy {
			if s.Before(b.End) && b.Start.Before(blockedEnd) {
				free = false
				break
			}
		}
		if free {
			cands = append(cands, Interval{Start: s, End: s.Add(length)})
		}
	}
	dist := func(iv Interval) time.Duration {
		d := iv.Start.Sub(want.Start)
		if d < 0 {
			return -d
		}
		return d
	}
	sort.SliceStable(cands, func(i, j int) bool { return dist(cands[i]) < dist(cands[j]) })
	if len(cands) > n {
		cands = cands[:n]
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].Start.Before(cands[j].Start) })
	return cands
}
