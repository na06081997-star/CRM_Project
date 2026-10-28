package api

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// ── Payments ─────────────────────────────────────────────────

func (s *Server) listPayments(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Payment, error) {
		return s.svc.ListPayments(r.Context(), a, store.PaymentFilter{
			BookingID: qUUID(r, "bookingId"), CustomerID: qUUID(r, "customerId"), Method: r.URL.Query().Get("method"),
			From: qTime(r, "from"), To: qTime(r, "to"), Limit: qInt(r, "limit", 500),
		})
	})(w, r)
}

func (s *Server) recordPayment(w http.ResponseWriter, r *http.Request) {
	var in service.PaymentInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	p, err := s.svc.RecordPayment(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, p)
}

func (s *Server) deletePayment(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		return s.svc.DeletePayment(r.Context(), a, id)
	})(w, r)
}

func (s *Server) paymentSummary(w http.ResponseWriter, r *http.Request) {
	sum, err := s.svc.PaymentSummary(r.Context(), mustActor(r))
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, sum)
}

func (s *Server) balances(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Booking, error) {
		return s.svc.Balances(r.Context(), a, qUUID(r, "customerId"))
	})(w, r)
}

// ── Tasks ────────────────────────────────────────────────────

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Task, error) {
		return s.svc.ListTasks(r.Context(), a, store.TaskFilter{
			AssigneeID: qUUID(r, "assigneeId"), Statuses: qList(r, "statuses"), From: qTime(r, "from"), To: qTime(r, "to"),
		})
	})(w, r)
}

func (s *Server) saveTask(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(t *domain.Task, id uuid.UUID) { t.ID = id },
		func(r *http.Request, a service.Actor, t domain.Task) (domain.Task, error) {
			return s.svc.SaveTask(r.Context(), a, t)
		})(w, r)
}

