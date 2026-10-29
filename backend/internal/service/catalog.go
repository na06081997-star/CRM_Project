package service

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// ── Buildings & rooms ────────────────────────────────────────

var hhmm = regexp.MustCompile(`^([01]\d|2[0-3]):[0-5]\d$`)

func (s *Service) SaveBuilding(ctx context.Context, actor Actor, b domain.Building) (domain.Building, error) {
	if err := actor.Require(domain.ActManageSettings); err != nil {
		return b, err
	}
	v := domain.NewValidation()
	if strings.TrimSpace(b.Name) == "" {
		v.Add("name", "יש להזין שם מבנה")
	}
	if !hhmm.MatchString(b.OpensAt) || !hhmm.MatchString(b.ClosesAt) {
		v.Add("opensAt", "שעות פעילות לא תקינות")
	}
	if err := v.OrNil(); err != nil {
		return b, err
	}
	if b.ID == uuid.Nil {
		return s.store.CreateBuilding(ctx, b)
	}
	return s.store.UpdateBuilding(ctx, b)
}

var hexColor = regexp.MustCompile(`^#[0-9a-fA-F]{6}$`)

func (s *Service) SaveRoom(ctx context.Context, actor Actor, r domain.Room) (domain.Room, error) {
	if err := actor.Require(domain.ActManageRooms); err != nil {
		return r, err
	}
	r.Name = strings.TrimSpace(r.Name)
	v := domain.NewValidation()
	if r.Name == "" {
		v.Add("name", "יש להזין שם חדר")
	}
	if r.BuildingID == uuid.Nil {
		v.Add("buildingId", "יש לבחור מבנה")
	}
	if r.Kind == "" {
		r.Kind = "other"
	}
	if r.Color == "" {
		r.Color = "#0f766e"
	}
	if !hexColor.MatchString(r.Color) {
		v.Add("color", "צבע לא תקין")
	}
	if r.Capacity < 0 || r.HourlyRate < 0 || r.MinHours < 0 {
		v.Add("capacity", "ערכים שליליים אינם מותרים")
	}
	if r.BufferMinutes < 0 || r.BufferMinutes > 240 {
		v.Add("bufferMinutes", "זמן חציצה בין 0 ל-240 דקות")
	}
	if err := v.OrNil(); err != nil {
		return r, err
	}
	var saved domain.Room
	var err error
	if r.ID == uuid.Nil {
		saved, err = s.store.CreateRoom(ctx, r)
	} else {
		saved, err = s.store.UpdateRoom(ctx, r)
	}
	if err != nil {
		return saved, err
	}
	s.audit(ctx, actor, "room", saved.ID, "save", map[string]any{"name": saved.Name, "rate": saved.HourlyRate})
	s.emit(ctx, EvSignageChanged, nil, realtime.TopicSignage, realtime.TopicStaff)
	return saved, nil
}

func (s *Service) SaveEquipment(ctx context.Context, actor Actor, e domain.EquipmentItem) (domain.EquipmentItem, error) {
	if err := actor.Require(domain.ActManageSettings); err != nil {
		return e, err
	}
	e.Code = strings.TrimSpace(strings.ToLower(e.Code))
	if e.Code == "" || strings.TrimSpace(e.Label) == "" || e.FlatPrice < 0 {
		v := domain.NewValidation()
		v.Add("code", "יש להזין קוד, שם ומחיר תקין")
		return e, v
	}
	return s.store.UpsertEquipment(ctx, e)
}

// ── Activities ───────────────────────────────────────────────

func (s *Service) SaveActivity(ctx context.Context, actor Actor, a domain.Activity) (domain.Activity, error) {
	if err := actor.Require(domain.ActManageActivity); err != nil {
		return a, err
	}
	if strings.TrimSpace(a.Name) == "" {
		v := domain.NewValidation()
		v.Add("name", "יש להזין שם פעילות")
		return a, v
	}
	if a.Color == "" {
		a.Color = "#0f766e"
	}
	return s.store.SaveActivity(ctx, a)
}

// ── Customers ────────────────────────────────────────────────

func (s *Service) SaveCustomer(ctx context.Context, actor Actor, id uuid.UUID, in store.CustomerInput) (domain.Customer, error) {
	if err := actor.Require(domain.ActManageCustomers); err != nil {
		return domain.Customer{}, err
	}
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		v := domain.NewValidation()
		v.Add("name", "יש להזין שם לקוח")
		return domain.Customer{}, v
	}
	switch in.Kind {
	case "private", "organization", "internal":
	default:
		in.Kind = "private"
	}
	var c domain.Customer
	var err error
	if id == uuid.Nil {
		c, err = s.store.CreateCustomer(ctx, in)
	} else {
		c, err = s.store.UpdateCustomer(ctx, id, in)
	}
	if err == nil {
		s.audit(ctx, actor, "customer", c.ID, "save", map[string]any{"name": c.Name})
	}
	return c, err
}

