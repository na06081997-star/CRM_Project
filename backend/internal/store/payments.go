package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const paymentSelect = `
	SELECT p.id, p.booking_id, p.amount, p.method, p.reference, p.paid_at, p.notes, p.recorded_by, p.created_at,
	       b.title, cu.name
	FROM payments p
	JOIN bookings b ON b.id = p.booking_id
	LEFT JOIN customers cu ON cu.id = b.customer_id`

func scanPayment(r pgx.Row) (domain.Payment, error) {
	var p domain.Payment
	err := r.Scan(&p.ID, &p.BookingID, &p.Amount, &p.Method, &p.Reference, &p.PaidAt, &p.Notes, &p.RecordedBy,
		&p.CreatedAt, &p.BookingTitle, &p.CustomerName)
	return p, mapErr(err)
}

type PaymentFilter struct {
	BookingID  *uuid.UUID
	CustomerID *uuid.UUID
	Method     string
	From, To   time.Time
	Limit      int
}

func (s *Store) ListPayments(ctx context.Context, f PaymentFilter) ([]domain.Payment, error) {
	w := &where{}
	if f.BookingID != nil {
		w.add("p.booking_id = $%d", *f.BookingID)
	}
	if f.CustomerID != nil {
		w.add("b.customer_id = $%d", *f.CustomerID)
	}
	if f.Method != "" {
		w.add("p.method = $%d", f.Method)
	}
	if !f.From.IsZero() {
		w.add("p.paid_at >= $%d", f.From)
	}
	if !f.To.IsZero() {
		w.add("p.paid_at < $%d", f.To)
	}
	sql := paymentSelect + w.sql() + ` ORDER BY p.paid_at DESC`
	if f.Limit > 0 {
		sql += " LIMIT " + w.next(f.Limit)
	}
	rows, err := s.q.Query(ctx, sql, w.args...)
	return collect(rows, err, scanPayment)
}

func (s *Store) CreatePayment(ctx context.Context, p domain.Payment) (domain.Payment, error) {
	var id uuid.UUID
	err := s.q.QueryRow(ctx, `
		INSERT INTO payments (booking_id, amount, method, reference, paid_at, notes, recorded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
		p.BookingID, p.Amount, p.Method, p.Reference, p.PaidAt, p.Notes, p.RecordedBy).Scan(&id)
	if err != nil {
		return domain.Payment{}, mapErr(err)
	}
	return scanPayment(s.q.QueryRow(ctx, paymentSelect+` WHERE p.id = $1`, id))
}

func (s *Store) DeletePayment(ctx context.Context, id uuid.UUID) (domain.Payment, error) {
	p, err := scanPayment(s.q.QueryRow(ctx, paymentSelect+` WHERE p.id = $1`, id))
	if err != nil {
		return p, err
	}
	_, err = s.q.Exec(ctx, `DELETE FROM payments WHERE id = $1`, id)
	return p, mapErr(err)
}

type PaymentSummary struct {
	TotalBilled   float64            `json:"totalBilled"`
	TotalPaid     float64            `json:"totalPaid"`
	Outstanding   float64            `json:"outstanding"`
	UnpaidCount   int                `json:"unpaidCount"`
	ByMethod      map[string]float64 `json:"byMethod"`
	PaidThisMonth float64            `json:"paidThisMonth"`
}

func (s *Store) PaymentSummary(ctx context.Context, monthStart time.Time) (PaymentSummary, error) {
	sum := PaymentSummary{ByMethod: map[string]float64{}}
	err := s.q.QueryRow(ctx, `
		WITH bal AS (
		    SELECT b.price, COALESCE((SELECT sum(amount) FROM payments p WHERE p.booking_id = b.id), 0) AS paid
		    FROM bookings b WHERE b.status IN ('pending', 'confirmed') AND b.price > 0
		)
		SELECT COALESCE(sum(price), 0), COALESCE(sum(paid), 0), count(*) FILTER (WHERE paid < price)
		FROM bal`).Scan(&sum.TotalBilled, &sum.TotalPaid, &sum.UnpaidCount)
	if err != nil {
		return sum, mapErr(err)
	}
	sum.Outstanding = sum.TotalBilled - sum.TotalPaid

	rows, err := s.q.Query(ctx, `SELECT method::text, sum(amount) FROM payments GROUP BY method`)
	if err != nil {
		return sum, mapErr(err)
	}
	defer rows.Close()
	for rows.Next() {
		var m string
		var v float64
		if err := rows.Scan(&m, &v); err != nil {
			return sum, err
		}
		sum.ByMethod[m] = v
	}
	if err := rows.Err(); err != nil {
		return sum, err
	}
	err = s.q.QueryRow(ctx, `SELECT COALESCE(sum(amount), 0) FROM payments WHERE paid_at >= $1`, monthStart).
		Scan(&sum.PaidThisMonth)
	return sum, mapErr(err)
}
