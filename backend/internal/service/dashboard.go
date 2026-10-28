package service

import (
	"context"
	"sort"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type RoomOccupancy struct {
	RoomID  uuid.UUID `json:"roomId"`
	Name    string    `json:"name"`
	Color   string    `json:"color"`
	Hours   float64   `json:"hours"`
	Percent int       `json:"percent"`
}

type DayCount struct {
	Date    string  `json:"date"` // YYYY-MM-DD
	Count   int     `json:"count"`
	Revenue float64 `json:"revenue"`
}

type Dashboard struct {
	TodayBookings   int                     `json:"todayBookings"`
	OngoingNow      int                     `json:"ongoingNow"`
	PendingRequests int                     `json:"pendingRequests"`
	OccupancyToday  int                     `json:"occupancyToday"` // percent of opening hours
	OccupancyWeek   int                     `json:"occupancyWeek"`
	OpenTasks       int                     `json:"openTasks"`
	Payments        store.PaymentSummary    `json:"payments"`
	Today           []domain.Booking        `json:"today"`
	LatestRequests  []domain.BookingRequest `json:"latestRequests"`
	RoomsWeek       []RoomOccupancy         `json:"roomsWeek"`
	Week            []DayCount              `json:"week"`
	ByType          map[string]int          `json:"byType"`
	TopUnpaid       []domain.Booking        `json:"topUnpaid"`
}

// openHoursPerDay is the denominator for occupancy (07:00–23:00).
const openHoursPerDay = 16.0

func (s *Service) Dashboard(ctx context.Context, actor Actor, buildingID *uuid.UUID) (Dashboard, error) {
	if err := actor.Require(domain.ActViewDashboard); err != nil {
		return Dashboard{}, err
	}
	now := s.now()
	today := s.startOfDay(now)
	weekStart := today.AddDate(0, 0, -int(today.Weekday())) // Sunday
	weekEnd := weekStart.AddDate(0, 0, 7)

	rooms, err := s.store.ListRooms(ctx, store.RoomFilter{BuildingID: buildingID, OnlyActive: true})
	if err != nil {
		return Dashboard{}, err
	}
	week, err := s.store.ListBookings(ctx, store.BookingFilter{
		From: weekStart, To: weekEnd, BuildingID: buildingID, Statuses: []string{"pending", "confirmed"},
	})
	if err != nil {
		return Dashboard{}, err
	}
	pending, err := s.store.ListRequests(ctx, store.RequestFilter{Status: "pending"})
	if err != nil {
		return Dashboard{}, err
	}
	tasks, err := s.store.ListTasks(ctx, store.TaskFilter{Statuses: []string{"open", "in_progress"}, To: today.AddDate(0, 0, 1)})
	if err != nil {
		return Dashboard{}, err
	}
	pay, err := s.PaymentSummary(ctx, actor)
	if err != nil {
		pay = store.PaymentSummary{} // coordinators without payment rights still get a dashboard
	}
	unpaid, err := s.store.ListBookings(ctx, store.BookingFilter{OnlyWithBalance: true, BuildingID: buildingID})
	if err != nil {
		return Dashboard{}, err
	}

	d := Dashboard{
		PendingRequests: len(pending),
		OpenTasks:       len(tasks),
		Payments:        pay,
		ByType:          map[string]int{},
		Today:           []domain.Booking{},
	}
	if len(pending) > 5 {
		d.LatestRequests = pending[:5]
	} else {
		d.LatestRequests = pending
	}
	sort.Slice(unpaid, func(i, j int) bool { return unpaid[i].Balance() > unpaid[j].Balance() })
	if len(unpaid) > 5 {
		unpaid = unpaid[:5]
	}
	d.TopUnpaid = unpaid

	roomHours := map[uuid.UUID]float64{}
	dayCounts := make([]DayCount, 7)
	for i := range dayCounts {
		dayCounts[i].Date = weekStart.AddDate(0, 0, i).Format("2006-01-02")
	}
	var todayHours, weekHours float64
	tomorrow := today.AddDate(0, 0, 1)
	for _, b := range week {
		if b.Type == domain.BookingClosed {
			continue
		}
		hours := b.EndsAt.Sub(b.StartsAt).Hours()
		roomHours[b.RoomID] += hours
		weekHours += hours
		d.ByType[string(b.Type)]++
		idx := int(s.startOfDay(b.StartsAt).Sub(weekStart).Hours() / 24)
		if idx >= 0 && idx < 7 {
			dayCounts[idx].Count++
			dayCounts[idx].Revenue += b.Price
		}
		if !b.StartsAt.Before(today) && b.StartsAt.Before(tomorrow) {
			d.TodayBookings++
			todayHours += hours
			d.Today = append(d.Today, b)
			if !now.Before(b.StartsAt) && now.Before(b.EndsAt) {
				d.OngoingNow++
			}
		}
	}
	d.Week = dayCounts

	if n := float64(len(rooms)); n > 0 {
		d.OccupancyToday = pct(todayHours, n*openHoursPerDay)
		d.OccupancyWeek = pct(weekHours, n*openHoursPerDay*7)
	}
	for _, r := range rooms {
		d.RoomsWeek = append(d.RoomsWeek, RoomOccupancy{
			RoomID: r.ID, Name: r.Name, Color: r.Color, Hours: roomHours[r.ID],
			Percent: pct(roomHours[r.ID], openHoursPerDay*7),
		})
	}
	sort.Slice(d.RoomsWeek, func(i, j int) bool { return d.RoomsWeek[i].Hours > d.RoomsWeek[j].Hours })
	return d, nil
}

func pct(part, total float64) int {
	if total <= 0 {
		return 0
	}
	p := int(part / total * 100)
	if p > 100 {
		return 100
	}
	return p
}

// ── Public signage ───────────────────────────────────────────

type SignageView struct {
	Screen   *domain.SignageScreen `json:"screen"`
	Building *domain.Building      `json:"building"`
	domain.SignageSnapshot
}

func (s *Service) Signage(ctx context.Context, slug string) (SignageView, error) {
	var view SignageView
	f := store.RoomFilter{OnlyActive: true}
	var screenID *uuid.UUID
	if slug != "" {
		sc, err := s.store.ScreenBySlug(ctx, slug)
		if err != nil {
			return view, err
		}
		view.Screen, screenID = &sc, &sc.ID
		f.BuildingID = sc.BuildingID
		if len(sc.RoomIDs) > 0 {
			f.IDs = sc.RoomIDs
		}
		if sc.BuildingID != nil {
			if b, err := s.store.BuildingByID(ctx, *sc.BuildingID); err == nil {
				view.Building = &b
			}
		}
	}
	rooms, err := s.store.ListRooms(ctx, f)
	if err != nil {
		return view, err
	}
	ids := make([]uuid.UUID, len(rooms))
	for i, r := range rooms {
		ids[i] = r.ID
	}
	now := s.now()
	day := s.startOfDay(now)
	bookings, err := s.store.ListBookings(ctx, store.BookingFilter{
		From: day, To: day.AddDate(0, 0, 1), RoomIDs: ids, Statuses: []string{"confirmed"},
	})
	if err != nil {
		return view, err
	}
	if len(ids) == 0 {
		bookings = nil
	}
	view.SignageSnapshot = domain.BuildSignage(now, rooms, bookings, 8)
	ann, err := s.store.ActiveAnnouncements(ctx, screenID, now)
	if err != nil {
		return view, err
	}
	view.Announcements = ann
	return view, nil
}

// SignageTicker nudges displays once a minute so countdowns stay exact even
// when no booking changes.
func (s *Service) SignageTick(ctx context.Context) {
	s.emit(ctx, EvSignageChanged, map[string]any{"tick": true}, realtime.TopicSignage)
}
