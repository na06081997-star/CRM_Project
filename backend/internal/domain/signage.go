package domain

import (
	"sort"
	"time"

	"github.com/google/uuid"
)

type RoomState string

const (
	RoomAvailable RoomState = "available"
	RoomOccupied  RoomState = "occupied"
	RoomSoon      RoomState = "soon" // free now, next booking starts shortly
	RoomClosed    RoomState = "closed"
)

// SoonThreshold is how early a room is flagged as "starting soon".
const SoonThreshold = 15 * time.Minute

type SignageEvent struct {
	BookingID uuid.UUID   `json:"bookingId"`
	Title     string      `json:"title"`
	Type      BookingType `json:"type"`
	StartsAt  time.Time   `json:"startsAt"`
	EndsAt    time.Time   `json:"endsAt"`
	RoomID    uuid.UUID   `json:"roomId"`
	RoomName  string      `json:"roomName"`
	Host      *string     `json:"host"`
}

type SignageRoom struct {
	RoomID           uuid.UUID     `json:"roomId"`
	Name             string        `json:"name"`
	Color            string        `json:"color"`
	State            RoomState     `json:"state"`
	Current          *SignageEvent `json:"current"`
	Next             *SignageEvent `json:"next"`
	MinutesRemaining int           `json:"minutesRemaining"` // until current ends
	MinutesUntilNext int           `json:"minutesUntilNext"` // until next starts
	ProgressPercent  int           `json:"progressPercent"`  // of current event
}

type SignageSnapshot struct {
	GeneratedAt   time.Time      `json:"generatedAt"`
	Rooms         []SignageRoom  `json:"rooms"`
	Upcoming      []SignageEvent `json:"upcoming"`
	Announcements []Announcement `json:"announcements"`
	Stats         SignageStats   `json:"stats"`
}

type SignageStats struct {
	Occupied  int `json:"occupied"`
	Available int `json:"available"`
	Today     int `json:"today"`
}

// BuildSignage computes what each room is doing at `now` from the day's
// bookings. Only confirmed bookings are shown publicly; maintenance/closed
// blocks mark the room closed without exposing their title.
func BuildSignage(now time.Time, rooms []Room, bookings []Booking, upcomingLimit int) SignageSnapshot {
	snap := SignageSnapshot{GeneratedAt: now}
	byRoom := map[uuid.UUID][]Booking{}
	for _, b := range bookings {
		if b.Status != StatusConfirmed {
			continue
		}
		byRoom[b.RoomID] = append(byRoom[b.RoomID], b)
		if b.Type != BookingMaintenance && b.Type != BookingClosed {
			snap.Stats.Today++
		}
	}

	for _, r := range rooms {
		list := byRoom[r.ID]
		sort.Slice(list, func(i, j int) bool { return list[i].StartsAt.Before(list[j].StartsAt) })
		sr := SignageRoom{RoomID: r.ID, Name: r.Name, Color: r.Color, State: RoomAvailable}

		for i := range list {
			b := list[i]
			switch {
			case !now.Before(b.StartsAt) && now.Before(b.EndsAt) && sr.Current == nil:
				ev := toEvent(b, r.Name)
				sr.Current = &ev
				sr.MinutesRemaining = ceilMinutes(b.EndsAt.Sub(now))
				total := b.EndsAt.Sub(b.StartsAt)
				sr.ProgressPercent = int(100 * now.Sub(b.StartsAt) / total)
				if b.Type == BookingMaintenance || b.Type == BookingClosed {
					sr.State = RoomClosed
				} else {
					sr.State = RoomOccupied
				}
			case b.StartsAt.After(now) && sr.Next == nil:
				ev := toEvent(b, r.Name)
				sr.Next = &ev
				sr.MinutesUntilNext = ceilMinutes(b.StartsAt.Sub(now))
			}
		}
		if sr.State == RoomAvailable && sr.Next != nil && sr.Next.StartsAt.Sub(now) <= SoonThreshold {
			sr.State = RoomSoon
		}
		if sr.State == RoomOccupied || sr.State == RoomClosed {
			snap.Stats.Occupied++
		} else {
			snap.Stats.Available++
		}
		snap.Rooms = append(snap.Rooms, sr)
	}

	roomNames := map[uuid.UUID]string{}
	for _, r := range rooms {
		roomNames[r.ID] = r.Name
	}
	var upcoming []SignageEvent
	for _, b := range bookings {
		if b.Status == StatusConfirmed && b.StartsAt.After(now) &&
			b.Type != BookingMaintenance && b.Type != BookingClosed {
			if name, ok := roomNames[b.RoomID]; ok {
				upcoming = append(upcoming, toEvent(b, name))
			}
		}
	}
	sort.Slice(upcoming, func(i, j int) bool { return upcoming[i].StartsAt.Before(upcoming[j].StartsAt) })
	if upcomingLimit > 0 && len(upcoming) > upcomingLimit {
		upcoming = upcoming[:upcomingLimit]
	}
	snap.Upcoming = upcoming
	return snap
}

func toEvent(b Booking, roomName string) SignageEvent {
	title := b.Title
	if b.Type == BookingMaintenance || b.Type == BookingClosed {
		title = "סגור לתחזוקה"
	}
	host := b.CoordinatorName
	if host == nil {
		host = b.CustomerName
	}
	return SignageEvent{
		BookingID: b.ID, Title: title, Type: b.Type, StartsAt: b.StartsAt, EndsAt: b.EndsAt,
		RoomID: b.RoomID, RoomName: roomName, Host: host,
	}
}

func ceilMinutes(d time.Duration) int {
	m := int(d / time.Minute)
	if d%time.Minute > 0 {
		m++
	}
	return m
}
