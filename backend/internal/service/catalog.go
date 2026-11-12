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

// publicEntities are files shown without login (signage, room photos, logos).
var publicEntities = map[string]bool{"announcement": true, "room": true, "organization": true}

var allowedEntities = map[string]bool{
	"booking": true, "request": true, "room": true, "announcement": true, "customer": true, "organization": true,
	"task":      true, // ticket photos (what is broken / missing)
	"task_done": true, // proof photos taken when the task is completed
	"contract":  true, // signature and scanned signed contracts
}

// fileAccess checks whether actor may read (write=false) or add/remove
// (write=true) the files attached to entity/entityID.
func (s *Service) fileAccess(ctx context.Context, actor *Actor, entity string, entityID uuid.UUID, write bool) error {
	if !allowedEntities[entity] {
		v := domain.NewValidation()
		v.Add("entity", "סוג ישות לא נתמך")
		return v
	}
	if !write && publicEntities[entity] {
		return nil
	}
	if actor == nil {
		return domain.ErrUnauthorized
	}
	a := *actor
	switch entity {
	case "organization":
		return a.Require(domain.ActManageSettings)
	case "room":
		return a.Require(domain.ActManageRooms)
	case "announcement":
		return a.Require(domain.ActManageSignage)
	case "booking":
		if write {
			return a.Require(domain.ActManageBookings)
		}
		return a.Require(domain.ActViewCalendar)
	case "customer":
		if write {
			return a.Require(domain.ActManageCustomers)
		}
		return a.Require(domain.ActViewCalendar)
	case "request":
		if a.Can(domain.ActReviewRequests) || (!write && a.Can(domain.ActViewCalendar)) {
			return nil
		}
		if req, err := s.store.RequestByID(ctx, entityID); err == nil && req.UserID == a.User.ID {
			return nil
		}
		return domain.ErrForbidden
	case "task", "task_done":
		if a.Can(domain.ActManageTasks) {
			return nil
		}
		t, err := s.store.TaskByID(ctx, entityID)
		if err != nil {
			return domain.ErrForbidden
		}
		assignee := t.AssigneeID != nil && *t.AssigneeID == a.User.ID
		reporter := t.ReportedBy != nil && *t.ReportedBy == a.User.ID
		// The reporter adds what is wrong; the worker adds the proof it is done.
		if (entity == "task" && (reporter || (!write && assignee))) || (entity == "task_done" && (assignee || (!write && reporter))) {
			return nil
		}
		return domain.ErrForbidden
	case "contract":
		return a.Require(domain.ActManageCoords)
	}
	return domain.ErrForbidden
}

func (s *Service) Upload(ctx context.Context, actor Actor, entity string, entityID uuid.UUID, filename, contentType string, r io.Reader, size int64) (domain.Attachment, error) {
	if entity == "organization" {
		entityID = OrganizationEntityID
	}
	if err := s.fileAccess(ctx, &actor, entity, entityID, true); err != nil {
		return domain.Attachment{}, err
	}
	v := domain.NewValidation()
	if !allowedTypes[contentType] {
		v.Add("file", "סוג קובץ לא נתמך (PNG, JPG, WEBP, PDF, DOCX, XLSX)")
	}
	if size <= 0 || size > MaxUploadBytes {
		v.Add("file", "גודל קובץ מקסימלי 15MB")
	}
	if err := v.OrNil(); err != nil {
		return domain.Attachment{}, err
	}
	return s.storeFile(ctx, actor.User.ID, entity, entityID, filename, contentType, r, size, &actor)
}

// storeFile puts the object in S3 and records the attachment.
func (s *Service) storeFile(ctx context.Context, by uuid.UUID, entity string, entityID uuid.UUID, filename, contentType string, r io.Reader, size int64, actor *Actor) (domain.Attachment, error) {
	key, err := s.files.Put(ctx, entity, filename, contentType, r, size)
	if err != nil {
		return domain.Attachment{}, fmt.Errorf("upload: %w", err)
	}
	a, err := s.store.CreateAttachment(ctx, domain.Attachment{
		Entity: entity, EntityID: entityID, ObjectKey: key, Filename: filename, ContentType: contentType, SizeBytes: size,
	}, by)
	if err != nil {
		s.files.Delete(ctx, key) //nolint:errcheck
		return a, err
	}
	if actor != nil {
		s.audit(ctx, *actor, entity, entityID, "attach", map[string]any{"file": filename})
	}
	return a, nil
}

// ListFiles lists the attachments of an entity the actor may see.
func (s *Service) ListFiles(ctx context.Context, actor Actor, entity string, entityID uuid.UUID) ([]domain.Attachment, error) {
	if entity == "organization" {
		entityID = OrganizationEntityID
	}
	if err := s.fileAccess(ctx, &actor, entity, entityID, false); err != nil {
		return nil, err
	}
	return s.store.ListAttachments(ctx, entity, entityID)
}

// OpenFile returns the object stream for an attachment the actor may see.
func (s *Service) OpenFile(ctx context.Context, actor *Actor, id uuid.UUID) (domain.Attachment, io.ReadCloser, storage.ObjectInfo, error) {
	a, err := s.store.AttachmentByID(ctx, id)
	if err != nil {
		return a, nil, storage.ObjectInfo{}, err
	}
	if err := s.fileAccess(ctx, actor, a.Entity, a.EntityID, false); err != nil {
		return a, nil, storage.ObjectInfo{}, err
	}
	rc, info, err := s.files.Get(ctx, a.ObjectKey)
	return a, rc, info, err
}

func (s *Service) DeleteFile(ctx context.Context, actor Actor, id uuid.UUID) error {
	a, err := s.store.AttachmentByID(ctx, id)
	if err != nil {
		return err
	}
	if err := s.fileAccess(ctx, &actor, a.Entity, a.EntityID, true); err != nil {
		return err
	}
	if err := s.store.DeleteAttachment(ctx, id); err != nil {
		return err
	}
	s.files.Delete(ctx, a.ObjectKey) //nolint:errcheck
	return nil
}
