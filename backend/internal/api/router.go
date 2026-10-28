// Package api exposes the REST + WebSocket interface.
package api

import (
	"context"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"github.com/na06081997-star/CRM_Project/backend/internal/auth"
	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
)

type Server struct {
	cfg      *config.Config
	svc      *service.Service
	verifier auth.Verifier
	local    *auth.Local
	hub      *realtime.Hub
	files    storage.Store
	version  string
}

func NewServer(cfg *config.Config, svc *service.Service, v auth.Verifier, local *auth.Local, hub *realtime.Hub, files storage.Store, version string) *Server {
	return &Server{cfg: cfg, svc: svc, verifier: v, local: local, hub: hub, files: files, version: version}
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.RealIP, recoverer, requestLogger, securityHeaders, cors(s.cfg.CORSOrigins))
	r.Use(s.authenticate)

	r.Get("/api/health", s.health)

	// ── Public (no login) ──
	r.Route("/api/public", func(r chi.Router) {
		r.Get("/config", s.publicConfig)
		r.Get("/signage", s.publicSignage)
		r.Get("/files/{id}", s.downloadFile)
	})
	r.Get("/ws/signage", func(w http.ResponseWriter, r *http.Request) {
		s.hub.Serve(w, r, []string{realtime.TopicSignage})
	})
	if s.cfg.AuthMode == "dev" {
		r.Get("/api/auth/dev-users", s.devUsers)
		r.Post("/api/auth/dev-login", s.devLogin)
	}

	// ── Authenticated ──
	r.Group(func(r chi.Router) {
		r.Use(requireAuth)
		r.Get("/ws", s.ws)

		r.Route("/api", func(r chi.Router) {
			r.Get("/me", s.me)
			r.Put("/me", s.updateMe)
			r.Get("/dashboard", s.dashboard)
			r.Get("/reports", s.report)
			r.Get("/availability", s.availability)

			r.Route("/bookings", func(r chi.Router) {
				r.Get("/", s.listBookings)
				r.Post("/", s.createBooking)
				r.Post("/check", s.checkBooking)
				r.Get("/{id}", s.getBooking)
				r.Put("/{id}", s.updateBooking)
				r.Delete("/{id}", s.deleteBooking)
				r.Post("/{id}/move", s.moveBooking)
				r.Post("/{id}/cancel", s.cancelBooking)
			})
			r.Route("/requests", func(r chi.Router) {
				r.Get("/", s.listRequests)
				r.Post("/", s.submitRequest)
				r.Post("/{id}/review", s.reviewRequest)
				r.Post("/{id}/cancel", s.cancelRequest)
			})
			r.Route("/buildings", func(r chi.Router) {
				r.Get("/", s.listBuildings)
				r.Post("/", s.saveBuilding)
				r.Put("/{id}", s.saveBuilding)
			})
			r.Route("/rooms", func(r chi.Router) {
				r.Get("/", s.listRooms)
				r.Post("/", s.saveRoom)
				r.Put("/{id}", s.saveRoom)
			})
			r.Route("/equipment", func(r chi.Router) {
				r.Get("/", s.listEquipment)
				r.Post("/", s.saveEquipment)
			})
			r.Route("/activities", func(r chi.Router) {
				r.Get("/", s.listActivities)
				r.Post("/", s.saveActivity)
				r.Put("/{id}", s.saveActivity)
			})
			r.Route("/customers", func(r chi.Router) {
				r.Get("/", s.listCustomers)
				r.Post("/", s.saveCustomer)
				r.Get("/{id}", s.getCustomer)
				r.Put("/{id}", s.saveCustomer)
				r.Delete("/{id}", s.deleteCustomer)
			})
			r.Route("/payments", func(r chi.Router) {
				r.Get("/", s.listPayments)
				r.Post("/", s.recordPayment)
				r.Get("/summary", s.paymentSummary)
				r.Get("/balances", s.balances)
				r.Delete("/{id}", s.deletePayment)
			})
			r.Route("/tasks", func(r chi.Router) {
				r.Get("/", s.listTasks)
				r.Post("/", s.saveTask)
				r.Put("/{id}", s.saveTask)
				r.Post("/{id}/status", s.taskStatus)
				r.Delete("/{id}", s.deleteTask)
			})
			r.Route("/task-schedules", func(r chi.Router) {
				r.Get("/", s.listSchedules)
				r.Post("/", s.saveSchedule)
				r.Put("/{id}", s.saveSchedule)
				r.Delete("/{id}", s.deleteSchedule)
			})
			r.Route("/users", func(r chi.Router) {
				r.Get("/", s.listUsers)
				r.Post("/", s.saveUser)
				r.Put("/{id}", s.saveUser)
			})
			r.Route("/signage", func(r chi.Router) {
				r.Get("/screens", s.listScreens)
				r.Post("/screens", s.saveScreen)
				r.Put("/screens/{id}", s.saveScreen)
				r.Delete("/screens/{id}", s.deleteScreen)
				r.Get("/announcements", s.listAnnouncements)
				r.Post("/announcements", s.saveAnnouncement)
				r.Put("/announcements/{id}", s.saveAnnouncement)
				r.Delete("/announcements/{id}", s.deleteAnnouncement)
			})
			r.Route("/files", func(r chi.Router) {
				r.Get("/", s.listFiles)
				r.Post("/", s.uploadFile)
				r.Get("/{id}", s.downloadFile)
				r.Delete("/{id}", s.deleteFile)
			})
			r.Get("/notifications", s.listNotifications)
			r.Post("/notifications/read", s.readNotifications)
			r.Get("/audit", s.listAudit)
		})
	})
	return r
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	status := map[string]any{"status": "ok", "version": s.version, "time": time.Now()}
	if err := s.svc.Store().Ping(ctx); err != nil {
		status["status"], status["db"] = "degraded", err.Error()
		writeJSON(w, http.StatusServiceUnavailable, status)
		return
	}
	status["storage"] = "ok"
	if err := s.files.Ready(ctx); err != nil {
		status["storage"] = err.Error()
	}
	status["wsClients"] = s.hub.ConnectionCount()
	ok(w, status)
}

func (s *Server) publicConfig(w http.ResponseWriter, _ *http.Request) {
	ok(w, map[string]any{
		"appName":  s.cfg.AppName,
		"authMode": s.cfg.AuthMode,
		"timezone": s.cfg.Location.String(),
		"version":  s.version,
		"firebase": map[string]string{
			"apiKey": s.cfg.FirebaseAPIKey, "authDomain": s.cfg.FirebaseAuthDomain,
			"projectId": s.cfg.FirebaseProjectID, "appId": s.cfg.FirebaseAppID,
		},
	})
}

func (s *Server) ws(w http.ResponseWriter, r *http.Request) {
	a := mustActor(r)
	topics := []string{realtime.UserTopic(a.User.ID.String())}
	if a.Can(domain.ActViewCalendar) {
		topics = append(topics, realtime.TopicStaff)
	}
	s.hub.Serve(w, r, topics)
}
