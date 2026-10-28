package service

import (
	"context"
	"sort"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type ReportRow struct {
	Key      string  `json:"key"`
	Label    string  `json:"label"`
	Bookings int     `json:"bookings"`
	Hours    float64 `json:"hours"`
	Revenue  float64 `json:"revenue"`
	Paid     float64 `json:"paid"`
}

type Report struct {
	From     time.Time   `json:"from"`
	To       time.Time   `json:"to"`
	ByMonth  []ReportRow `json:"byMonth"`
	ByRoom   []ReportRow `json:"byRoom"`
	ByType   []ReportRow `json:"byType"`
	Totals   ReportRow   `json:"totals"`
	Payments []ReportRow `json:"paymentsByMethod"`
}

func (s *Service) Report(ctx context.Context, actor Actor, from, to time.Time) (Report, error) {
	if err := actor.Require(domain.ActViewReports); err != nil {
		return Report{}, err
	}
	bookings, err := s.store.ListBookings(ctx, store.BookingFilter{From: from, To: to, Statuses: []string{"confirmed"}})
	if err != nil {
		return Report{}, err
	}
	rep := Report{From: from, To: to}
	months, rooms, types := map[string]*ReportRow{}, map[string]*ReportRow{}, map[string]*ReportRow{}
	add := func(m map[string]*ReportRow, key, label string, b domain.Booking) {
		row, ok := m[key]
		if !ok {
			row = &ReportRow{Key: key, Label: label}
			m[key] = row
		}
		row.Bookings++
		row.Hours += b.EndsAt.Sub(b.StartsAt).Hours()
		row.Revenue += b.Price
		row.Paid += b.PaidAmount
	}
	for _, b := range bookings {
		if b.Type == domain.BookingClosed || b.Type == domain.BookingMaintenance {
			continue
		}
		mk := b.StartsAt.In(s.loc).Format("2006-01")
		add(months, mk, mk, b)
		add(rooms, b.RoomID.String(), b.RoomName, b)
		add(types, string(b.Type), string(b.Type), b)
		rep.Totals.Bookings++
		rep.Totals.Hours += b.EndsAt.Sub(b.StartsAt).Hours()
		rep.Totals.Revenue += b.Price
		rep.Totals.Paid += b.PaidAmount
	}
	flatten := func(m map[string]*ReportRow, byKey bool) []ReportRow {
		out := make([]ReportRow, 0, len(m))
		for _, r := range m {
			out = append(out, *r)
		}
		sort.Slice(out, func(i, j int) bool {
			if byKey {
				return out[i].Key < out[j].Key
			}
			return out[i].Revenue > out[j].Revenue || (out[i].Revenue == out[j].Revenue && out[i].Hours > out[j].Hours)
		})
		return out
	}
	rep.ByMonth, rep.ByRoom, rep.ByType = flatten(months, true), flatten(rooms, false), flatten(types, false)

	payments, err := s.store.ListPayments(ctx, store.PaymentFilter{From: from, To: to})
	if err != nil {
		return rep, err
	}
	methods := map[string]*ReportRow{}
	for _, p := range payments {
		row, ok := methods[string(p.Method)]
		if !ok {
			row = &ReportRow{Key: string(p.Method), Label: string(p.Method)}
			methods[string(p.Method)] = row
		}
		row.Bookings++
		row.Paid += p.Amount
	}
	rep.Payments = flatten(methods, true)
	return rep, nil
}
