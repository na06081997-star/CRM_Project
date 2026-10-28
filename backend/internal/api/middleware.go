package api

import (
	"context"
	"log/slog"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
)

type ctxKey int

const actorKey ctxKey = 1

func actorFrom(ctx context.Context) (service.Actor, bool) {
	a, ok := ctx.Value(actorKey).(service.Actor)
	return a, ok
}

func mustActor(r *http.Request) service.Actor {
	a, _ := actorFrom(r.Context())
	return a
}

func bearer(r *http.Request) string {
	if h := r.Header.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// Browsers cannot set headers on WebSocket handshakes.
	if strings.HasPrefix(r.URL.Path, "/ws") {
		return r.URL.Query().Get("token")
	}
	return ""
}

// authenticate resolves the bearer token to an Actor when present.
func (s *Server) authenticate(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tok := bearer(r)
		if tok == "" {
			next.ServeHTTP(w, r)
			return
		}
		id, err := s.verifier.Verify(r.Context(), tok)
		if err != nil {
			fail(w, r, domain.ErrUnauthorized)
			return
		}
		u, err := s.svc.ResolveIdentity(r.Context(), id)
		if err != nil {
			fail(w, r, err)
			return
		}
		ctx := context.WithValue(r.Context(), actorKey, service.Actor{User: u})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := actorFrom(r.Context()); !ok {
			fail(w, r, domain.ErrUnauthorized)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func requestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		next.ServeHTTP(ww, r)
		if r.URL.Path == "/api/health" {
			return
		}
		slog.Info("http", "method", r.Method, "path", r.URL.Path, "status", ww.Status(),
			"ms", time.Since(start).Milliseconds(), "req_id", middleware.GetReqID(r.Context()))
	})
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil && rec != http.ErrAbortHandler {
				slog.Error("panic", "err", rec, "stack", string(debug.Stack()))
				writeJSON(w, http.StatusInternalServerError, errorBody{Error: "internal", Message: "אירעה שגיאה, נסו שוב"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "strict-origin-when-cross-origin")
		h.Set("X-Frame-Options", "DENY")
		next.ServeHTTP(w, r)
	})
}

func cors(origins []string) func(http.Handler) http.Handler {
	allowed := map[string]bool{}
	for _, o := range origins {
		allowed[o] = true
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if o := r.Header.Get("Origin"); o != "" && allowed[o] {
				w.Header().Set("Access-Control-Allow-Origin", o)
				w.Header().Set("Vary", "Origin")
				w.Header().Set("Access-Control-Allow-Headers", "Authorization, Content-Type")
				w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
				if r.Method == http.MethodOptions {
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
			next.ServeHTTP(w, r)
		})
	}
}
