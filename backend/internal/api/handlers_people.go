package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
)

// ── Coordinators and their contracts ─────────────────────────

func (s *Server) createCoordinator(w http.ResponseWriter, r *http.Request) {
	var in service.CoordinatorInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CreateCoordinator(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, res)
}

func (s *Server) listContracts(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Contract, error) {
		return s.svc.ListContracts(r.Context(), a, qUUID(r, "userId"))
	})(w, r)
}

func (s *Server) createContract(w http.ResponseWriter, r *http.Request) {
	var in service.ContractInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CreateContract(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, res)
}

func (s *Server) resendContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.ResendContract(r.Context(), mustActor(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) markContractSigned(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	c, err := s.svc.MarkContractSigned(r.Context(), mustActor(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, c)
}

func (s *Server) cancelContract(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		return s.svc.CancelContract(r.Context(), a, id)
	})(w, r)
}

// Public signing page (the link sent by SMS / e-mail).

func (s *Server) publicContract(w http.ResponseWriter, r *http.Request) {
	c, err := s.svc.PublicContract(r.Context(), chi.URLParam(r, "token"))
	if err != nil {
		fail(w, r, err)
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	ok(w, c)
}

func (s *Server) signContract(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, service.MaxSignatureBytes*2)
	var in service.SignInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	c, err := s.svc.SignContract(r.Context(), chi.URLParam(r, "token"), in, r.RemoteAddr)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, c)
}

// ── Operational tickets ──────────────────────────────────────

func (s *Server) reportIssue(w http.ResponseWriter, r *http.Request) {
	var in service.IssueInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	t, err := s.svc.ReportIssue(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, t)
}

func (s *Server) assignTask(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in struct {
		AssigneeID *uuid.UUID `json:"assigneeId"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	t, err := s.svc.AssignTask(r.Context(), mustActor(r), id, in.AssigneeID)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, t)
}

// ── Slot check with alternatives ─────────────────────────────

func (s *Server) checkSlot(w http.ResponseWriter, r *http.Request) {
	var q service.SlotQuery
	if err := decode(r, &q); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CheckSlot(r.Context(), mustActor(r), q)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) customerContract(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	var in service.CustomerContractInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CustomerContract(r.Context(), mustActor(r), id, in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, res)
}
