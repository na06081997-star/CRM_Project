//go:build integration

// Integration tests against a real PostgreSQL. They recreate the public
// schema of TEST_DATABASE_URL, so never point them at a real database.
//
//	TEST_DATABASE_URL=postgres://crm:crm@localhost:5432/crm_test?sslmode=disable go test -tags integration ./...
package service

import (
	"context"
	"errors"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/db"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/notify"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type recNotifier struct {
	mu   sync.Mutex
	sent []notify.Message
}

func (r *recNotifier) Send(_ context.Context, m notify.Message) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.sent = append(r.sent, m)
	return nil
}

func setup(t *testing.T) (*Service, *store.Store) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if _, err := pool.Exec(ctx, `DROP SCHEMA public CASCADE; CREATE SCHEMA public;`); err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Jerusalem")
	cfg := &config.Config{Location: loc, ReminderLead: time.Hour}
	files, _ := storage.NewS3("", "", "", "", "")
	st := store.New(pool)
	svc := New(cfg, st, nil, &recNotifier{}, files)
	if err := svc.SeedDemo(ctx); err != nil {
		t.Fatal(err)
	}
	return svc, st
}

func actor(t *testing.T, st *store.Store, email string) Actor {
	u, err := st.UserByEmail(context.Background(), email)
	if err != nil {
		t.Fatal(err)
	}
	return Actor{User: u}
}

func hall(t *testing.T, st *store.Store) domain.Room {
	rooms, err := st.ListRooms(context.Background(), store.RoomFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rooms {
		if r.Name == "אולם אירועים" {
			return r
		}
	}
	t.Fatal("hall not seeded")
	return domain.Room{}
}

func TestBookingConflictsAndSeries(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	room := hall(t, st)
	start := time.Now().AddDate(0, 2, 0).Truncate(time.Hour)

	res, err := svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "סדרה", Type: domain.BookingRental,
		StartsAt: start, EndsAt: start.Add(2 * time.Hour), RepeatWeeks: 3})
	if err != nil || len(res.Created) != 3 {
		t.Fatalf("series: %v %+v", err, res)
	}
	if res.Created[0].SeriesID == nil || res.Created[0].Price == 0 {
		t.Fatalf("series id / auto price missing: %+v", res.Created[0])
	}

	// Overlap with the second occurrence.
	_, err = svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "x", Type: domain.BookingInternal,
		StartsAt: start.AddDate(0, 0, 7).Add(time.Hour), EndsAt: start.AddDate(0, 0, 7).Add(3 * time.Hour)})
	var ce *domain.ConflictError
	if !errors.As(err, &ce) || len(ce.Conflicts) != 1 {
		t.Fatalf("expected one conflict, got %v", err)
	}

	// Inside the 30 min buffer after the first one.
	_, err = svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "x", Type: domain.BookingInternal,
		StartsAt: start.Add(2*time.Hour + 10*time.Minute), EndsAt: start.Add(3 * time.Hour)})
	if !errors.As(err, &ce) || ce.Conflicts[0].Reason != domain.ConflictBuffer {
		t.Fatalf("expected buffer conflict, got %v", err)
	}

	// Cancelling the series frees the slot.
	if err := svc.CancelBooking(ctx, admin, res.Created[0].ID, "series"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "after cancel", Type: domain.BookingInternal,
		StartsAt: start.AddDate(0, 0, 7), EndsAt: start.AddDate(0, 0, 7).Add(time.Hour)}); err != nil {
		t.Fatalf("slot should be free after cancel: %v", err)
	}
}

// The database constraint must hold even when two writers race.
func TestConcurrentBookingsNeverOverlap(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	room := hall(t, st)
	start := time.Now().AddDate(0, 3, 0).Truncate(time.Hour)

	var wg sync.WaitGroup
	var mu sync.Mutex
	ok := 0
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "race", Type: domain.BookingInternal,
				StartsAt: start, EndsAt: start.Add(time.Hour)})
			if err == nil {
				mu.Lock()
				ok++
				mu.Unlock()
			}
		}()
	}
	wg.Wait()
	if ok != 1 {
		t.Fatalf("exactly one concurrent booking must succeed, got %d", ok)
	}
}

