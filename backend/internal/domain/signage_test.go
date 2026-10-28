package domain

import "testing"

func TestBuildSignage(t *testing.T) {
	rooms := []Room{
		{ID: roomA, Name: "אולם אירועים"},
		{ID: roomB, Name: "סטודיו"},
	}
	current := booking(roomA, 10, 0, 12, 0, 0)
	current.Title = "כנס מתנדבים"
	next := booking(roomB, 11, 10, 12, 0, 0)
	later := booking(roomA, 18, 0, 20, 0, 0)
	pending := booking(roomB, 10, 0, 11, 0, 0)
	pending.Status = StatusPending

	now := at(11, 0)
	snap := BuildSignage(now, rooms, []Booking{later, current, next, pending}, 10)

	if len(snap.Rooms) != 2 {
		t.Fatalf("want 2 rooms, got %d", len(snap.Rooms))
	}
	a, b := snap.Rooms[0], snap.Rooms[1]
	if a.State != RoomOccupied || a.Current == nil || a.Current.Title != "כנס מתנדבים" {
		t.Errorf("room A should be occupied by current booking: %+v", a)
	}
	if a.MinutesRemaining != 60 || a.ProgressPercent != 50 {
		t.Errorf("room A remaining=%d progress=%d", a.MinutesRemaining, a.ProgressPercent)
	}
	if a.Next == nil || !a.Next.StartsAt.Equal(later.StartsAt) {
		t.Errorf("room A next should be the evening booking")
	}
	if b.State != RoomSoon || b.MinutesUntilNext != 10 {
		t.Errorf("room B should be 'soon' (10 min), got %s %d", b.State, b.MinutesUntilNext)
	}
	if snap.Stats.Occupied != 1 || snap.Stats.Available != 1 || snap.Stats.Today != 3 {
		t.Errorf("stats = %+v", snap.Stats)
	}
	if len(snap.Upcoming) != 2 || !snap.Upcoming[0].StartsAt.Equal(next.StartsAt) {
		t.Errorf("upcoming should list future confirmed bookings sorted: %+v", snap.Upcoming)
	}
}

func TestBuildSignageHidesMaintenanceTitle(t *testing.T) {
	m := booking(roomA, 10, 0, 12, 0, 0)
	m.Type, m.Title = BookingMaintenance, "החלפת נורות - פנימי"
	snap := BuildSignage(at(11, 0), []Room{{ID: roomA}}, []Booking{m}, 5)
	r := snap.Rooms[0]
	if r.State != RoomClosed || r.Current.Title == m.Title {
		t.Fatalf("maintenance must show as closed with a generic title: %+v", r)
	}
	if snap.Stats.Today != 0 || len(snap.Upcoming) != 0 {
		t.Fatalf("maintenance is not a public event: %+v", snap)
	}
}

func TestBuildSignageUpcomingLimit(t *testing.T) {
	var bs []Booking
	for h := 12; h < 20; h++ {
		bs = append(bs, booking(roomA, h, 0, h, 30, 0))
	}
	snap := BuildSignage(at(11, 0), []Room{{ID: roomA}}, bs, 3)
	if len(snap.Upcoming) != 3 {
		t.Fatalf("upcoming limit not applied: %d", len(snap.Upcoming))
	}
}