// ── Signage config ───────────────────────────────────────────

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{1,40}$`)

func (s *Service) SaveScreen(ctx context.Context, actor Actor, sc domain.SignageScreen) (domain.SignageScreen, error) {
	if err := actor.Require(domain.ActManageSignage); err != nil {
		return sc, err
	}
	sc.Slug = strings.ToLower(strings.TrimSpace(sc.Slug))
	v := domain.NewValidation()
	if !slugRe.MatchString(sc.Slug) {
		v.Add("slug", "מזהה באותיות לטיניות קטנות, ספרות ומקפים בלבד")
	}
	if strings.TrimSpace(sc.Name) == "" {
		v.Add("name", "יש להזין שם מסך")
	}
	if sc.RotateSeconds == 0 {
		sc.RotateSeconds = 12
	}
	if sc.Theme == "" {
		sc.Theme = "dark"
	}
	if err := v.OrNil(); err != nil {
		return sc, err
	}
	saved, err := s.store.SaveScreen(ctx, sc)
	if err == nil {
		s.emit(ctx, EvSignageChanged, nil, realtime.TopicSignage)
	}
	return saved, err
}

func (s *Service) SaveAnnouncement(ctx context.Context, actor Actor, a domain.Announcement) (domain.Announcement, error) {
	if err := actor.Require(domain.ActManageSignage); err != nil {
		return a, err
	}
	if strings.TrimSpace(a.Title) == "" {
		v := domain.NewValidation()
		v.Add("title", "יש להזין כותרת")
		return a, v
	}
	if a.StartsAt.IsZero() {
		a.StartsAt = s.now()
	}
	saved, err := s.store.SaveAnnouncement(ctx, a, actor.ID())
	if err == nil {
		s.emit(ctx, EvSignageChanged, nil, realtime.TopicSignage)
	}
	return saved, err
}

func (s *Service) DeleteAnnouncement(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := actor.Require(domain.ActManageSignage); err != nil {
		return err
	}
	err := s.store.DeleteAnnouncement(ctx, id)
	if err == nil {
		s.emit(ctx, EvSignageChanged, nil, realtime.TopicSignage)
	}
	return err
}

// ── Files ────────────────────────────────────────────────────

const MaxUploadBytes = 15 << 20

var allowedTypes = map[string]bool{
	"image/png": true, "image/jpeg": true, "image/webp": true, "image/gif": true, "image/svg+xml": false,
	"application/pdf": true,
	"application/vnd.openxmlformats-officedocument.wordprocessingml.document": true,
	"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet":       true,
}

var allowedEntities = map[string]bool{"booking": true, "request": true, "room": true, "announcement": true, "customer": true, "task": true, "organization": true}

func (s *Service) Upload(ctx context.Context, actor Actor, entity string, entityID uuid.UUID, filename, contentType string, r io.Reader, size int64) (domain.Attachment, error) {
	if entity == "organization" {
		if err := actor.Require(domain.ActManageSettings); err != nil {
			return domain.Attachment{}, err
		}
		entityID = OrganizationEntityID
	}
	// Portal users may attach files to their own requests only.
	if !actor.Can(domain.ActUploadFiles) {
		if entity != "request" {
			return domain.Attachment{}, domain.ErrForbidden
		}
		req, err := s.store.RequestByID(ctx, entityID)
		if err != nil || req.UserID != actor.User.ID {
			return domain.Attachment{}, domain.ErrForbidden
		}
	}
	v := domain.NewValidation()
	if !allowedEntities[entity] {
		v.Add("entity", "סוג ישות לא נתמך")
	}
	if !allowedTypes[contentType] {
		v.Add("file", "סוג קובץ לא נתמך (PNG, JPG, WEBP, PDF, DOCX, XLSX)")
	}
	if size <= 0 || size > MaxUploadBytes {
		v.Add("file", "גודל קובץ מקסימלי 15MB")
	}
	if err := v.OrNil(); err != nil {
		return domain.Attachment{}, err
	}
	key, err := s.files.Put(ctx, entity, filename, contentType, r, size)
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("upload: %w", err)
	}
	a, err := s.store.CreateAttachment(ctx, domain.Attachment{
		Entity: entity, EntityID: entityID, ObjectKey: key, Filename: filename, ContentType: contentType, SizeBytes: size,
	}, actor.User.ID)
	if err != nil {
		s.files.Delete(ctx, key) //nolint:errcheck
		return a, err
	}
	s.audit(ctx, actor, entity, entityID, "attach", map[string]any{"file": filename})
	return a, nil
}

// OpenFile returns the object stream for an attachment the actor may see.
func (s *Service) OpenFile(ctx context.Context, actor *Actor, id uuid.UUID) (domain.Attachment, io.ReadCloser, storage.ObjectInfo, error) {
	a, err := s.store.AttachmentByID(ctx, id)
	if err != nil {
		return a, nil, storage.ObjectInfo{}, err
	}
	public := a.Entity == "announcement" || a.Entity == "room" || a.Entity == "organization"
	if !public {
		if actor == nil {
			return a, nil, storage.ObjectInfo{}, domain.ErrUnauthorized
		}
		if !actor.Can(domain.ActViewCalendar) {
			req, err := s.store.RequestByID(ctx, a.EntityID)
			if a.Entity != "request" || err != nil || req.UserID != actor.User.ID {
				return a, nil, storage.ObjectInfo{}, domain.ErrForbidden
			}
		}
	}
	rc, info, err := s.files.Get(ctx, a.ObjectKey)
	return a, rc, info, err
}

func (s *Service) DeleteFile(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := actor.Require(domain.ActUploadFiles); err != nil {
		return err
	}
	a, err := s.store.AttachmentByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.store.DeleteAttachment(ctx, id); err != nil {
		return err
	}
	s.files.Delete(ctx, a.ObjectKey) //nolint:errcheck
	return nil
}
