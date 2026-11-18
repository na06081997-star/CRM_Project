package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// SeedDemo fills an empty database with a realistic Hebrew demo matnas,
// anchored on the current week so the calendar and signage look alive.
func (s *Service) SeedDemo(ctx context.Context) error {
	if o, err := s.store.Organization(ctx); err != nil {
		return err
	} else if o.Name == "" {
		o.Name, o.ShortName, o.Subtitle = "מינהל קהילתי מעלה רמות", "מעלה רמות", "מינהל קהילתי"
		o.LogoURL, o.MarkURL = "/brand/clients/maale-ramot.png", "/brand/clients/maale-ramot-mark.png"
		o.Address = "רמות, ירושלים"
		if _, err := s.store.SaveOrganization(ctx, o); err != nil {
			return err
		}
	}
	n, err := s.store.CountUsers(ctx)
	if err != nil || n > 0 {
		return err
	}
	slog.Info("seeding demo data")
	return s.store.InTx(ctx, func(tx *store.Store) error {
		sd := &seeder{tx: tx, ctx: ctx, loc: s.loc}
		sd.run(s.startOfDay(s.now()))
		return sd.err
	})
}

type seeder struct {
	tx  *store.Store
	ctx context.Context
	loc *time.Location
	err error
}

func ptr[T any](v T) *T { return &v }

func (sd *seeder) user(email, name, phone string, role domain.Role, kind string) domain.User {
	if sd.err != nil {
		return domain.User{}
	}
	in := store.UserInput{Email: email, FullName: name, Phone: ptr(phone), Role: role, IsActive: true}
	if kind != "" {
		in.StaffKind = ptr(kind)
	}
	u, err := sd.tx.CreateUser(sd.ctx, in)
	sd.err = err
	return u
}

func (sd *seeder) room(b uuid.UUID, name, kind string, capacity int, color string, rate, min float64, buffer, order int, equipment ...string) domain.Room {
	if sd.err != nil {
		return domain.Room{}
	}
	r, err := sd.tx.CreateRoom(sd.ctx, domain.Room{
		BuildingID: b, Name: name, Kind: kind, Capacity: capacity, Color: color, HourlyRate: rate, MinHours: min,
		BufferMinutes: buffer, IsBookable: true, IsActive: true, SortOrder: order, Equipment: equipment,
		Description: "", Rules: "יש להשאיר את החדר נקי ומסודר. אין להכניס אוכל ללא אישור.",
	})
	sd.err = err
	return r
}

func (sd *seeder) customer(name, kind, contact, phone, email string, userID *uuid.UUID) domain.Customer {
	if sd.err != nil {
		return domain.Customer{}
	}
	c, err := sd.tx.CreateCustomer(sd.ctx, store.CustomerInput{Name: name, Kind: kind, ContactName: ptr(contact),
		Phone: ptr(phone), Email: ptr(email), UserID: userID})
	sd.err = err
	return c
}

func (sd *seeder) booking(b domain.Booking) domain.Booking {
	if sd.err != nil {
		return domain.Booking{}
	}
	// Pre-check: an exclusion violation would abort the whole transaction.
	existing, err := sd.tx.BlockingBookings(sd.ctx, b.RoomID, b.StartsAt.Add(-4*time.Hour), b.BlockedUntil())
	if sd.err = err; err != nil || len(domain.FindConflicts(b, existing)) > 0 {
		return domain.Booking{}
	}
	out, err := sd.tx.CreateBooking(sd.ctx, b)
	sd.err = err
	return out
}

