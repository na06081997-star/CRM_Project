package service

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/notify"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// ContractLinkTTL is how long a signing link stays valid after it is sent.
const ContractLinkTTL = 14 * 24 * time.Hour

// MaxSignatureBytes bounds the drawn signature image.
const MaxSignatureBytes = 512 << 10

// CoordinatorInput adds a רכז, optionally with a contract to sign first.
type CoordinatorInput struct {
	FullName string `json:"fullName"`
	Email    string `json:"email"`
	Phone    string `json:"phone"`
	// Contract: "digital" (link sent by SMS + email, the account opens once
	// signed), "scan" (a signed paper copy will be uploaded) or "none"
	// (account active right away).
	Contract string `json:"contract"`
	Title    string `json:"title"`
	Body     string `json:"body"`
}

type CoordinatorResult struct {
	User     domain.User      `json:"user"`
	Contract *domain.Contract `json:"contract"`
	// SignURL is the signing link (digital contracts), to copy or send again.
	SignURL string `json:"signUrl,omitempty"`
}

func (s *Service) CreateCoordinator(ctx context.Context, actor Actor, in CoordinatorInput) (CoordinatorResult, error) {
	if err := actor.Require(domain.ActManageCoords); err != nil {
		return CoordinatorResult{}, err
	}
	in.Phone = strings.TrimSpace(in.Phone)
	ui := UserInput{Email: in.Email, FullName: in.FullName, Role: domain.RoleCoordinator, IsActive: in.Contract == "none"}
	if in.Phone != "" {
		ui.Phone = &in.Phone
	}
	if err := ui.validate(); err != nil {
		return CoordinatorResult{}, err
	}
	v := domain.NewValidation()
	switch in.Contract {
	case "none":
	case "digital", "scan":
		if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Body) == "" {
			v.Add("body", "יש להזין כותרת ונוסח לחוזה")
		}
		if in.Contract == "digital" && in.Phone == "" {
			v.Add("phone", "לשליחת קישור בהודעה יש להזין טלפון")
		}
	default:
		v.Add("contract", "יש לבחור אפשרות חוזה")
	}
	if in.Phone != "" {
		if p := notify.NormalizeIsraeliPhone(in.Phone); len(p) < 12 || len(p) > 13 {
			v.Add("phone", "מספר טלפון לא תקין")
		}
	}
	if err := v.OrNil(); err != nil {
		return CoordinatorResult{}, err
	}
	if _, err := s.store.UserByEmail(ctx, ui.Email); err == nil {
		v.Add("email", "כבר קיים משתמש עם אימייל זה")
		return CoordinatorResult{}, v
	}

	var res CoordinatorResult
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		u, err := tx.CreateUser(ctx, store.UserInput{Email: ui.Email, FullName: ui.FullName, Phone: ui.Phone, Role: ui.Role, IsActive: ui.IsActive})
		if err != nil {
			return err
		}
		res.User = u
		if in.Contract == "none" {
			return nil
		}
		c, err := newContract(ctx, tx, actor, u.ID, in.Contract, in.Title, in.Body)
		res.Contract = &c
		return err
	})
	if err != nil {
		return CoordinatorResult{}, err
	}
	s.audit(ctx, actor, "user", res.User.ID, "create_coordinator", map[string]any{"email": res.User.Email, "contract": in.Contract})
	if res.Contract != nil && res.Contract.Method == "digital" {
		if err := s.sendContract(ctx, *res.Contract, res.User); err != nil {
			return res, err
		}
		res.SignURL = s.signURL(res.Contract.Token)
	}
	return res, nil
}

