package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type PaymentInput struct {
	BookingID uuid.UUID            `json:"bookingId"`
	Amount    float64              `json:"amount"`
	Method    domain.PaymentMethod `json:"method"`
	Reference string               `json:"reference"`
	PaidAt    *time.Time           `json:"paidAt"`
	Notes     string               `json:"notes"`
}

func (s *Service) ListPayments(ctx context.Context, actor Actor, f store.PaymentFilter) ([]domain.Payment, error) {
	if err := actor.Require(domain.ActViewPayments); err != nil {
		return nil, err
	}
	return s.store.ListPayments(ctx, f)
}

func (s *Service) PaymentSummary(ctx context.Context, actor Actor) (store.PaymentSummary, error) {
	if err := actor.Require(domain.ActViewPayments); err != nil {
		return store.PaymentSummary{}, err
	}
	now := s.now().In(s.loc)
	return s.store.PaymentSummary(ctx, time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, s.loc))
}

// Balances lists bookings that still have money due.
func (s *Service) Balances(ctx context.Context, actor Actor, customerID *uuid.UUID) ([]domain.Booking, error) {
	if err := actor.Require(domain.ActViewPayments); err != nil {
		return nil, err
	}
	return s.store.ListBookings(ctx, store.BookingFilter{OnlyWithBalance: true, CustomerID: customerID})
}

func (s *Service) RecordPayment(ctx context.Context, actor Actor, in PaymentInput) (domain.Payment, error) {
	if err := actor.Require(domain.ActManagePayments); err != nil {
		return domain.Payment{}, err
	}
	v := domain.NewValidation()
	if in.Amount <= 0 {
		v.Add("amount", "הסכום חייב להיות חיובי")
	}
	if !in.Method.Valid() {
		v.Add("method", "יש לבחור אמצעי תשלום")
	}
	if in.Method == domain.PayCheck && strings.TrimSpace(in.Reference) == "" {
		v.Add("reference", "יש להזין מספר צ׳ק")
	}
	if err := v.OrNil(); err != nil {
		return domain.Payment{}, err
	}

	var p domain.Payment
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		b, err := tx.BookingByID(ctx, in.BookingID)
		if err != nil {
			return err
		}
		if b.Status == domain.StatusCancelled || b.Status == domain.StatusRejected {
			v.Add("bookingId", "לא ניתן לרשום תשלום על הזמנה מבוטלת")
			return v
		}
		if in.Amount > b.Balance()+0.001 {
			v.Add("amount", fmt.Sprintf("הסכום גבוה מהיתרה לתשלום (₪%.2f)", b.Balance()))
			return v
		}
		paidAt := s.now()
		if in.PaidAt != nil {
			paidAt = *in.PaidAt
		}
		p, err = tx.CreatePayment(ctx, domain.Payment{
			BookingID: b.ID, Amount: in.Amount, Method: in.Method, Reference: strings.TrimSpace(in.Reference),
			PaidAt: paidAt, Notes: in.Notes, RecordedBy: actor.ID(),
		})
		return err
	})
	if err != nil {
		return domain.Payment{}, err
	}
	s.audit(ctx, actor, "payment", p.ID, "create", map[string]any{"bookingId": p.BookingID, "amount": p.Amount, "method": p.Method, "reference": p.Reference})
	s.emit(ctx, EvPaymentChanged, map[string]any{"action": "created", "bookingId": p.BookingID}, realtime.TopicStaff)
	return p, nil
}

func (s *Service) DeletePayment(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := actor.Require(domain.ActManagePayments); err != nil {
		return err
	}
	p, err := s.store.DeletePayment(ctx, id)
	if err != nil {
		return err
	}
	s.audit(ctx, actor, "payment", id, "delete", map[string]any{"bookingId": p.BookingID, "amount": p.Amount, "method": p.Method})
	s.emit(ctx, EvPaymentChanged, map[string]any{"action": "deleted", "bookingId": p.BookingID}, realtime.TopicStaff)
	return nil
}
