package store

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

// ── Attachments ──────────────────────────────────────────────

const attachmentCols = `id, entity, entity_id, object_key, filename, content_type, size_bytes, created_at`

func scanAttachment(r pgx.Row) (domain.Attachment, error) {
	var a domain.Attachment
	err := r.Scan(&a.ID, &a.Entity, &a.EntityID, &a.ObjectKey, &a.Filename, &a.ContentType, &a.SizeBytes, &a.CreatedAt)
	return a, mapErr(err)
}

func (s *Store) CreateAttachment(ctx context.Context, a domain.Attachment, by uuid.UUID) (domain.Attachment, error) {
	return scanAttachment(s.q.QueryRow(ctx, `
		INSERT INTO attachments (entity, entity_id, object_key, filename, content_type, size_bytes, uploaded_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING `+attachmentCols,
		a.Entity, a.EntityID, a.ObjectKey, a.Filename, a.ContentType, a.SizeBytes, by))
}

func (s *Store) ListAttachments(ctx context.Context, entity string, entityID uuid.UUID) ([]domain.Attachment, error) {
	rows, err := s.q.Query(ctx, `SELECT `+attachmentCols+` FROM attachments WHERE entity = $1 AND entity_id = $2
		ORDER BY created_at`, entity, entityID)
	return collect(rows, err, scanAttachment)
}

func (s *Store) AttachmentByID(ctx context.Context, id uuid.UUID) (domain.Attachment, error) {
	return scanAttachment(s.q.QueryRow(ctx, `SELECT `+attachmentCols+` FROM attachments WHERE id = $1`, id))
}

func (s *Store) DeleteAttachment(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.Exec(ctx, `DELETE FROM attachments WHERE id = $1`, id)
	return mapErr(err)
}

// ── Notifications ────────────────────────────────────────────

const notificationCols = `id, user_id, channel, recipient, subject, body, status, error, read_at, sent_at, created_at`

func scanNotification(r pgx.Row) (domain.Notification, error) {
	var n domain.Notification
	err := r.Scan(&n.ID, &n.UserID, &n.Channel, &n.Recipient, &n.Subject, &n.Body, &n.Status, &n.Error,
		&n.ReadAt, &n.SentAt, &n.CreatedAt)
	return n, mapErr(err)
}

func (s *Store) CreateNotification(ctx context.Context, n domain.Notification) (domain.Notification, error) {
	return scanNotification(s.q.QueryRow(ctx, `
		INSERT INTO notifications (user_id, channel, recipient, subject, body, status, error, sent_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+notificationCols,
		n.UserID, n.Channel, n.Recipient, n.Subject, n.Body, n.Status, n.Error, n.SentAt))
}

func (s *Store) ListNotifications(ctx context.Context, userID *uuid.UUID, limit int) ([]domain.Notification, error) {
	w := &where{}
	if userID != nil {
		w.add("user_id = $%d", *userID)
	}
	rows, err := s.q.Query(ctx, `SELECT `+notificationCols+` FROM notifications`+w.sql()+
		` ORDER BY created_at DESC LIMIT `+w.next(limit), w.args...)
	return collect(rows, err, scanNotification)
}

func (s *Store) MarkNotificationsRead(ctx context.Context, userID uuid.UUID) error {
	_, err := s.q.Exec(ctx, `UPDATE notifications SET read_at = now() WHERE user_id = $1 AND read_at IS NULL`, userID)
	return mapErr(err)
}

// ── Audit log ────────────────────────────────────────────────

func (s *Store) Audit(ctx context.Context, actor *uuid.UUID, entity string, entityID *uuid.UUID, action string, data any) error {
	raw, err := json.Marshal(data)
	if err != nil || data == nil {
		raw = []byte("{}")
	}
	_, err = s.q.Exec(ctx, `INSERT INTO audit_log (actor_id, entity, entity_id, action, data) VALUES ($1, $2, $3, $4, $5)`,
		actor, entity, entityID, action, raw)
	return mapErr(err)
}

type AuditFilter struct {
	Entity   string
	EntityID *uuid.UUID
	Before   time.Time
	Limit    int
}

func (s *Store) ListAudit(ctx context.Context, f AuditFilter) ([]domain.AuditEntry, error) {
	w := &where{}
	if f.Entity != "" {
		w.add("a.entity = $%d", f.Entity)
	}
	if f.EntityID != nil {
		w.add("a.entity_id = $%d", *f.EntityID)
	}
	if !f.Before.IsZero() {
		w.add("a.created_at < $%d", f.Before)
	}
	if f.Limit <= 0 || f.Limit > 500 {
		f.Limit = 100
	}
	rows, err := s.q.Query(ctx, `
		SELECT a.id, a.actor_id, u.full_name, a.entity, a.entity_id, a.action, a.data, a.created_at
		FROM audit_log a LEFT JOIN users u ON u.id = a.actor_id`+w.sql()+
		` ORDER BY a.created_at DESC LIMIT `+w.next(f.Limit), w.args...)
	return collect(rows, err, func(r pgx.Row) (domain.AuditEntry, error) {
		var e domain.AuditEntry
		err := r.Scan(&e.ID, &e.ActorID, &e.ActorName, &e.Entity, &e.EntityID, &e.Action, &e.Data, &e.CreatedAt)
		return e, err
	})
}