func newContract(ctx context.Context, tx *store.Store, actor Actor, userID uuid.UUID, method, title, body string) (domain.Contract, error) {
	token, err := randomToken()
	if err != nil {
		return domain.Contract{}, err
	}
	return tx.CreateContract(ctx, domain.Contract{
		UserID: userID, Title: strings.TrimSpace(title), Body: strings.TrimSpace(body), Method: method,
		Token: token, ExpiresAt: time.Now().Add(ContractLinkTTL),
	}, actor.ID())
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// signURL is the public signing page; the first CORS origin is the site URL.
func (s *Service) signURL(token string) string {
	base := ""
	if len(s.cfg.CORSOrigins) > 0 {
		base = strings.TrimRight(s.cfg.CORSOrigins[0], "/")
	}
	return base + "/sign/" + token
}

// sendContract messages the signing link (SMS / e-mail, per NOTIFY_CHANNELS).
func (s *Service) sendContract(ctx context.Context, c domain.Contract, u domain.User) error {
	expires := time.Now().Add(ContractLinkTTL)
	if err := s.store.MarkContractSent(ctx, c.ID, expires); err != nil {
		return err
	}
	org, _ := s.store.Organization(ctx)
	s.notifyUser(u, "חוזה לחתימה – "+org.Name,
		fmt.Sprintf("שלום %s,\nמצורף קישור לחתימה דיגיטלית על \"%s\":\n%s\nלאחר החתימה החשבון שלך במערכת ייפתח. הקישור בתוקף עד %s.",
			u.FullName, c.Title, s.signURL(c.Token), expires.In(s.loc).Format("02/01/2006")))
	return nil
}

func (s *Service) ListContracts(ctx context.Context, actor Actor, userID *uuid.UUID) ([]domain.Contract, error) {
	if err := actor.Require(domain.ActManageCoords); err != nil {
		return nil, err
	}
	return s.store.ListContracts(ctx, userID)
}

type ContractInput struct {
	UserID uuid.UUID `json:"userId"`
	Method string    `json:"method"` // digital | scan
	Title  string    `json:"title"`
	Body   string    `json:"body"`
}

// CreateContract prepares a contract for an existing coordinator (and sends a digital one).
func (s *Service) CreateContract(ctx context.Context, actor Actor, in ContractInput) (CoordinatorResult, error) {
	if err := actor.Require(domain.ActManageCoords); err != nil {
		return CoordinatorResult{}, err
	}
	u, err := s.store.UserByID(ctx, in.UserID)
	if err != nil || u.Role != domain.RoleCoordinator {
		return CoordinatorResult{}, domain.ErrNotFound
	}
	v := domain.NewValidation()
	if in.Method != "digital" && in.Method != "scan" {
		v.Add("method", "יש לבחור אופן חתימה")
	}
	if strings.TrimSpace(in.Title) == "" || strings.TrimSpace(in.Body) == "" {
		v.Add("body", "יש להזין כותרת ונוסח לחוזה")
	}
	if err := v.OrNil(); err != nil {
		return CoordinatorResult{}, err
	}
	var c domain.Contract
	if err := s.store.InTx(ctx, func(tx *store.Store) error {
		c, err = newContract(ctx, tx, actor, u.ID, in.Method, in.Title, in.Body)
		return err
	}); err != nil {
		return CoordinatorResult{}, err
	}
	s.audit(ctx, actor, "contract", c.ID, "create", map[string]any{"user": u.Email, "method": c.Method})
	res := CoordinatorResult{User: u, Contract: &c}
	if c.Method == "digital" {
		if err := s.sendContract(ctx, c, u); err != nil {
			return res, err
		}
		res.SignURL = s.signURL(c.Token)
	}
	return res, nil
}

// ResendContract sends the link again and renews its validity.
func (s *Service) ResendContract(ctx context.Context, actor Actor, id uuid.UUID) (CoordinatorResult, error) {
	if err := actor.Require(domain.ActManageCoords); err != nil {
		return CoordinatorResult{}, err
	}
	c, err := s.store.ContractByID(ctx, id)
	if err != nil {
		return CoordinatorResult{}, err
	}
	if c.Status != domain.ContractPending || c.Method != "digital" {
		v := domain.NewValidation()
		v.Add("id", "ניתן לשלוח שוב רק חוזה דיגיטלי שממתין לחתימה")
		return CoordinatorResult{}, v
	}
	u, err := s.store.UserByID(ctx, c.UserID)
	if err != nil {
		return CoordinatorResult{}, err
	}
	if err := s.sendContract(ctx, c, u); err != nil {
		return CoordinatorResult{}, err
	}
	s.audit(ctx, actor, "contract", c.ID, "resend", nil)
	c, _ = s.store.ContractByID(ctx, id)
	return CoordinatorResult{User: u, Contract: &c, SignURL: s.signURL(c.Token)}, nil
}

// MarkContractSigned closes a contract whose signed scan was uploaded and opens the account.
func (s *Service) MarkContractSigned(ctx context.Context, actor Actor, id uuid.UUID) (domain.Contract, error) {
	if err := actor.Require(domain.ActManageCoords); err != nil {
		return domain.Contract{}, err
	}
	c, err := s.store.ContractByID(ctx, id)
	if err != nil {
		return c, err
	}
	files, err := s.store.ListAttachments(ctx, "contract", c.ID)
	if err != nil {
		return c, err
	}
	if len(files) == 0 {
		v := domain.NewValidation()
		v.Add("file", "יש להעלות קודם את החוזה החתום (סריקה או צילום)")
		return c, v
	}
	if err := s.completeContract(ctx, c, "סריקת חוזה חתום ("+actor.User.FullName+")", ""); err != nil {
		return c, err
	}
	s.audit(ctx, actor, "contract", c.ID, "mark_signed", nil)
	return s.store.ContractByID(ctx, id)
}

func (s *Service) CancelContract(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := actor.Require(domain.ActManageCoords); err != nil {
		return err
	}
	if err := s.store.CancelContract(ctx, id); err != nil {
		return err
	}
	s.audit(ctx, actor, "contract", id, "cancel", nil)
	return nil
}

// ── Public signing page ──────────────────────────────────────

// PublicContract is what the signer sees (no internal data).
type PublicContract struct {
	Title     string     `json:"title"`
	Body      string     `json:"body"`
	Status    string     `json:"status"` // pending | signed | cancelled | expired
	Name      string     `json:"name"`
	SignedAt  *time.Time `json:"signedAt"`
	ExpiresAt time.Time  `json:"expiresAt"`
}

func (s *Service) contractForToken(ctx context.Context, token string) (domain.Contract, error) {
	if len(token) < 20 || len(token) > 100 {
		return domain.Contract{}, domain.ErrNotFound
	}
	c, err := s.store.ContractByToken(ctx, token)
	if err != nil || c.Method != "digital" {
		return domain.Contract{}, domain.ErrNotFound
	}
	return c, nil
}

func (s *Service) PublicContract(ctx context.Context, token string) (PublicContract, error) {
	c, err := s.contractForToken(ctx, token)
	if err != nil {
		return PublicContract{}, err
	}
	status := c.Status
	if status == domain.ContractPending && s.now().After(c.ExpiresAt) {
		status = "expired"
	}
	return PublicContract{Title: c.Title, Body: c.Body, Status: status, Name: c.UserName, SignedAt: c.SignedAt, ExpiresAt: c.ExpiresAt}, nil
}

type SignInput struct {
	Name      string `json:"name"`
	Agree     bool   `json:"agree"`
	Signature string `json:"signature"` // data:image/png;base64,…
}

// SignContract records an online signature and opens the coordinator's account.
func (s *Service) SignContract(ctx context.Context, token string, in SignInput, ip string) (PublicContract, error) {
	c, err := s.contractForToken(ctx, token)
	if err != nil {
		return PublicContract{}, err
	}
	v := domain.NewValidation()
	switch {
	case c.Status == domain.ContractSigned:
		v.Add("token", "החוזה כבר נחתם")
	case c.Status != domain.ContractPending:
		v.Add("token", "החוזה בוטל")
	case s.now().After(c.ExpiresAt):
		v.Add("token", "תוקף הקישור פג – יש לבקש קישור חדש מהמתנ״ס")
	}
	name := strings.TrimSpace(in.Name)
	if len([]rune(name)) < 2 || len([]rune(name)) > 120 {
		v.Add("name", "יש להזין שם מלא")
	}
	if !in.Agree {
		v.Add("agree", "יש לאשר שקראת את החוזה")
	}
	png, err := decodePNG(in.Signature)
	if err != nil {
		v.Add("signature", "יש לחתום בתיבת החתימה")
	}
	if err := v.OrNil(); err != nil {
		return PublicContract{}, err
	}
	if _, err := s.storeFile(ctx, c.UserID, "contract", c.ID, "חתימה.png", "image/png", bytes.NewReader(png), int64(len(png)), nil); err != nil {
		return PublicContract{}, err
	}
	if err := s.completeContract(ctx, c, name, ip); err != nil {
		return PublicContract{}, err
	}
	return s.PublicContract(ctx, token)
}

// decodePNG checks a base64 data URL holds a reasonably sized PNG.
func decodePNG(dataURL string) ([]byte, error) {
	const prefix = "data:image/png;base64,"
	if !strings.HasPrefix(dataURL, prefix) || len(dataURL) > MaxSignatureBytes*4/3+len(prefix)+4 {
		return nil, errors.New("not a png data url")
	}
	b, err := base64.StdEncoding.DecodeString(dataURL[len(prefix):])
	if err != nil || len(b) < 100 || len(b) > MaxSignatureBytes || !bytes.HasPrefix(b, []byte("\x89PNG\r\n\x1a\n")) {
		return nil, errors.New("invalid png")
	}
	return b, nil
}

// completeContract marks the contract signed, activates the account and tells everyone.
func (s *Service) completeContract(ctx context.Context, c domain.Contract, name, ip string) error {
	var signed bool
	err := s.store.InTx(ctx, func(tx *store.Store) error {
		var err error
		if signed, err = tx.SignContract(ctx, c.ID, name, ip); err != nil || !signed {
			return err
		}
		return tx.SetUserActive(ctx, c.UserID, true)
	})
	if err != nil {
		return err
	}
	if !signed {
		v := domain.NewValidation()
		v.Add("token", "החוזה כבר טופל")
		return v
	}
	if u, err := s.store.UserByID(ctx, c.UserID); err == nil {
		site := strings.TrimSuffix(s.signURL(""), "/sign/")
		s.notifyUser(u, "החשבון שלך הופעל ✅",
			fmt.Sprintf("שלום %s,\nהחוזה \"%s\" נחתם והחשבון שלך במערכת פעיל.\nכניסה: %s (עם חשבון Google של %s)", u.FullName, c.Title, site, u.Email))
	}
	s.notifyManagers(ctx, "חוזה נחתם", fmt.Sprintf("%s חתם/ה על \"%s\". החשבון הופעל.", c.UserName, c.Title))
	s.emit(ctx, EvUserChanged, map[string]any{"id": c.UserID}, realtime.TopicStaff)
	return nil
}
