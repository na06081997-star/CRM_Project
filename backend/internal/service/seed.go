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
	sd.user("owner@matnas.local", "מנהל־על (בעלים)", "050-0000000", domain.RoleSuperAdmin, "")
	admin := sd.user("maya@matnas.local", "מאיה שגיא", "050-1111111", domain.RoleAdmin, "")
	yoav := sd.user("yoav@matnas.local", "יואב כהן", "050-2222222", domain.RoleCoordinator, "")
	rotem := sd.user("rotem@matnas.local", "רותם אזולאי", "052-3333333", domain.RoleCoordinator, "")
	amit := sd.user("amit@matnas.local", "עמית ברק", "054-4444444", domain.RoleCoordinator, "")
	avi := sd.user("avi@matnas.local", "אבי לוי", "053-5555555", domain.RoleStaff, "cleaner")
	samir := sd.user("samir@matnas.local", "סמיר חסן", "058-6666666", domain.RoleStaff, "technician")
	dana := sd.user("dana@example.com", "דנה מזרחי", "050-7777777", domain.RoleUser, "")
	uri := sd.user("uri@example.com", "אורי פרץ", "052-8888888", domain.RoleUser, "")
	if sd.err != nil {
		return
	}

	// ── Facilities ──
	main, err := tx.CreateBuilding(ctx, domain.Building{Name: "מתנ״ס נווה אורנים", Address: "רחוב האורנים 12", OpensAt: "07:00", ClosesAt: "23:00"})
	if sd.err = err; err != nil {
		return
	}
	music, err := tx.CreateBuilding(ctx, domain.Building{Name: "בית המוזיקה", Address: "רחוב הזמר 3", OpensAt: "08:00", ClosesAt: "22:00"})
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

	hall := sd.room(main.ID, "אולם אירועים", "event_hall", 250, "#7c3aed", 450, 3, 30, 1, "sound", "lighting", "stage", "catering", "projector")
	sports := sd.room(main.ID, "אולם ספורט", "sports_hall", 120, "#0f766e", 220, 1, 15, 2, "sound")
	meet1 := sd.room(main.ID, "חדר ישיבות 1", "meeting_room", 20, "#0369a1", 90, 1, 10, 3, "projector", "catering")
	meet2 := sd.room(main.ID, "חדר ישיבות 2", "meeting_room", 12, "#0891b2", 70, 1, 10, 4, "projector")
	studio := sd.room(main.ID, "סטודיו מחול", "studio", 30, "#db2777", 150, 1, 15, 5, "sound")
	classroom := sd.room(main.ID, "חדר חוגים", "classroom", 25, "#ca8a04", 100, 1, 10, 6, "projector")
	musicA := sd.room(music.ID, "חדר מוזיקה א׳", "music_room", 8, "#ea580c", 80, 1, 10, 7, "sound")
	musicB := sd.room(music.ID, "חדר מוזיקה ב׳", "music_room", 6, "#65a30d", 70, 1, 10, 8)
	rec := sd.room(music.ID, "חדר הקלטות", "studio", 5, "#be123c", 180, 2, 15, 9, "sound")
	if sd.err != nil {
		return
	}

	// ── Customers ──
	yadbeyad := sd.customer("עמותת יד ביד", "organization", "שירה בן דוד", "03-5550101", "office@yadbeyad.org.il", nil)
	school := sd.customer("בית ספר אלון", "organization", "מנהלת: אורנה", "03-5550202", "alon@school.org.il", nil)
	golan := sd.customer("משפחת גולן", "private", "רונית גולן", "050-9990000", "ronit.golan@example.com", nil)
	danaC := sd.customer("דנה מזרחי", "private", "דנה מזרחי", "050-7777777", "dana@example.com", &dana.ID)
	internal := sd.customer("הנהלת המתנ״ס", "internal", "מאיה שגיא", "050-1111111", "maya@matnas.local", nil)
	if sd.err != nil {
		return
	}

	// ── Activities (weekly programs) ──
	type program struct {
		name   string
		coord  domain.User
		room   domain.Room
		color  string
		days   []time.Weekday
		sh, sm int
		eh, em int
	}
	programs := []program{
		{"חוג כדורסל", yoav, sports, "#0f766e", []time.Weekday{0, 2}, 18, 0, 19, 30},
		{"בלט לגיל הרך", rotem, studio, "#db2777", []time.Weekday{1, 3}, 16, 0, 17, 0},
		{"היפ-הופ נוער", rotem, studio, "#9333ea", []time.Weekday{0, 4}, 17, 15, 18, 30},
		{"יוגה בוקר", amit, studio, "#16a34a", []time.Weekday{5}, 9, 0, 10, 30},
		{"שיעור גיטרה פרטי", amit, musicA, "#ea580c", []time.Weekday{2}, 17, 0, 18, 0},
		{"מקהלת גיל הזהב", amit, musicB, "#65a30d", []time.Weekday{1}, 10, 0, 11, 30},
		{"קרמיקה למבוגרים", yoav, classroom, "#ca8a04", []time.Weekday{3}, 19, 0, 21, 0},
	}
	for _, p := range programs {
		act, err := tx.SaveActivity(ctx, domain.Activity{Name: p.name, CoordinatorID: &p.coord.ID, DefaultRoomID: &p.room.ID,
			Color: p.color, IsActive: true, Audience: "קהל רחב"})
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
		sd.booking(domain.Booking{RoomID: meet1.ID, Title: "ישיבת צוות שבועית", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
			StartsAt: at(day, 8, 30), EndsAt: at(day, 10, 0), BufferMinutes: 10, CustomerID: &internal.ID, SeriesID: &series, CreatedBy: &admin.ID})
	}

	// ── One-off bookings this week & next ──
	day := func(offset int) time.Time { return sunday.AddDate(0, 0, offset) }
	fundraiser := sd.booking(domain.Booking{RoomID: hall.ID, Title: "ערב התרמה – עמותת יד ביד", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(3), 19, 0), EndsAt: at(day(3), 23, 0), BufferMinutes: 30, CustomerID: &yadbeyad.ID, Attendees: 180,
		Equipment: []string{"sound", "lighting", "catering"}, Price: 2650, CreatedBy: &admin.ID, Notes: "כולל הקמת במה"})
	sd.booking(domain.Booking{RoomID: hall.ID, Title: "כנס מתנדבים", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
		StartsAt: at(day(1), 10, 0), EndsAt: at(day(1), 13, 0), BufferMinutes: 30, CustomerID: &internal.ID, CoordinatorID: &yoav.ID, Attendees: 80, CreatedBy: &admin.ID})
	barmitzva := sd.booking(domain.Booking{RoomID: hall.ID, Title: "בר מצווה – משפחת גולן", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(4), 18, 0), EndsAt: at(day(4), 23, 0), BufferMinutes: 30, CustomerID: &golan.ID, Attendees: 150,
		Equipment: []string{"sound", "lighting"}, Price: 2900, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: hall.ID, Title: "הרצאה: גינון עירוני", Type: domain.BookingActivity, Status: domain.StatusConfirmed,
		StartsAt: at(day(2), 18, 0), EndsAt: at(day(2), 20, 0), BufferMinutes: 30, CoordinatorID: &rotem.ID, Attendees: 60, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: classroom.ID, Title: "ערב קריוקי לנוער", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
		StartsAt: at(day(1), 19, 30), EndsAt: at(day(1), 21, 30), BufferMinutes: 10, CoordinatorID: &amit.ID, Attendees: 25, CreatedBy: &admin.ID})
	schoolB := sd.booking(domain.Booking{RoomID: sports.ID, Title: "טורניר כדורעף – בית ספר אלון", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(2), 9, 0), EndsAt: at(day(2), 13, 0), BufferMinutes: 15, CustomerID: &school.ID, Attendees: 90, Price: 880, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: sports.ID, Title: "תחזוקת רצפת פרקט", Type: domain.BookingMaintenance, Status: domain.StatusConfirmed,
		StartsAt: at(day(3), 7, 0), EndsAt: at(day(3), 12, 0), CreatedBy: &admin.ID, InternalNotes: "קבלן: פרקט הצפון"})
	sd.booking(domain.Booking{RoomID: meet2.ID, Title: "סדנת כתיבה יוצרת", Type: domain.BookingActivity, Status: domain.StatusPending,
		StartsAt: at(day(2), 17, 0), EndsAt: at(day(2), 19, 0), BufferMinutes: 10, CoordinatorID: &rotem.ID, Attendees: 10, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: meet1.ID, Title: "פגישת ועד הורים", Type: domain.BookingInternal, Status: domain.StatusConfirmed,
		StartsAt: at(day(1), 20, 0), EndsAt: at(day(1), 21, 30), BufferMinutes: 10, CustomerID: &internal.ID, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: rec.ID, Title: "הקלטת אלבום – להקת 'גלים'", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(4), 10, 0), EndsAt: at(day(4), 16, 0), BufferMinutes: 15, CustomerID: &danaC.ID, Price: 1080, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: hall.ID, Title: "יריד קהילתי", Type: domain.BookingActivity, Status: domain.StatusConfirmed,
		StartsAt: at(day(9), 16, 0), EndsAt: at(day(9), 21, 0), BufferMinutes: 30, CoordinatorID: &yoav.ID, Attendees: 220, CreatedBy: &admin.ID})
	sd.booking(domain.Booking{RoomID: musicA.ID, Title: "חזרות להקה", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: at(day(0), 16, 0), EndsAt: at(day(0), 19, 0), BufferMinutes: 10, CustomerID: &danaC.ID, Price: 240, CreatedBy: &admin.ID})
	// Something happening right now for the live signage.
	now := time.Now().In(sd.loc)
	if now.Hour() >= 7 && now.Hour() < 22 {
		start := now.Truncate(30 * time.Minute).Add(-30 * time.Minute)
		sd.booking(domain.Booking{RoomID: meet2.ID, Title: "סדנת מחשבים לגיל השלישי", Type: domain.BookingActivity, Status: domain.StatusConfirmed,
			StartsAt: start, EndsAt: start.Add(2 * time.Hour), BufferMinutes: 10, CoordinatorID: &amit.ID, Attendees: 12, CreatedBy: &admin.ID})
	}
	if sd.err != nil {
		return
	}

	// ── Payments ──
	for _, p := range []domain.Payment{
		{BookingID: fundraiser.ID, Amount: 1000, Method: domain.PayBankTransfer, Reference: "העברה 88213", PaidAt: today.AddDate(0, 0, -5)},
		{BookingID: barmitzva.ID, Amount: 1500, Method: domain.PayCheck, Reference: "צ׳ק 004512", PaidAt: today.AddDate(0, 0, -10)},
		{BookingID: barmitzva.ID, Amount: 1400, Method: domain.PayCheck, Reference: "צ׳ק 004513", PaidAt: today.AddDate(0, 0, -2)},
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
		{UserID: dana.ID, CustomerID: &danaC.ID, RoomID: hall.ID, Title: "יום הולדת 40", EventKind: "אירוע פרטי",
			StartsAt: at(day(11), 19, 0), EndsAt: at(day(11), 23, 0), Attendees: 100, Equipment: []string{"sound", "catering"}, Notes: "נשמח לסידור שולחנות עגולים", QuotedPrice: 2350},
		{UserID: uri.ID, RoomID: meet1.ID, Title: "סדנת יזמות", EventKind: "סדנה",
			StartsAt: at(day(8), 18, 0), EndsAt: at(day(8), 21, 0), Attendees: 18, Equipment: []string{"projector"}, QuotedPrice: 420},
		{UserID: yoav.ID, RoomID: sports.ID, Title: "טורניר כדורסל חנוכה", EventKind: "פעילות קהילתית", Route: "activity",
			StartsAt: at(day(10), 9, 0), EndsAt: at(day(10), 13, 0), Attendees: 60, TablesCount: 4, ChairsCount: 80, Equipment: []string{"sound"}, Notes: "צריך גם שולחן ניקוד"},
		{UserID: uri.ID, RoomID: studio.ID, Title: "חזרה להופעת ריקוד", EventKind: "חזרה",
			StartsAt: at(day(7), 17, 30), EndsAt: at(day(7), 19, 0), Attendees: 12, QuotedPrice: 225, Notes: "מתנגש עם היפ-הופ? אפשר גם 19:00"},
	} {
		if _, sd.err = tx.CreateRequest(ctx, r); sd.err != nil {
			return
		}
	}

	// ── Staff tasks ──
	for _, sch := range []domain.TaskSchedule{
		{Title: "ניקיון אולם אירועים", Kind: "cleaning", RoomID: &hall.ID, AssigneeID: &avi.ID, Weekdays: []int{0, 1, 2, 3, 4, 5}, TimeOfDay: "07:00", DurationMinutes: 90, IsActive: true},
		{Title: "ניקיון חדרי ישיבות", Kind: "cleaning", RoomID: &meet1.ID, AssigneeID: &avi.ID, Weekdays: []int{0, 2, 4}, TimeOfDay: "14:00", DurationMinutes: 45, IsActive: true},
		{Title: "בדיקת מערכת הגברה", Kind: "inspection", RoomID: &hall.ID, AssigneeID: &samir.ID, Weekdays: []int{4}, TimeOfDay: "15:00", DurationMinutes: 60, IsActive: true},
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
		{Title: "החלפת נורה בחדר ישיבות 2", Kind: "repair", RoomID: &meet2.ID, AssigneeID: &samir.ID, DueAt: at(today, 12, 0), Description: "נורת לד בתקרה מהבהבת"},
		{Title: "סידור 180 כיסאות לערב התרמה", Kind: "setup", RoomID: &hall.ID, AssigneeID: &avi.ID, DueAt: at(day(3), 16, 0), BookingID: &fundraiser.ID},
		{Title: "תיקון ברז בשירותי אולם ספורט", Kind: "repair", RoomID: &sports.ID, AssigneeID: &samir.ID, DueAt: at(today.AddDate(0, 0, 1), 9, 0)},
	} {
		if _, sd.err = tx.CreateTask(ctx, t, &admin.ID); sd.err != nil {
			return
		}
	}

	// ── Signage ──
	if _, sd.err = tx.SaveScreen(ctx, domain.SignageScreen{Slug: "lobby", Name: "מסך לובי ראשי", BuildingID: &main.ID, Theme: "dark",
		WelcomeText: "ברוכים הבאים למתנ״ס נווה אורנים", RotateSeconds: 12, IsActive: true}); sd.err != nil {
		return
	}
	if _, sd.err = tx.SaveScreen(ctx, domain.SignageScreen{Slug: "music", Name: "מסך בית המוזיקה", BuildingID: &music.ID, Theme: "light",
		WelcomeText: "בית המוזיקה – נווה אורנים", RotateSeconds: 15, IsActive: true}); sd.err != nil {
		return
	}
	for _, a := range []domain.Announcement{
		{Title: "ההרשמה לחוגי החורף נפתחה!", Body: "מגוון חוגים לכל הגילאים – הרשמה במזכירות או באתר", StartsAt: today.AddDate(0, 0, -7), Priority: 2},
		{Title: "יריד קהילתי", Body: fmt.Sprintf("ביום %s בשעה 16:00 באולם האירועים – הכניסה חופשית", day(9).Format("02/01")), StartsAt: today.AddDate(0, 0, -3), Priority: 1},
		{Title: "שימו לב", Body: "החניה הצפונית סגורה לרגל עבודות עד סוף החודש", StartsAt: today.AddDate(0, 0, -1)},
	} {
		if _, sd.err = tx.SaveAnnouncement(ctx, a, &admin.ID); sd.err != nil {
			return
		}
	}
}