func (sd *seeder) run(today time.Time) {
	ctx, tx := sd.ctx, sd.tx
	sunday := today.AddDate(0, 0, -int(today.Weekday()))
	at := func(day time.Time, h, m int) time.Time {
		return time.Date(day.Year(), day.Month(), day.Day(), h, m, 0, 0, sd.loc)
	}

	// ── People ──
	sd.user("matanya@matnas.local", "מתניה כהן", "050-0000000", domain.RoleSuperAdmin, "")
	admin := sd.user("shiri@matnas.local", "שירי קאופמן", "050-1111111", domain.RoleAdmin, "")
	yosef := sd.user("yosef@matnas.local", "יוסף ברסי", "050-2222222", domain.RoleCoordinator, "")
	michael := sd.user("michael@matnas.local", "מיכאל קרפוך", "052-3333333", domain.RoleCoordinator, "")
	hana := sd.user("hana@matnas.local", "חנה שלזינגר", "054-4444444", domain.RoleCoordinator, "")
	sarahS := sd.user("sarah.s@matnas.local", "שרה שטיין", "050-3434343", domain.RoleCoordinator, "")
	leah := sd.user("leah@matnas.local", "לאה שלזינגר", "052-5656565", domain.RoleCoordinator, "")
	rachel := sd.user("rachel@matnas.local", "רחל ברנדמן", "054-7878787", domain.RoleCoordinator, "")
	sarahE := sd.user("sarah.e@matnas.local", "שרה אלמליח", "053-9090909", domain.RoleCoordinator, "")
	avi := sd.user("avi@matnas.local", "אבי לוי", "053-5555555", domain.RoleStaff, "cleaner")
	samir := sd.user("samir@matnas.local", "סמיר חסן", "058-6666666", domain.RoleStaff, "technician")
	dana := sd.user("dana@example.com", "דנה מזרחי", "050-7777777", domain.RoleUser, "")
	uri := sd.user("uri@example.com", "אורי פרץ", "052-8888888", domain.RoleUser, "")
	if sd.err != nil {
		return
	}

	// ── Facilities ──
	main, err := tx.CreateBuilding(ctx, domain.Building{Name: "המרכז הקהילתי מעלה רמות", Address: "רמות, ירושלים", OpensAt: "07:00", ClosesAt: "23:00"})
	if sd.err = err; err != nil {
		return
	}
	for _, e := range []domain.EquipmentItem{
		{Code: "sound", Label: "מערכת הגברה", FlatPrice: 350},
		{Code: "lighting", Label: "תאורת במה", FlatPrice: 300},
		{Code: "projector", Label: "מקרן ומסך", FlatPrice: 150},
		{Code: "catering", Label: "פינת כיבוד", FlatPrice: 200},
		{Code: "stage", Label: "במה ניידת", FlatPrice: 250},
		{Code: "chairs", Label: "ריהוט נוסף", FlatPrice: 120},
	} {
		if _, sd.err = tx.UpsertEquipment(ctx, e); sd.err != nil {
			return
		}
	}

	meeting := sd.room(main.ID, "חדר ישיבות", "meeting_room", 20, "#0369a1", 90, 1, 10, 1, "projector", "catering")
	kitchen := sd.room(main.ID, "מטבח צמיד", "other", 15, "#ea580c", 120, 1, 15, 2, "catering")
	craft := sd.room(main.ID, "חדר 11 יצירה", "classroom", 20, "#ca8a04", 100, 1, 10, 3, "projector")
	sewing := sd.room(main.ID, "חדר תפירה", "classroom", 12, "#db2777", 80, 1, 10, 4)
	lobby := sd.room(main.ID, "לובי", "event_hall", 120, "#7c3aed", 350, 2, 30, 5, "sound", "lighting", "stage", "catering", "chairs")
	jimbori := sd.room(main.ID, "ג׳מבורי", "studio", 25, "#16a34a", 150, 1, 15, 6, "sound")
	if sd.err != nil {
		return
	}

	// ── Customers ──
	yadbeyad := sd.customer("עמותת יד ביד", "organization", "שירה בן דוד", "02-5550101", "office@yadbeyad.org.il", nil)
	school := sd.customer("בית ספר אלון", "organization", "מנהלת: אורנה", "02-5550202", "alon@school.org.il", nil)
	golan := sd.customer("משפחת גולן", "private", "רונית גולן", "050-9990000", "ronit.golan@example.com", nil)
	danaC := sd.customer("דנה מזרחי", "private", "דנה מזרחי", "050-7777777", "dana@example.com", &dana.ID)
	internal := sd.customer("הנהלת המינהל", "internal", "שירי קאופמן", "050-1111111", "shiri@matnas.local", nil)
	if sd.err != nil {
		return
	}

	// ── Activities (weekly programs, one area per coordinator) ──
	type program struct {
		name     string
		audience string
		coord    domain.User
		room     domain.Room
		color    string
		days     []time.Weekday
		sh, sm   int
		eh, em   int
	}
	programs := []program{
		// קליטה
		{"אולפן עברית לעולים", "עולים חדשים", yosef, craft, "#0284c7", []time.Weekday{0, 2}, 9, 0, 11, 0},
		{"מפגש עולים חדשים", "עולים חדשים", yosef, meeting, "#0369a1", []time.Weekday{3}, 18, 0, 19, 30},
		// נוער
		{"מועדון נוער", "נוער", michael, lobby, "#9333ea", []time.Weekday{0, 2}, 18, 30, 21, 0},
		// צמיד
		{"סדנת בישול – צמיד", "משפחות צמיד", hana, kitchen, "#ea580c", []time.Weekday{1, 3}, 16, 0, 18, 0},
		{"מפגש הורים – צמיד", "משפחות צמיד", hana, meeting, "#c2410c", []time.Weekday{4}, 17, 0, 18, 30},
		// ספורט
		{"פילאטיס לנשים", "נשים", sarahS, jimbori, "#0f766e", []time.Weekday{0, 2}, 20, 0, 21, 0},
		{"התעמלות קרקע לילדים", "ילדים", sarahS, jimbori, "#14b8a6", []time.Weekday{1, 3}, 16, 30, 17, 30},
		// גיל הרך
		{"ג׳מבורי הורה וילד", "גיל הרך", leah, jimbori, "#16a34a", []time.Weekday{0, 2, 4}, 10, 0, 11, 30},
		{"שעת סיפור לפעוטות", "גיל הרך", leah, craft, "#65a30d", []time.Weekday{3}, 16, 30, 17, 30},
		// עבודה קהילתית
		{"פורום תושבים", "תושבי השכונה", rachel, meeting, "#475569", []time.Weekday{1}, 19, 30, 21, 0},
		{"מפגש גיל הזהב", "גיל הזהב", rachel, sewing, "#a16207", []time.Weekday{2}, 10, 0, 12, 0},
		// תרבות
		{"חוג קרמיקה", "מבוגרים", sarahE, craft, "#ca8a04", []time.Weekday{1, 4}, 18, 0, 20, 0},
		{"חוג תפירה למתחילות", "מבוגרים", sarahE, sewing, "#db2777", []time.Weekday{0, 3}, 17, 0, 19, 0},
	}
	for _, p := range programs {
		act, err := tx.SaveActivity(ctx, domain.Activity{Name: p.name, CoordinatorID: &p.coord.ID, DefaultRoomID: &p.room.ID,
			Color: p.color, IsActive: true, Audience: p.audience})
		if sd.err = err; err != nil {
			return
		}
		series := uuid.New()
		for week := -3; week <= 6; week++ {
			for _, wd := range p.days {
				day := sunday.AddDate(0, 0, week*7+int(wd))
				sd.booking(domain.Booking{
					RoomID: p.room.ID, Title: p.name, Type: domain.BookingActivity, Status: domain.StatusConfirmed,
					StartsAt: at(day, p.sh, p.sm), EndsAt: at(day, p.eh, p.em), BufferMinutes: p.room.BufferMinutes,
					ActivityID: &act.ID, CoordinatorID: &p.coord.ID, SeriesID: &series, Attendees: 15, CreatedBy: &admin.ID,
				})
			}
		}
	}

	// Weekly staff meeting
	series := uuid.New()
	for week := -3; week <= 6; week++ {
		day := sunday.AddDate(0, 0, week*7)
		sd.booking(domain.Booking{RoomID: meeting.ID, Title: "ישיבת צוות שבועית", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
			StartsAt: at(day, 8, 0), EndsAt: at(day, 9, 30), BufferMinutes: 10, CustomerID: &internal.ID, SeriesID: &series, CreatedBy: &admin.ID})
	}

	// ── One-off bookings this week & next ──
	day := func(offset int) time.Time { return sunday.AddDate(0, 0, offset) }
	fundraiser := sd.booking(domain.Booking{RoomID: lobby.ID, Title: "ערב התרמה – עמותת יד ביד", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(3), 19, 0), EndsAt: at(day(3), 23, 0), BufferMinutes: 30, CustomerID: &yadbeyad.ID, Attendees: 100,
		Equipment: []string{"sound", "lighting", "catering"}, Price: 2250, CreatedBy: &admin.ID, Notes: "כולל הקמת במה"})
	sd.booking(domain.Booking{RoomID: lobby.ID, Title: "כנס מתנדבים", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
		StartsAt: at(day(1), 10, 0), EndsAt: at(day(1), 13, 0), BufferMinutes: 30, CustomerID: &internal.ID, CoordinatorID: &rachel.ID, Attendees: 80, CreatedBy: &admin.ID})
	barmitzva := sd.booking(domain.Booking{RoomID: lobby.ID, Title: "בר מצווה – משפחת גולן", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(4), 18, 0), EndsAt: at(day(4), 23, 0), BufferMinutes: 30, CustomerID: &golan.ID, Attendees: 110,
		Equipment: []string{"sound", "lighting"}, Price: 2400, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: lobby.ID, Title: "ערב תרבות: הרצאה על ירושלים", Type: domain.BookingActivity, Status: domain.StatusConfirmed,
		StartsAt: at(day(1), 19, 0), EndsAt: at(day(1), 21, 0), BufferMinutes: 30, CoordinatorID: &sarahE.ID, Attendees: 60, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: jimbori.ID, Title: "ערב קריוקי לנוער", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
		StartsAt: at(day(4), 19, 30), EndsAt: at(day(4), 21, 30), BufferMinutes: 15, CoordinatorID: &michael.ID, Attendees: 25, CreatedBy: &admin.ID})
	schoolB := sd.booking(domain.Booking{RoomID: craft.ID, Title: "סדנת יצירה – בית ספר אלון", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(2), 12, 0), EndsAt: at(day(2), 14, 0), BufferMinutes: 10, CustomerID: &school.ID, Attendees: 20, Price: 800, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: kitchen.ID, Title: "החלפת כיריים במטבח", Type: domain.BookingMaintenance, Status: domain.StatusConfirmed,
		StartsAt: at(day(2), 7, 0), EndsAt: at(day(2), 12, 0), CreatedBy: &admin.ID, InternalNotes: "קבלן: מטבחי ירושלים"})
	sd.booking(domain.Booking{RoomID: meeting.ID, Title: "סדנת כתיבה לעולים", Type: domain.BookingActivity, Status: domain.StatusPending,
		StartsAt: at(day(2), 17, 0), EndsAt: at(day(2), 19, 0), BufferMinutes: 10, CoordinatorID: &yosef.ID, Attendees: 10, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: meeting.ID, Title: "ישיבת ועד שכונה", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
		StartsAt: at(day(2), 20, 0), EndsAt: at(day(2), 21, 30), BufferMinutes: 10, CustomerID: &internal.ID, CoordinatorID: &rachel.ID, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: jimbori.ID, Title: "יום הולדת 5 – דנה מזרחי", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(5), 10, 0), EndsAt: at(day(5), 13, 0), BufferMinutes: 15, CustomerID: &danaC.ID, Attendees: 25,
		Equipment: []string{"sound"}, Price: 800, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: lobby.ID, Title: "יריד קהילתי", Type: domain.BookingActivity, Status: domain.StatusConfirmed,
		StartsAt: at(day(8), 16, 0), EndsAt: at(day(8), 21, 0), BufferMinutes: 30, CoordinatorID: &sarahE.ID, Attendees: 120, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: sewing.ID, Title: "סדנת סריגה פרטית", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(0), 13, 0), EndsAt: at(day(0), 15, 0), BufferMinutes: 10, CustomerID: &danaC.ID, Price: 160, CreatedBy: &admin.ID})
	// Something happening right now for the live signage.
	now := time.Now().In(sd.loc)
	if now.Hour() >= 7 && now.Hour() < 22 {
		start := now.Truncate(30 * time.Minute).Add(-30 * time.Minute)
		sd.booking(domain.Booking{RoomID: meeting.ID, Title: "סדנת מחשבים לגיל השלישי", Type: domain.BookingActivity, Status: domain.StatusConfirmed,
			StartsAt: start, EndsAt: start.Add(2 * time.Hour), BufferMinutes: 10, CoordinatorID: &rachel.ID, Attendees: 12, CreatedBy: &admin.ID})
	}
	if sd.err != nil {
		return
	}

	// ── Payments ──
	for _, p := range []domain.Payment{
		{BookingID: fundraiser.ID, Amount: 1000, Method: domain.PayBankTransfer, Reference: "העברה 88213", PaidAt: today.AddDate(0, 0, -5)},
		{BookingID: barmitzva.ID, Amount: 1200, Method: domain.PayCheck, Reference: "צ׳ק 004512", PaidAt: today.AddDate(0, 0, -10)},
		{BookingID: barmitzva.ID, Amount: 1200, Method: domain.PayCheck, Reference: "צ׳ק 004513", PaidAt: today.AddDate(0, 0, -2)},
		{BookingID: schoolB.ID, Amount: 400, Method: domain.PayCash, PaidAt: today.AddDate(0, 0, -1)},
	} {
		if p.BookingID == uuid.Nil {
			continue
		}
		p.RecordedBy = &admin.ID
		if _, sd.err = tx.CreatePayment(ctx, p); sd.err != nil {
			return
		}
	}

	// ── Portal requests ──
	for _, r := range []domain.BookingRequest{
		{UserID: dana.ID, CustomerID: &danaC.ID, RoomID: lobby.ID, Title: "יום הולדת 40", EventKind: "אירוע פרטי",
			StartsAt: at(day(11), 19, 0), EndsAt: at(day(11), 23, 0), Attendees: 90, Equipment: []string{"sound", "catering"}, Notes: "נשמח לסידור שולחנות עגולים", QuotedPrice: 1950},
		{UserID: uri.ID, RoomID: meeting.ID, Title: "סדנת יזמות", EventKind: "סדנה",
			StartsAt: at(day(7), 18, 0), EndsAt: at(day(7), 21, 0), Attendees: 18, Equipment: []string{"projector"}, QuotedPrice: 420},
		{UserID: yosef.ID, RoomID: lobby.ID, Title: "טקס קבלת פנים לעולים חדשים", EventKind: "פעילות קהילתית", Route: "activity",
			StartsAt: at(day(10), 10, 0), EndsAt: at(day(10), 13, 0), Attendees: 70, TablesCount: 6, ChairsCount: 70, Equipment: []string{"sound"}, Notes: "צריך גם פינת כיבוד קטנה"},
		{UserID: uri.ID, RoomID: jimbori.ID, Title: "חזרה להופעת ריקוד", EventKind: "חזרה",
			StartsAt: at(day(7), 17, 30), EndsAt: at(day(7), 19, 0), Attendees: 12, QuotedPrice: 225, Notes: "אפשר גם בשעה מאוחרת יותר"},
	} {
		if _, sd.err = tx.CreateRequest(ctx, r); sd.err != nil {
			return
		}
	}

	// ── Staff tasks ──
	for _, sch := range []domain.TaskSchedule{
		{Title: "ניקיון הלובי", Kind: "cleaning", RoomID: &lobby.ID, AssigneeID: &avi.ID, Weekdays: []int{0, 1, 2, 3, 4, 5}, TimeOfDay: "07:00", DurationMinutes: 60, IsActive: true},
		{Title: "ניקיון חדר ישיבות", Kind: "cleaning", RoomID: &meeting.ID, AssigneeID: &avi.ID, Weekdays: []int{0, 2, 4}, TimeOfDay: "14:00", DurationMinutes: 45, IsActive: true},
		{Title: "ניקיון מטבח צמיד", Kind: "cleaning", RoomID: &kitchen.ID, AssigneeID: &avi.ID, Weekdays: []int{1, 3}, TimeOfDay: "18:30", DurationMinutes: 45, IsActive: true},
		{Title: "בדיקת מערכת הגברה בלובי", Kind: "inspection", RoomID: &lobby.ID, AssigneeID: &samir.ID, Weekdays: []int{4}, TimeOfDay: "15:00", DurationMinutes: 60, IsActive: true},
	} {
		saved, err := tx.SaveSchedule(ctx, sch)
		if sd.err = err; err != nil {
			return
		}
		for _, due := range domain.Occurrences(saved, today, today.AddDate(0, 0, 7), sd.loc) {
			if _, sd.err = tx.MaterializeTask(ctx, saved, due); sd.err != nil {
				return
			}
		}
	}
	for _, t := range []domain.Task{
		{Title: "החלפת נורה בחדר תפירה", Kind: "repair", RoomID: &sewing.ID, AssigneeID: &samir.ID, DueAt: at(today, 12, 0), Description: "נורת לד בתקרה מהבהבת"},
		{Title: "סידור 100 כיסאות לערב התרמה", Kind: "setup", RoomID: &lobby.ID, AssigneeID: &avi.ID, DueAt: at(day(3), 16, 0), BookingID: &fundraiser.ID},
		{Title: "תיקון ברז במטבח צמיד", Kind: "repair", RoomID: &kitchen.ID, AssigneeID: &samir.ID, DueAt: at(today.AddDate(0, 0, 1), 9, 0)},
	} {
		if _, sd.err = tx.CreateTask(ctx, t, &admin.ID); sd.err != nil {
			return
		}
	}
	// Tickets opened by coordinators: one still waiting for a worker.
	for _, t := range []domain.Task{
		{Title: "המקרן בחדר 11 לא נדלק", Kind: "repair", RoomID: &craft.ID, DueAt: at(today, 8, 30), ReportedBy: &yosef.ID,
			Description: "נורית אדומה מהבהבת, ניסיתי לנתק ולחבר מחדש"},
		{Title: "חסר טונר במדפסת של חדר הרכזים", Kind: "supplies", DueAt: at(today, 9, 0), ReportedBy: &hana.ID},
		{Title: "ניקיון הג׳מבורי אחרי פעילות", Kind: "cleaning", RoomID: &jimbori.ID, AssigneeID: &avi.ID, DueAt: at(today, 13, 0), ReportedBy: &leah.ID,
			Description: "נשפכו משקאות על המזרנים"},
	} {
		if _, sd.err = tx.CreateTask(ctx, t, t.ReportedBy); sd.err != nil {
			return
		}
	}

	// ── Signage ──
	if _, sd.err = tx.SaveScreen(ctx, domain.SignageScreen{Slug: "lobby", Name: "מסך לובי ראשי", BuildingID: &main.ID, Theme: "dark",
		WelcomeText: "ברוכים הבאים למינהל הקהילתי מעלה רמות", RotateSeconds: 12, IsActive: true}); sd.err != nil {
		return
	}
	for _, a := range []domain.Announcement{
		{Title: "ההרשמה לחוגי החורף נפתחה!", Body: "מגוון חוגים לכל הגילאים – הרשמה במזכירות או באתר", StartsAt: today.AddDate(0, 0, -7), Priority: 2},
		{Title: "יריד קהילתי", Body: fmt.Sprintf("ביום %s בשעה 16:00 בלובי – הכניסה חופשית", day(8).Format("02/01")), StartsAt: today.AddDate(0, 0, -3), Priority: 1},
		{Title: "שימו לב", Body: "החניה הצפונית סגורה לרגל עבודות עד סוף החודש", StartsAt: today.AddDate(0, 0, -1)},
	} {
		if _, sd.err = tx.SaveAnnouncement(ctx, a, &admin.ID); sd.err != nil {
			return
		}
	}
}
