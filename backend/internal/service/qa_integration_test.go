//go:build integration

package service

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"image"
	"image/color"
	"image/png"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// memFiles is an in-memory object store for tests that upload files.
type memFiles struct{ objects map[string][]byte }

func (m *memFiles) Put(_ context.Context, prefix, filename, _ string, r io.Reader, _ int64) (string, error) {
	b, err := io.ReadAll(r)
	key := prefix + "/" + uuid.NewString() + "-" + filename
	m.objects[key] = b
	return key, err
}
func (m *memFiles) Get(_ context.Context, key string) (io.ReadCloser, storage.ObjectInfo, error) {
	b := m.objects[key]
	return io.NopCloser(bytes.NewReader(b)), storage.ObjectInfo{Size: int64(len(b))}, nil
}
func (m *memFiles) Delete(_ context.Context, key string) error { delete(m.objects, key); return nil }
func (m *memFiles) Ready(context.Context) error                { return nil }

// localAt returns daysAhead days from now at hour h, Israel time.
func localAt(svc *Service, daysAhead, h int) time.Time {
	d := time.Now().In(svc.loc).AddDate(0, 0, daysAhead)
	return time.Date(d.Year(), d.Month(), d.Day(), h, 0, 0, 0, svc.loc)
}

func roomNamed(t *testing.T, st *store.Store, name string) domain.Room {
	rooms, err := st.ListRooms(context.Background(), store.RoomFilter{})
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range rooms {
		if r.Name == name {
			return r
		}
	}
	t.Fatalf("room %s not seeded", name)
	return domain.Room{}
}