func TestRequestApprovalFlowAndPayments(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	user := actor(t, st, "dana@example.com")
	room := hall(t, st)
	start := time.Now().AddDate(0, 1, 0)
	start = time.Date(start.Year(), start.Month(), start.Day(), 18, 0, 0, 0, svc.loc)

	req, err := svc.SubmitRequest(ctx, user, RequestInput{RoomID: room.ID, Title: "אירוסין", StartsAt: start, EndsAt: start.Add(3 * time.Hour), Attendees: 50})
	if err != nil {
		t.Fatal(err)
	}
	if req.QuotedPrice != 3*room.HourlyRate {
		t.Errorf("quote = %v", req.QuotedPrice)
	}
	// A second request for the same slot is accepted (not yet booked)…
	other, err := svc.SubmitRequest(ctx, actor(t, st, "uri@example.com"), RequestInput{RoomID: room.ID, Title: "כפול", StartsAt: start, EndsAt: start.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	// Users cannot review.
	if _, err := svc.ReviewRequest(ctx, user, req.ID, ReviewDecision{Approve: true}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("user review must be forbidden, got %v", err)
	}
	price := 1200.0
	approved, err := svc.ReviewRequest(ctx, admin, req.ID, ReviewDecision{Approve: true, FinalPrice: &price})
	if err != nil || approved.Status != domain.RequestApproved || approved.BookingID == nil {
		t.Fatalf("approve: %v %+v", err, approved)
	}
	// …but cannot be approved once the slot is taken.
	if _, err := svc.ReviewRequest(ctx, admin, other.ID, ReviewDecision{Approve: true}); err == nil {
		t.Fatal("approving an overlapping request must fail")
	}

	b, _ := st.BookingByID(ctx, *approved.BookingID)
	if b.Price != 1200 || b.Status != domain.StatusConfirmed {
		t.Fatalf("booking %+v", b)
	}
	if _, err := svc.RecordPayment(ctx, admin, PaymentInput{BookingID: b.ID, Amount: 1500, Method: domain.PayCash}); err == nil {
		t.Fatal("overpayment must be rejected")
	}
	if _, err := svc.RecordPayment(ctx, admin, PaymentInput{BookingID: b.ID, Amount: 700, Method: domain.PayCheck}); err == nil {
		t.Fatal("check without number must be rejected")
	}
	if _, err := svc.RecordPayment(ctx, admin, PaymentInput{BookingID: b.ID, Amount: 700, Method: domain.PayCheck, Reference: "77"}); err != nil {
		t.Fatal(err)
	}
	b, _ = st.BookingByID(ctx, b.ID)
	if b.Balance() != 500 {
		t.Fatalf("balance = %v", b.Balance())
	}
	entries, _ := st.ListAudit(ctx, store.AuditFilter{Entity: "payment"})
	if len(entries) != 1 {
		t.Fatalf("payment must be audited, got %d entries", len(entries))
	}
}

func TestStaffTaskPermissions(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	avi := actor(t, st, "avi@matnas.local")
	samir := actor(t, st, "samir@matnas.local")
	tasks, err := svc.ListTasks(ctx, avi, store.TaskFilter{})
	if err != nil || len(tasks) == 0 {
		t.Fatalf("avi should see own tasks: %v", err)
	}
	for _, tk := range tasks {
		if tk.AssigneeID == nil || *tk.AssigneeID != avi.User.ID {
			t.Fatal("staff must only see their own tasks")
		}
	}
	if _, err := svc.UpdateTaskStatus(ctx, samir, tasks[0].ID, domain.TaskDone, ""); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("another worker must not complete it: %v", err)
	}
	done, err := svc.UpdateTaskStatus(ctx, avi, tasks[0].ID, domain.TaskDone, "בוצע")
	if err != nil || done.CompletedAt == nil || done.CompletionNote != "בוצע" {
		t.Fatalf("complete: %v %+v", err, done)
	}
}

func TestSignageAndDashboard(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	view, err := svc.Signage(ctx, "lobby")
	if err != nil || view.Screen == nil || len(view.Rooms) == 0 {
		t.Fatalf("signage: %v %+v", err, view)
	}
	if _, err := svc.Signage(ctx, "nope"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown screen should 404: %v", err)
	}
	d, err := svc.Dashboard(ctx, actor(t, st, "maya@matnas.local"), nil)
	if err != nil || len(d.Week) != 7 || d.PendingRequests != 3 {
		t.Fatalf("dashboard: %v %+v", err, d)
	}
	if _, err := svc.Dashboard(ctx, actor(t, st, "dana@example.com"), nil); !errors.Is(err, domain.ErrForbidden) {
		t.Fatal("portal users must not see the dashboard")
	}
}
