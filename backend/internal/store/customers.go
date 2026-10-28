package store

import (
	"context"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const customerSelect = `
	SELECT c.id, c.name, c.kind, c.contact_name, c.phone, c.email, c.tax_id, c.notes, c.user_id, c.created_at,
	       COALESCE(agg.cnt, 0), COALESCE(agg.billed, 0), COALESCE(agg.paid, 0)
	FROM customers c
	LEFT JOIN LATERAL (
	    SELECT count(*) AS cnt, sum(b.price) AS billed,
	           (SELECT COALESCE(sum(p.amount), 0) FROM payments p JOIN bookings b2 ON b2.id = p.booking_id
	             WHERE b2.customer_id = c.id) AS paid
	    FROM bookings b WHERE b.customer_id = c.id AND b.status IN ('pending', 'confirmed')
	) agg ON true`

func scanCustomer(r pgx.Row) (domain.Customer, error) {
	var c domain.Customer
	err := r.Scan(&c.ID, &c.Name, &c.Kind, &c.ContactName, &c.Phone, &c.Email, &c.TaxID, &c.Notes, &c.UserID,
		&c.CreatedAt, &c.BookingsCount, &c.TotalBilled, &c.TotalPaid)
	return c, mapErr(err)
}

func (s *Store) ListCustomers(ctx context.Context, search string) ([]domain.Customer, error) {
	w := &where{}
	if q := strings.TrimSpace(search); q != "" {
		p := w.next("%" + q + "%")
		w.raw("(c.name ILIKE " + p + " OR c.contact_name ILIKE " + p + " OR c.phone ILIKE " + p + " OR c.email ILIKE " + p + ")")
	}
	rows, err := s.q.Query(ctx, customerSelect+w.sql()+` ORDER BY c.name`, w.args...)
	return collect(rows, err, scanCustomer)
}

func (s *Store) CustomerByID(ctx context.Context, id uuid.UUID) (domain.Customer, error) {
	return scanCustomer(s.q.QueryRow(ctx, customerSelect+` WHERE c.id = $1`, id))
}

func (s *Store) CustomerByUserID(ctx context.Context, userID uuid.UUID) (domain.Customer, error) {
	return scanCustomer(s.q.QueryRow(ctx, customerSelect+` WHERE c.user_id = $1`, userID))
}

type CustomerInput struct {
	Name        string
	Kind        string
	ContactName *string
	Phone       *string
	Email       *string
	TaxID       *string
	Notes       string
	UserID      *uuid.UUID
}

func (s *Store) CreateCustomer(ctx context.Context, in CustomerInput) (domain.Customer, error) {
	var id uuid.UUID
	err := s.q.QueryRow(ctx, `
		INSERT INTO customers (name, kind, contact_name, phone, email, tax_id, notes, user_id)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING id`,
		in.Name, in.Kind, in.ContactName, in.Phone, in.Email, in.TaxID, in.Notes, in.UserID).Scan(&id)
	if err != nil {
		return domain.Customer{}, mapErr(err)
	}
	return s.CustomerByID(ctx, id)
}

func (s *Store) UpdateCustomer(ctx context.Context, id uuid.UUID, in CustomerInput) (domain.Customer, error) {
	tag, err := s.q.Exec(ctx, `
		UPDATE customers SET name = $2, kind = $3, contact_name = $4, phone = $5, email = $6, tax_id = $7,
		       notes = $8, updated_at = now() WHERE id = $1`,
		id, in.Name, in.Kind, in.ContactName, in.Phone, in.Email, in.TaxID, in.Notes)
	if err != nil {
		return domain.Customer{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Customer{}, domain.ErrNotFound
	}
	return s.CustomerByID(ctx, id)
}

func (s *Store) DeleteCustomer(ctx context.Context, id uuid.UUID) error {
	tag, err := s.q.Exec(ctx, `DELETE FROM customers WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}
