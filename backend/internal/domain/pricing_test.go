package domain

import "testing"

func TestQuote(t *testing.T) {
	prices := map[string]float64{"sound": 300, "lighting": 250}
	tests := []struct {
		name      string
		in        PriceInput
		billable  float64
		total     float64
		lineCount int
	}{
		{
			name:     "two hours rental",
			in:       PriceInput{Type: BookingRental, HourlyRate: 200, MinHours: 1, Start: at(10, 0), End: at(12, 0)},
			billable: 2, total: 400, lineCount: 1,
		},
		{
			name:     "minimum hours applied",
			in:       PriceInput{Type: BookingRental, HourlyRate: 200, MinHours: 3, Start: at(10, 0), End: at(11, 0)},
			billable: 3, total: 600, lineCount: 1,
		},
		{
			name:     "rounded up to half hour",
			in:       PriceInput{Type: BookingRental, HourlyRate: 100, MinHours: 0, Start: at(10, 0), End: at(11, 10)},
			billable: 1.5, total: 150, lineCount: 1,
		},
		{
			name:     "exact half hour not rounded further",
			in:       PriceInput{Type: BookingRental, HourlyRate: 100, MinHours: 0, Start: at(10, 0), End: at(11, 30)},
			billable: 1.5, total: 150, lineCount: 1,
		},
		{
			name: "equipment extras, duplicates and unknown ignored",
			in: PriceInput{
				Type: BookingRental, HourlyRate: 500, MinHours: 4, Start: at(18, 0), End: at(23, 0),
				Equipment: []string{"sound", "lighting", "sound", "kitchen"}, EquipmentPrices: prices,
			},
			billable: 5, total: 2500 + 300 + 250, lineCount: 3,
		},
		{
			name:     "activities are free",
			in:       PriceInput{Type: BookingActivity, HourlyRate: 200, MinHours: 1, Start: at(10, 0), End: at(12, 0)},
			billable: 0, total: 0, lineCount: 0,
		},
		{
			name:     "free room has no base line",
			in:       PriceInput{Type: BookingRental, HourlyRate: 0, MinHours: 1, Start: at(10, 0), End: at(12, 0)},
			billable: 2, total: 0, lineCount: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			q := Quote(tt.in)
			if q.BillableHours != tt.billable {
				t.Errorf("billable = %v, want %v", q.BillableHours, tt.billable)
			}
			if q.Total != tt.total {
				t.Errorf("total = %v, want %v", q.Total, tt.total)
			}
			if len(q.Lines) != tt.lineCount {
				t.Errorf("lines = %d, want %d (%+v)", len(q.Lines), tt.lineCount, q.Lines)
			}
		})
	}
}

func TestBookingBalance(t *testing.T) {
	b := Booking{Price: 1000.10, PaidAmount: 400.05}
	if got := b.Balance(); got != 600.05 {
		t.Fatalf("balance = %v", got)
	}
}