func (s *Server) taskStatus(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in struct {
		Status domain.TaskStatus `json:"status"`
		Note   string            `json:"note"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	t, err := s.svc.UpdateTaskStatus(r.Context(), mustActor(r), id, in.Status, in.Note)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, t)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		return s.svc.DeleteTask(r.Context(), a, id)
	})(w, r)
}

func (s *Server) listSchedules(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.TaskSchedule, error) {
		return s.svc.ListSchedules(r.Context(), a)
	})(w, r)
}

func (s *Server) saveSchedule(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(t *domain.TaskSchedule, id uuid.UUID) { t.ID = id },
		func(r *http.Request, a service.Actor, t domain.TaskSchedule) (domain.TaskSchedule, error) {
			return s.svc.SaveSchedule(r.Context(), a, t)
		})(w, r)
}

func (s *Server) deleteSchedule(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		return s.svc.DeleteSchedule(r.Context(), a, id)
	})(w, r)
}

// ── Files ────────────────────────────────────────────────────

func (s *Server) uploadFile(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxUploadBytes+1<<20)
	if err := r.ParseMultipartForm(4 << 20); err != nil {
		fail(w, r, errBadRequest)
		return
	}
	entityID, err := uuid.Parse(r.FormValue("entityId"))
	if err != nil {
		fail(w, r, errBadRequest)
		return
	}
	file, hdr, err := r.FormFile("file")
	if err != nil {
		fail(w, r, errBadRequest)
		return
	}
	defer file.Close()

	// Trust the sniffed type over the client-declared one.
	head := make([]byte, 512)
	n, _ := io.ReadFull(file, head)
	ctype := http.DetectContentType(head[:n])
	if ext := strings.ToLower(filepath.Ext(hdr.Filename)); ctype == "application/zip" || ctype == "application/octet-stream" {
		if byExt := mime.TypeByExtension(ext); byExt != "" {
			ctype = byExt
		}
	}
	if i := strings.Index(ctype, ";"); i > 0 {
		ctype = ctype[:i]
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		fail(w, r, err)
		return
	}
	a, err := s.svc.Upload(r.Context(), mustActor(r), r.FormValue("entity"), entityID, filepath.Base(hdr.Filename), ctype, file, hdr.Size)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, a)
}

func (s *Server) listFiles(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Attachment, error) {
		id := qUUID(r, "entityId")
		if id == nil {
			return nil, errBadRequest
		}
		entity := r.URL.Query().Get("entity")
		if !a.Can(domain.ActViewCalendar) {
			req, err := s.svc.Store().RequestByID(r.Context(), *id)
			if entity != "request" || err != nil || req.UserID != a.User.ID {
				return nil, domain.ErrForbidden
			}
		}
		return s.svc.Store().ListAttachments(r.Context(), entity, *id)
	})(w, r)
}

func (s *Server) downloadFile(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var actor *service.Actor
	if a, ok := actorFrom(r.Context()); ok {
		actor = &a
	}
	att, rc, info, err := s.svc.OpenFile(r.Context(), actor, id)
	if err != nil {
		fail(w, r, err)
		return
	}
	defer rc.Close()
	w.Header().Set("Content-Type", att.ContentType)
	w.Header().Set("Content-Length", fmt.Sprint(info.Size))
	w.Header().Set("Cache-Control", "private, max-age=3600")
	disposition := "inline"
	if !strings.HasPrefix(att.ContentType, "image/") && att.ContentType != "application/pdf" {
		disposition = "attachment"
	}
	w.Header().Set("Content-Disposition", fmt.Sprintf("%s; filename*=UTF-8''%s", disposition, url.PathEscape(att.Filename)))
	io.Copy(w, rc) //nolint:errcheck
}

func (s *Server) deleteFile(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		return s.svc.DeleteFile(r.Context(), a, id)
	})(w, r)
}

// ── Notifications & audit ────────────────────────────────────

func (s *Server) listNotifications(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Notification, error) {
		uid := &a.User.ID
		if r.URL.Query().Get("all") == "true" && a.Can(domain.ActViewAudit) {
			uid = nil
		}
		return s.svc.Store().ListNotifications(r.Context(), uid, qInt(r, "limit", 30))
	})(w, r)
}

func (s *Server) readNotifications(w http.ResponseWriter, r *http.Request) {
	if err := s.svc.Store().MarkNotificationsRead(r.Context(), mustActor(r).User.ID); err != nil {
		fail(w, r, err)
		return
	}
	noContent(w)
}

func (s *Server) listAudit(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.AuditEntry, error) {
		if err := requireAny(a, domain.ActViewAudit, domain.ActViewPayments); err != nil {
			return nil, err
		}
		return s.svc.Store().ListAudit(r.Context(), store.AuditFilter{
			Entity: r.URL.Query().Get("entity"), EntityID: qUUID(r, "entityId"), Before: qTime(r, "before"), Limit: qInt(r, "limit", 100),
		})
	})(w, r)
}

// ── Public signage & dev login ───────────────────────────────

func (s *Server) publicSignage(w http.ResponseWriter, r *http.Request) {
	view, err := s.svc.Signage(r.Context(), r.URL.Query().Get("screen"))
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	ok(w, view)
}

func (s *Server) devUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.svc.Store().ListUsers(r.Context(), store.UserFilter{Active: ptrBool(true)})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, users)
}

func (s *Server) devLogin(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Email string `json:"email"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	u, err := s.svc.Store().UserByEmail(r.Context(), in.Email)
	if errors.Is(err, domain.ErrNotFound) || (err == nil && !u.IsActive) {
		fail(w, r, domain.ErrUnauthorized)
		return
	}
	if err != nil {
		fail(w, r, err)
		return
	}
	tok, exp, err := s.local.Issue(u.ID, u.Email)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"token": tok, "expiresAt": exp, "user": u, "permissions": domain.Permissions(u.Role)})
}

func ptrBool(b bool) *bool { return &b }
