package domain

import (
	"math"
	"time"
)

type PriceInput struct {
	Type            BookingType
	HourlyRate      float64
	MinHours        float64
	Start, End      time.Time
	Equipment       []string
	EquipmentPrices map[string]float64 // code → flat price
	EquipmentLabels map[string]string  // code → display label (optional)
}

type PriceLine struct {
	Label  string  `json:"label"`
	Amount float64 `json:"amount"`
}

type PriceQuote struct {
	Hours         float64     `json:"hours"`
	BillableHours float64     `json:"billableHours"`
	Base          float64     `json:"base"`
	Extras        float64     `json:"extras"`
	Total         float64     `json:"total"`
	Lines         []PriceLine `json:"lines"`
}

// Quote computes the price of a booking. Only paid rentals are charged:
// billable time is rounded up to the next half hour and never below the
// room's minimum, plus a flat fee per optional equipment item.
func Quote(in PriceInput) PriceQuote {
	q := PriceQuote{Hours: round2(in.End.Sub(in.Start).Hours())}
	if in.Type != BookingRental || !in.End.After(in.Start) {
		return q
	}

	halfHours := math.Ceil(in.End.Sub(in.Start).Minutes()/30 - 1e-9)
	q.BillableHours = math.Max(halfHours/2, in.MinHours)
	q.Base = round2(q.BillableHours * in.HourlyRate)
	if q.Base > 0 {
		q.Lines = append(q.Lines, PriceLine{Label: "שכירות חדר", Amount: q.Base})
	}

	seen := map[string]bool{}
	for _, code := range in.Equipment {
		if seen[code] {
			continue
		}
		seen[code] = true
		if p := in.EquipmentPrices[code]; p > 0 {
			q.Extras += p
			label := code
			if l := in.EquipmentLabels[code]; l != "" {
				label = l
			}
			q.Lines = append(q.Lines, PriceLine{Label: label, Amount: round2(p)})
		}
	}
	q.Extras = round2(q.Extras)
	q.Total = round2(q.Base + q.Extras)
	return q
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
