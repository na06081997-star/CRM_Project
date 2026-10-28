package domain

import (
	"time"
)

// Occurrences expands a recurring task schedule into concrete due times in
// [from, to), evaluated in loc so "08:00 every Sunday" survives DST changes.
func Occurrences(s TaskSchedule, from, to time.Time, loc *time.Location) []time.Time {
	tod, err := time.Parse("15:04", s.TimeOfDay)
	if err != nil || !s.IsActive || len(s.Weekdays) == 0 {
		return nil
	}
	days := map[time.Weekday]bool{}
	for _, d := range s.Weekdays {
		if d >= 0 && d <= 6 {
			days[time.Weekday(d)] = true
		}
	}

	var out []time.Time
	f := from.In(loc)
	for d := time.Date(f.Year(), f.Month(), f.Day(), 0, 0, 0, 0, loc); d.Before(to); d = d.AddDate(0, 0, 1) {
		if !days[d.Weekday()] {
			continue
		}
		due := time.Date(d.Year(), d.Month(), d.Day(), tod.Hour(), tod.Minute(), 0, 0, loc)
		if !due.Before(from) && due.Before(to) {
			out = append(out, due)
		}
	}
	return out
}

// RecurrenceDates returns the start times of a weekly series: the first
// occurrence plus one per week until `until` (inclusive), capped at max.
func RecurrenceDates(first, until time.Time, weeks int, max int) []time.Time {
	out := []time.Time{first}
	for i := 1; len(out) < max; i++ {
		next := first.AddDate(0, 0, 7*i)
		if (!until.IsZero() && next.After(until)) || (until.IsZero() && i >= weeks) {
			break
		}
		out = append(out, next)
	}
	return out
}
