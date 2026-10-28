// Package service orchestrates use cases: it validates input, applies the
// domain rules, persists through the store and emits realtime events.
package service

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/notify"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

type Publisher interface {
	Publish(ctx context.Context, ev realtime.Event)
}

type Notifier interface {
	Send(ctx context.Context, m notify.Message) error
}

type Service struct {
	cfg    *config.Config
	store  *store.Store
	events Publisher
	notify Notifier
	files  storage.Store
	loc    *time.Location
	now    func() time.Time
}

func New(cfg *config.Config, st *store.Store, events Publisher, n Notifier, files storage.Store) *Service {
	return &Service{cfg: cfg, store: st, events: events, notify: n, files: files, loc: cfg.Location, now: time.Now}
}

func (s *Service) Store() *store.Store { return s.store }

// Actor is the authenticated caller of a use case.
type Actor struct {
	User domain.User
}

func (a Actor) ID() *uuid.UUID { id := a.User.ID; return &id }

func (a Actor) Can(act domain.Action) bool { return a.User.IsActive && domain.Can(a.User.Role, act) }

func (a Actor) Require(act domain.Action) error {
	if !a.Can(act) {
		return domain.ErrForbidden
	}
	return nil
}

// Event type names shared with the frontend.
const (
	EvBookingChanged = "booking.changed"
	EvRequestChanged = "request.changed"
	EvPaymentChanged = "payment.changed"
	EvTaskChanged    = "task.changed"
	EvSignageChanged = "signage.changed"
	EvNotification   = "notification"
)

func (s *Service) emit(ctx context.Context, typ string, data any, topics ...string) {
	if s.events == nil {
		return
	}
	s.events.Publish(ctx, realtime.Event{Type: typ, Data: data, Topics: topics})
}

func (s *Service) audit(ctx context.Context, actor Actor, entity string, id uuid.UUID, action string, data any) {
	if err := s.store.Audit(ctx, actor.ID(), entity, &id, action, data); err != nil {
		slog.Error("audit write failed", "entity", entity, "action", action, "err", err)
	}
}

// startOfDay returns local midnight of t.
func (s *Service) startOfDay(t time.Time) time.Time {
	l := t.In(s.loc)
	return time.Date(l.Year(), l.Month(), l.Day(), 0, 0, 0, 0, s.loc)
}

// notifyUser sends a message to a user asynchronously and pushes it live.
func (s *Service) notifyUser(u domain.User, subject, body string) {
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		r := notify.Recipient{UserID: &u.ID, Name: u.FullName, Email: u.Email}
		if u.Phone != nil {
			r.Phone = *u.Phone
		}
		if u.TelegramChatID != nil {
			r.TelegramChatID = *u.TelegramChatID
		}
		if err := s.notify.Send(ctx, notify.Message{To: r, Subject: subject, Body: body}); err != nil {
			slog.Warn("notification failed", "user", u.ID, "err", err)
		}
		s.emit(ctx, EvNotification, map[string]string{"subject": subject, "body": body}, realtime.UserTopic(u.ID.String()))
	}()
}
