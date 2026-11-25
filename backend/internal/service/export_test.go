package service

import (
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/xlsx"
)

func sheetWorkbook(s xlsx.Sheet, loc *time.Location) xlsx.Workbook {
	return xlsx.Workbook{Location: loc, Sheets: []xlsx.Sheet{s}}
}

func colIndex(t *testing.T, titles []string, title string) int {
	t.Helper()
	for i, c := range titles {
		if c == title {
			return i
		}
	}
	return -1
}

func TestBookingsSheetRows(t *testing.T) {
	loc, _ := time.LoadLocation("Asia/Jerusalem")
	building := uuid.New()
	start := time.Date(2026, 11, 29, 18, 0, 0, 0, loc) // a Sunday
	list := []domain.Booking{{
		Title: "חוג ציור", Type: domain.BookingRental, Status: domain.StatusConfirmed,
		StartsAt: start, EndsAt: start.Add(2 * time.Hour), RoomName: "סטודיו", BuildingID: building,
		CoordinatorName: ptr("יוסף"), CustomerName: ptr("משפחת כהן"), Attendees: 12,
		Price: 500, PaidAmount: 200, Notes: "להביא כיסאות",
	}}

	s := bookingsSheet(list, map[uuid.UUID]string{building: "מבנה ראשי"}, true, loc)
	if s.Name != "הזמנות" || len(s.Rows) != 1 || s.TotalsLabel == "" {
		t.Fatalf("unexpected sheet: %+v", s)
	}
	var titles []string
	for _, c := range s.Columns {
		titles = append(titles, c.Title)
	}
	row := s.Rows[0]
	want := map[string]any{
		"יום": "ראשון", "חדר": "סטודיו", "מבנה": "מבנה ראשי", "כותרת": "חוג ציור",
		"סוג": "שכירות בתשלום", "סטטוס": "מאושר", "רכז/ת": "יוסף", "לקוח": "משפחת כהן",
		"משתתפים": 12, "מחיר": 500.0, "שולם": 200.0, "יתרה": 300.0, "הערות": "להביא כיסאות",
	}
	for title, v := range want {
		i := colIndex(t, titles, title)
		if i < 0 {
			t.Errorf("missing column %s", title)
			continue
		}
		if row[i] != v {
			t.Errorf("%s = %v, want %v", title, row[i], v)
		}
	}
	if len(row) != len(s.Columns) {
		t.Fatalf("row has %d cells for %d columns", len(row), len(s.Columns))
	}
	if _, err := (sheetWorkbook(s, loc)).Bytes(); err != nil {
		t.Fatalf("sheet does not encode: %v", err)
	}
}

func TestBookingsSheetHidesMoneyFromNonManagers(t *testing.T) {
	s := bookingsSheet([]domain.Booking{{Title: "x", StartsAt: time.Now(), EndsAt: time.Now().Add(time.Hour)}}, nil, false, time.UTC)
	for _, c := range s.Columns {
		if c.Title == "מחיר" || c.Title == "שולם" || c.Title == "יתרה" {
			t.Errorf("money column %s must not be exported without bookings:manage", c.Title)
		}
	}
	if len(s.Rows[0]) != len(s.Columns) {
		t.Fatal("row and columns differ")
	}
}

func TestPaymentAndBalanceSheets(t *testing.T) {
	p := paymentsSheet([]domain.Payment{{Amount: 150, Method: domain.PayCheck, Reference: "1234", PaidAt: time.Now(), BookingTitle: "אירוע", CustomerName: ptr("כהן")}})
	if p.Name != "תשלומים" || p.Rows[0][3] != 150.0 || p.Rows[0][4] != "צ׳ק" || p.Rows[0][5] != "1234" {
		t.Fatalf("payments sheet: %+v", p.Rows)
	}
	b := balancesSheet([]domain.Booking{{Title: "אירוע", Price: 1000, PaidAmount: 150, StartsAt: time.Now()}})
	if b.Name != "יתרות" || b.Rows[0][6] != 850.0 {
		t.Fatalf("balances sheet: %+v", b.Rows)
	}
	// Empty instance: both sheets still encode with headers only.
	if _, err := sheetWorkbook(paymentsSheet(nil), time.UTC).Bytes(); err != nil {
		t.Fatal(err)
	}
	if _, err := sheetWorkbook(balancesSheet(nil), time.UTC).Bytes(); err != nil {
		t.Fatal(err)
	}
}