// Cancelling a booking frees its cleaning buffer and cancels its tasks.
func TestCancelReleasesBufferAndTasks(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	cleaner := actor(t, st, "avi@matnas.local")
	room := hall(t, st) // 30 min buffer
	start := localAt(svc, 100, 10)

	res, err := svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "כנס", Type: domain.BookingInternal, StartsAt: start, EndsAt: start.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	b := res.Created[0]
	task, err := svc.SaveTask(ctx, admin, domain.Task{Title: "ניקיון אחרי הכנס", Kind: "cleaning", RoomID: &room.ID, BookingID: &b.ID,
		AssigneeID: &cleaner.User.ID, DueAt: b.EndsAt})
	if err != nil {
		t.Fatal(err)
	}

	next := BookingInput{RoomID: room.ID, Title: "מיד אחרי", Type: domain.BookingInternal, StartsAt: b.EndsAt.Add(15 * time.Minute), EndsAt: b.EndsAt.Add(time.Hour)}
	var ce *domain.ConflictError
	if _, err := svc.CreateBooking(ctx, admin, next); !errors.As(err, &ce) || ce.Conflicts[0].Reason != domain.ConflictBuffer {
		t.Fatalf("expected a buffer conflict before cancelling, got %v", err)
	}

	if err := svc.CancelBooking(ctx, admin, b.ID, "one"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.CreateBooking(ctx, admin, next); err != nil {
		t.Fatalf("the cleaning slot should be released with the booking: %v", err)
	}
	got, err := st.TaskByID(ctx, task.ID)
	if err != nil || got.Status != domain.TaskCancelled {
		t.Fatalf("attached task should be cancelled, got %v %v", got.Status, err)
	}
}

// Approving an owner's cancellation request also cascades to the tasks.
func TestResolvedCancellationCancelsTasks(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	yoav := actor(t, st, "yoav@matnas.local")
	room := roomNamed(t, st, "חדר חוגים")
	start := localAt(svc, 101, 16)
	res, err := svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "חוג", Type: domain.BookingActivity,
		CoordinatorID: &yoav.User.ID, StartsAt: start, EndsAt: start.Add(time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	b := res.Created[0]
	task, err := svc.SaveTask(ctx, admin, domain.Task{Title: "הקמה", Kind: "setup", BookingID: &b.ID, DueAt: start.Add(-time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if err := svc.RequestCancellation(ctx, yoav, b.ID, "החוג בוטל"); err != nil {
		t.Fatal(err)
	}
	if err := svc.ResolveCancellation(ctx, admin, b.ID, true, ""); err != nil {
		t.Fatal(err)
	}
	if got, _ := st.TaskByID(ctx, task.ID); got.Status != domain.TaskCancelled {
		t.Fatalf("task should be cancelled, got %s", got.Status)
	}
}

func TestCoordinatorCannotRequestPaidRental(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	yoav := actor(t, st, "yoav@matnas.local")
	dana := actor(t, st, "dana@example.com")
	room := roomNamed(t, st, "חדר ישיבות 2")
	start := localAt(svc, 102, 10)
	in := RequestInput{RoomID: room.ID, Title: "ישיבה", StartsAt: start, EndsAt: start.Add(time.Hour), Phone: "050-2222222", Route: "rental"}

	_, err := svc.SubmitRequest(ctx, yoav, in)
	var ve *domain.ValidationError
	if !errors.As(err, &ve) || ve.Fields["route"] == "" {
		t.Fatalf("coordinator rental must be refused, got %v", err)
	}
	in.Route = "activity"
	if _, err := svc.SubmitRequest(ctx, yoav, in); err != nil {
		t.Fatalf("coordinator activity request: %v", err)
	}
	in.Route, in.StartsAt, in.EndsAt = "rental", start.Add(3*time.Hour), start.Add(4*time.Hour)
	if _, err := svc.SubmitRequest(ctx, dana, in); err != nil {
		t.Fatalf("portal user rental request: %v", err)
	}
}

// A weekly request becomes a series on approval; taken dates are skipped.
func TestRecurringRequestApproval(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	yoav := actor(t, st, "yoav@matnas.local")
	room := roomNamed(t, st, "חדר חוגים")
	first := localAt(svc, 110, 17)

	// Someone already holds the third week.
	if _, err := svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "אירוע", Type: domain.BookingInternal,
		StartsAt: first.AddDate(0, 0, 14), EndsAt: first.AddDate(0, 0, 14).Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}

	until := first.AddDate(0, 0, 21).Format("2006-01-02")
	check, err := svc.CheckSlot(ctx, yoav, SlotQuery{RoomID: room.ID, StartsAt: first, EndsAt: first.Add(time.Hour), RepeatUntil: until})
	if err != nil {
		t.Fatal(err)
	}
	if len(check.Occurrences) != 4 || check.Occurrences[2].Free || !check.Occurrences[0].Free {
		t.Fatalf("preview should show 4 weeks with the third taken: %+v", check.Occurrences)
	}

	req, err := svc.SubmitRequest(ctx, yoav, RequestInput{RoomID: room.ID, Title: "חוג ציור", Route: "activity", StartsAt: first,
		EndsAt: first.Add(time.Hour), Phone: "050-2222222", RepeatUntil: until})
	if err != nil {
		t.Fatal(err)
	}
	if req.RepeatUntil == nil {
		t.Fatal("repeatUntil not stored")
	}
	out, err := svc.ReviewRequest(ctx, admin, req.ID, ReviewDecision{Approve: true})
	if err != nil {
		t.Fatal(err)
	}
	if out.ApprovedCount != 3 || len(out.SkippedDates) != 1 {
		t.Fatalf("want 3 approved + 1 skipped, got %d / %v", out.ApprovedCount, out.SkippedDates)
	}
	series, err := st.ListBookings(ctx, store.BookingFilter{RoomIDs: []uuid.UUID{room.ID}, From: first.Add(-time.Hour), To: first.AddDate(0, 0, 22),
		Statuses: []string{"confirmed"}, Search: "חוג ציור"})
	if err != nil || len(series) != 3 || series[0].SeriesID == nil || *series[0].SeriesID != *series[2].SeriesID || series[0].Type != domain.BookingActivity {
		t.Fatalf("expected one 3-meeting activity series, got %d %v", len(series), err)
	}

	if _, err := svc.SubmitRequest(ctx, yoav, RequestInput{RoomID: room.ID, Title: "ארוך", Route: "activity", StartsAt: first.Add(2 * time.Hour),
		EndsAt: first.Add(3 * time.Hour), Phone: "050-2222222", RepeatUntil: first.AddDate(2, 0, 0).Format("2006-01-02")}); err == nil {
		t.Fatal("a series longer than a year must be refused")
	}
}

// Occupied slots come with alternatives that keep clear of cleaning buffers.
func TestCheckSlotSuggestsAlternatives(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	yoav := actor(t, st, "yoav@matnas.local")
	room := hall(t, st)
	start := localAt(svc, 103, 12)
	if _, err := svc.CreateBooking(ctx, admin, BookingInput{RoomID: room.ID, Title: "סודי", Type: domain.BookingRental, StartsAt: start, EndsAt: start.Add(2 * time.Hour)}); err != nil {
		t.Fatal(err)
	}
	res, err := svc.CheckSlot(ctx, yoav, SlotQuery{RoomID: room.ID, StartsAt: start.Add(time.Hour), EndsAt: start.Add(2 * time.Hour)})
	if err != nil {
		t.Fatal(err)
	}
	if res.Free || len(res.Conflicts) == 0 || res.Conflicts[0].Title != "" {
		t.Fatalf("expected an anonymous conflict, got %+v", res.Conflicts)
	}
	if len(res.SameRoom) == 0 || len(res.OtherRooms) == 0 {
		t.Fatalf("expected alternatives, got %+v / %+v", res.SameRoom, res.OtherRooms)
	}
	busyEnd := start.Add(2*time.Hour + time.Duration(room.BufferMinutes)*time.Minute)
	for _, s := range res.SameRoom {
		if s.Start.Before(busyEnd) && start.Before(s.End.Add(time.Duration(room.BufferMinutes)*time.Minute)) {
			t.Errorf("suggestion %v–%v collides with the booking or a buffer", s.Start, s.End)
		}
	}
}

func TestTicketFlow(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	yoav := actor(t, st, "yoav@matnas.local")
	samir := actor(t, st, "samir@matnas.local")
	avi := actor(t, st, "avi@matnas.local")
	room := roomNamed(t, st, "חדר ישיבות 1")

	tk, err := svc.ReportIssue(ctx, yoav, IssueInput{Kind: "repair", Title: "כיסא שבור", Description: "רגל סדוקה", RoomID: &room.ID})
	if err != nil {
		t.Fatal(err)
	}
	if tk.AssigneeID != nil || tk.ReporterPhone == nil || *tk.ReporterPhone != "050-2222222" {
		t.Fatalf("ticket should be unassigned with the reporter's phone: %+v", tk)
	}
	if _, err := svc.ReportIssue(ctx, samir, IssueInput{Kind: "repair", Description: "x"}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("staff do not open tickets: %v", err)
	}
	if _, err := svc.ReportIssue(ctx, yoav, IssueInput{Kind: "supplies"}); err == nil {
		t.Fatal("a ticket without any detail must be refused")
	}

	mine, err := svc.ListTasks(ctx, yoav, store.TaskFilter{})
	if err != nil || len(mine) == 0 {
		t.Fatalf("coordinator should list own tickets: %v", err)
	}
	for _, x := range mine {
		if x.ReportedBy == nil || *x.ReportedBy != yoav.User.ID {
			t.Fatalf("coordinator sees someone else's task: %+v", x)
		}
	}
	unassigned, _ := svc.ListTasks(ctx, admin, store.TaskFilter{Unassigned: true, Statuses: []string{"open"}})
	if len(unassigned) == 0 {
		t.Fatal("admin should see unassigned tickets")
	}

	if _, err := svc.AssignTask(ctx, admin, tk.ID, &yoav.User.ID); err == nil {
		t.Fatal("only staff can be assigned")
	}
	if _, err := svc.AssignTask(ctx, yoav, tk.ID, &samir.User.ID); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("coordinators cannot assign: %v", err)
	}
	if _, err := svc.AssignTask(ctx, admin, tk.ID, &samir.User.ID); err != nil {
		t.Fatal(err)
	}

	// Photos: the reporter adds what is broken, the worker the proof it is fixed.
	can := func(a Actor, entity string, write bool) bool {
		return svc.fileAccess(ctx, &a, entity, tk.ID, write) == nil
	}
	if !can(yoav, "task", true) || !can(samir, "task", false) || can(samir, "task", true) {
		t.Error("ticket photo access is wrong")
	}
	if !can(samir, "task_done", true) || !can(yoav, "task_done", false) || can(yoav, "task_done", true) || can(avi, "task_done", false) {
		t.Error("completion photo access is wrong")
	}

	done, err := svc.UpdateTaskStatus(ctx, samir, tk.ID, domain.TaskDone, "הוחלף")
	if err != nil || done.Status != domain.TaskDone {
		t.Fatalf("worker completes the ticket: %v", err)
	}
}

func signaturePNG(t *testing.T) string {
	img := image.NewRGBA(image.Rect(0, 0, 120, 40))
	for x := 10; x < 110; x++ {
		img.Set(x, 20+x%7, color.Black)
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buf.Bytes())
}

func TestCoordinatorContractOnboarding(t *testing.T) {
	svc, st := setup(t)
	svc.files = &memFiles{objects: map[string][]byte{}}
	svc.cfg.CORSOrigins = []string{"https://maaleramot.example"}
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	yoav := actor(t, st, "yoav@matnas.local")

	in := CoordinatorInput{FullName: "נועה רכזת", Email: "noa@example.com", Phone: "050-1234567", Contract: "digital", Title: "הסכם רכז/ת", Body: "נוסח החוזה"}
	if _, err := svc.CreateCoordinator(ctx, yoav, in); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("coordinators cannot add coordinators: %v", err)
	}
	res, err := svc.CreateCoordinator(ctx, admin, in)
	if err != nil {
		t.Fatal(err)
	}
	if res.User.IsActive || res.User.Role != domain.RoleCoordinator || res.Contract == nil {
		t.Fatalf("new coordinator must wait for the signature: %+v", res.User)
	}
	if !strings.HasPrefix(res.SignURL, "https://maaleramot.example/sign/") {
		t.Fatalf("bad sign url %q", res.SignURL)
	}
	token := strings.TrimPrefix(res.SignURL, "https://maaleramot.example/sign/")

	if _, err := svc.CreateCoordinator(ctx, admin, in); err == nil {
		t.Fatal("duplicate e-mail must be refused")
	}
	if _, err := svc.PublicContract(ctx, "not-a-real-token-but-long-enough"); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown token: %v", err)
	}
	pub, err := svc.PublicContract(ctx, token)
	if err != nil || pub.Status != "pending" || pub.Body != "נוסח החוזה" {
		t.Fatalf("public contract: %+v %v", pub, err)
	}
	if _, err := svc.SignContract(ctx, token, SignInput{Name: "נועה", Agree: true, Signature: "data:image/png;base64,AAAA"}, "1.2.3.4"); err == nil {
		t.Fatal("an empty signature must be refused")
	}
	pub, err = svc.SignContract(ctx, token, SignInput{Name: "נועה רכזת", Agree: true, Signature: signaturePNG(t)}, "1.2.3.4")
	if err != nil || pub.Status != "signed" {
		t.Fatalf("sign: %+v %v", pub, err)
	}
	u, _ := st.UserByID(ctx, res.User.ID)
	if !u.IsActive {
		t.Fatal("signing must activate the account")
	}
	files, _ := st.ListAttachments(ctx, "contract", res.Contract.ID)
	if len(files) != 1 || files[0].ContentType != "image/png" {
		t.Fatalf("signature image should be stored, got %+v", files)
	}
	if _, err := svc.SignContract(ctx, token, SignInput{Name: "נועה", Agree: true, Signature: signaturePNG(t)}, ""); err == nil {
		t.Fatal("a contract cannot be signed twice")
	}

	// Scan flow: refused until the signed copy is uploaded.
	scan, err := svc.CreateCoordinator(ctx, admin, CoordinatorInput{FullName: "דן", Email: "dan@example.com", Contract: "scan", Title: "הסכם", Body: "נוסח"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := svc.MarkContractSigned(ctx, admin, scan.Contract.ID); err == nil {
		t.Fatal("marking signed needs an uploaded scan")
	}
	if _, err := svc.Upload(ctx, admin, "contract", scan.Contract.ID, "scan.pdf", "application/pdf", strings.NewReader("%PDF-1.4"), 8); err != nil {
		t.Fatal(err)
	}
	if c, err := svc.MarkContractSigned(ctx, admin, scan.Contract.ID); err != nil || c.Status != "signed" {
		t.Fatalf("mark signed: %v", err)
	}
	if u, _ := st.UserByID(ctx, scan.User.ID); !u.IsActive {
		t.Fatal("scan signature must activate the account")
	}
}

func TestBillingFilters(t *testing.T) {
	svc, st := setup(t)
	ctx := context.Background()
	admin := actor(t, st, "maya@matnas.local")
	yoav := actor(t, st, "yoav@matnas.local")
	for _, p := range []string{"paid", "partial", "unpaid", "open"} {
		list, err := svc.Billing(ctx, admin, store.BookingFilter{Payment: p})
		if err != nil {
			t.Fatal(err)
		}
		for _, b := range list {
			ok := map[string]bool{
				"paid": b.PaidAmount >= b.Price, "partial": b.PaidAmount > 0 && b.PaidAmount < b.Price,
				"unpaid": b.PaidAmount == 0, "open": b.PaidAmount < b.Price,
			}[p]
			if !ok || b.Price <= 0 {
				t.Errorf("%s filter returned %s (price %.0f paid %.0f)", p, b.Title, b.Price, b.PaidAmount)
			}
		}
	}
	paid, _ := svc.Billing(ctx, admin, store.BookingFilter{Payment: "paid"})
	partial, _ := svc.Billing(ctx, admin, store.BookingFilter{Payment: "partial"})
	if len(paid)+len(partial) == 0 {
		t.Error("seed has paid bookings")
	}
	if _, err := svc.Billing(ctx, yoav, store.BookingFilter{}); !errors.Is(err, domain.ErrForbidden) {
		t.Fatalf("coordinators do not see billing: %v", err)
	}
}
