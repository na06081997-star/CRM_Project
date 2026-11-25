package service

import (
	"context"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
	"github.com/na06081997-star/CRM_Project/backend/internal/xlsx"
)

// Excel exports. They reuse the list services, so an export always holds
// exactly what the same user sees on screen with the same filters: a רכז or
// a portal user only gets their own rows, and only managers get the money.

var hebrewWeekday = [...]string{"ראשון", "שני", "שלישי", "רביעי", "חמישי", "שישי", "שבת"}

var bookingTypeHe = map[domain.BookingType]string{
	domain.BookingActivity:    "פעילות באמצעות רכז",
	domain.BookingRental:      "שכירות בתשלום",
	domain.BookingInternal:    "הזמנה פנימית",
	domain.BookingMaintenance: "תחזוקה",
	domain.BookingClosed:      "סגור / לא זמין",
}

var bookingStatusHe = map[domain.BookingStatus]string{
	domain.StatusPending:   "ממתין לאישור",
	domain.StatusConfirmed: "מאושר",
	domain.StatusCancelled: "בוטל",
	domain.StatusRejected:  "נדחה",
}

var paymentMethodHe = map[domain.PaymentMethod]string{
	"bank_transfer": "העברה בנקאית",
	"check":         "צ׳ק",
	"cash":          "מזומן",
	"credit_card":   "כרטיס אשראי",
}

func strOr(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// bookingsSheet lays out bookings one per row. withMoney adds price, paid
// and balance columns with a totals row.
func bookingsSheet(list []domain.Booking, buildings map[uuid.UUID]string, withMoney bool, loc *time.Location) xlsx.Sheet {
	cols := []xlsx.Column{
		{Title: "תאריך", Kind: xlsx.Date, Width: 12},
		{Title: "יום", Kind: xlsx.Text, Width: 8},
		{Title: "התחלה", Kind: xlsx.Time, Width: 8},
		{Title: "סיום", Kind: xlsx.Time, Width: 8},
		{Title: "חדר", Kind: xlsx.Text, Width: 18},
		{Title: "מבנה", Kind: xlsx.Text, Width: 16},
		{Title: "כותרת", Kind: xlsx.Text, Width: 28},
		{Title: "סוג", Kind: xlsx.Text, Width: 18},
		{Title: "סטטוס", Kind: xlsx.Text, Width: 13},
		{Title: "רכז/ת", Kind: xlsx.Text, Width: 16},
		{Title: "לקוח", Kind: xlsx.Text, Width: 20},
		{Title: "משתתפים", Kind: xlsx.Number, Width: 10},
	}
	if withMoney {
		cols = append(cols,
			xlsx.Column{Title: "מחיר", Kind: xlsx.Shekel, Width: 12},
			xlsx.Column{Title: "שולם", Kind: xlsx.Shekel, Width: 12},
			xlsx.Column{Title: "יתרה", Kind: xlsx.Shekel, Width: 12},
		)
	}
	cols = append(cols, xlsx.Column{Title: "הערות", Kind: xlsx.Text, Width: 30})

	rows := make([][]any, 0, len(list))
	for _, b := range list {
		row := []any{
			b.StartsAt, hebrewWeekday[b.StartsAt.In(loc).Weekday()], b.StartsAt, b.EndsAt,
			b.RoomName, buildings[b.BuildingID], b.Title,
			bookingTypeHe[b.Type], bookingStatusHe[b.Status],
			strOr(b.CoordinatorName), strOr(b.CustomerName), b.Attendees,
		}
		if withMoney {
			row = append(row, b.Price, b.PaidAmount, b.Balance())
		}
		row = append(row, b.Notes)
		rows = append(rows, row)
	}
	return xlsx.Sheet{Name: "הזמנות", Columns: cols, Rows: rows, TotalsLabel: `סה"כ`}
}

func paymentsSheet(list []domain.Payment) xlsx.Sheet {
	rows := make([][]any, 0, len(list))
	for _, p := range list {
		method, ok := paymentMethodHe[p.Method]
		if !ok {
			method = string(p.Method)
		}
		rows = append(rows, []any{p.PaidAt, strOr(p.CustomerName), p.BookingTitle, p.Amount, method, p.Reference, p.Notes})
	}
	return xlsx.Sheet{
		Name: "תשלומים",
		Columns: []xlsx.Column{
			{Title: "תאריך", Kind: xlsx.Date, Width: 12},
			{Title: "לקוח", Kind: xlsx.Text, Width: 20},
			{Title: "הזמנה", Kind: xlsx.Text, Width: 28},
			{Title: "סכום", Kind: xlsx.Shekel, Width: 12},
			{Title: "אמצעי תשלום", Kind: xlsx.Text, Width: 15},
			{Title: "אסמכתא", Kind: xlsx.Text, Width: 14},
			{Title: "הערות", Kind: xlsx.Text, Width: 30},
		},
		Rows:        rows,
		TotalsLabel: `סה"כ`,
	}
}

func balancesSheet(list []domain.Booking) xlsx.Sheet {
	rows := make([][]any, 0, len(list))
	for _, b := range list {
		rows = append(rows, []any{b.StartsAt, b.Title, strOr(b.CustomerName), b.RoomName, b.Price, b.PaidAmount, b.Balance()})
	}
	return xlsx.Sheet{
		Name: "יתרות",
		Columns: []xlsx.Column{
			{Title: "תאריך", Kind: xlsx.Date, Width: 12},
			{Title: "הזמנה", Kind: xlsx.Text, Width: 28},
			{Title: "לקוח", Kind: xlsx.Text, Width: 20},
			{Title: "חדר", Kind: xlsx.Text, Width: 18},
			{Title: "מחיר", Kind: xlsx.Shekel, Width: 12},
			{Title: "שולם", Kind: xlsx.Shekel, Width: 12},
			{Title: "יתרה", Kind: xlsx.Shekel, Width: 12},
		},
		Rows:        rows,
		TotalsLabel: `סה"כ`,
	}
}

// ExportBookings returns the calendar list for the given filters as a workbook.
func (s *Service) ExportBookings(ctx context.Context, actor Actor, f store.BookingFilter) (xlsx.Workbook, error) {
	list, err := s.ListBookings(ctx, actor, f)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	buildings := map[uuid.UUID]string{}
	if bs, err := s.store.ListBuildings(ctx); err == nil {
		for _, b := range bs {
			buildings[b.ID] = b.Name
		}
	}
	return xlsx.Workbook{Location: s.loc, Sheets: []xlsx.Sheet{
		bookingsSheet(list, buildings, actor.Can(domain.ActManageBookings), s.loc),
	}}, nil
}

// ExportPayments returns the payments matching the filters and the open
// balances (of the selected customer, if any) as a two-sheet workbook.
func (s *Service) ExportPayments(ctx context.Context, actor Actor, f store.PaymentFilter) (xlsx.Workbook, error) {
	f.Limit = 0
	payments, err := s.ListPayments(ctx, actor, f)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	balances, err := s.Balances(ctx, actor, f.CustomerID)
	if err != nil {
		return xlsx.Workbook{}, err
	}
	return xlsx.Workbook{Location: s.loc, Sheets: []xlsx.Sheet{paymentsSheet(payments), balancesSheet(balances)}}, nil
}
