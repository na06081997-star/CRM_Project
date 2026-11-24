package api

import (
	"net"
	"net/http"
	"strings"

	"github.com/na06081997-star/CRM_Project/backend/internal/service"
)

// clientIP trusts only X-Real-Ip, which Traefik overwrites with the peer
// address; the backend is never reachable without it. Other forwarding
// headers can be forged by the client and are ignored.
func clientIP(r *http.Request) string {
	if ip := net.ParseIP(strings.TrimSpace(r.Header.Get("X-Real-Ip"))); ip != nil {
		return ip.String()
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func reqInfo(r *http.Request) service.RequestInfo { return service.RequestInfo{IP: clientIP(r)} }

// noStore keeps auth answers (session tokens, link states) out of caches.
func noStore(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string `json:"identifier"`
		Password   string `json:"password"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.Login(r.Context(), reqInfo(r), in.Identifier, in.Password)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) verifyCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Challenge string `json:"challenge"`
		Code      string `json:"code"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.VerifyCode(r.Context(), reqInfo(r), in.Challenge, in.Code)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) resendCode(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Challenge string `json:"challenge"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.ResendCode(r.Context(), reqInfo(r), in.Challenge)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

// checkLink: the token travels in the body (never in a logged URL).
func (s *Server) checkLink(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Purpose string `json:"purpose"`
		Token   string `json:"token"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.CheckLink(r.Context(), in.Purpose, in.Token)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

type passwordBody struct {
	Token           string `json:"token"`
	Password        string `json:"password"`
	PasswordConfirm string `json:"passwordConfirm"`
}

func (s *Server) activate(w http.ResponseWriter, r *http.Request) {
	var in passwordBody
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	res, err := s.svc.Activate(r.Context(), reqInfo(r), in.Token, in.Password, in.PasswordConfirm)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, res)
}

func (s *Server) forgotPassword(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Identifier string `json:"identifier"`
	}
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.ForgotPassword(r.Context(), reqInfo(r), in.Identifier); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"ok": true,
		"message": "אם קיים חשבון פעיל עם הפרטים האלה, נשלח אליו אימייל עם קישור לבחירת סיסמה חדשה. הקישור בתוקף לזמן מוגבל."})
}

func (s *Server) resetPassword(w http.ResponseWriter, r *http.Request) {
	var in passwordBody
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	if err := s.svc.ResetPassword(r.Context(), reqInfo(r), in.Token, in.Password, in.PasswordConfirm); err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"ok": true})
}

// ── Administration ───────────────────────────────────────────

func (s *Server) inviteUser(w http.ResponseWriter, r *http.Request) {
	var in service.InviteInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	u, err := s.svc.InviteUser(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	created(w, u)
}

func (s *Server) resendInvitation(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	u, err := s.svc.ResendInvitation(r.Context(), mustActor(r), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, u)
}
