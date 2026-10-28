package service

import (
	"context"
	"errors"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/auth"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// ResolveIdentity maps a verified token to an active user. Firebase users are
// linked by UID, then by email, and created on first login (as admins when
// listed in BOOTSTRAP_ADMIN_EMAILS, otherwise as portal users).
func (s *Service) ResolveIdentity(ctx context.Context, id auth.Identity) (domain.User, error) {
	var u domain.User
	var err error
	switch {
	case id.UserID != uuid.Nil:
		u, err = s.store.UserByID(ctx, id.UserID)
	case id.FirebaseUID != "":
		u, err = s.store.UserByFirebaseUID(ctx, id.FirebaseUID)
		if errors.Is(err, domain.ErrNotFound) && id.Email != "" {
			u, err = s.store.UserByEmail(ctx, id.Email)
			if err == nil {
				err = s.store.LinkFirebaseUID(ctx, u.ID, id.FirebaseUID)
			}
		}
		if errors.Is(err, domain.ErrNotFound) {
			u, err = s.createFirebaseUser(ctx, id)
		}
	default:
		return u, domain.ErrUnauthorized
	}
	if errors.Is(err, domain.ErrNotFound) {
		return u, domain.ErrUnauthorized
	}
	if err != nil {
		return u, err
	}
	if !u.IsActive {
		return u, domain.ErrForbidden
	}
	return u, nil
}

func (s *Service) createFirebaseUser(ctx context.Context, id auth.Identity) (domain.User, error) {
	if id.Email == "" {
		return domain.User{}, domain.ErrUnauthorized
	}
	role := domain.RoleUser
	for _, e := range s.cfg.BootstrapAdminEmails {
		if strings.EqualFold(e, id.Email) {
			role = domain.RoleAdmin
		}
	}
	name := id.Name
	if name == "" {
		name = strings.Split(id.Email, "@")[0]
	}
	uid := id.FirebaseUID
	return s.store.CreateUser(ctx, store.UserInput{Email: id.Email, FullName: name, Role: role, IsActive: true, FirebaseUID: &uid})
}

type UserInput struct {
	Email          string      `json:"email"`
	FullName       string      `json:"fullName"`
	Phone          *string     `json:"phone"`
	Role           domain.Role `json:"role"`
	StaffKind      *string     `json:"staffKind"`
	TelegramChatID *string     `json:"telegramChatId"`
	IsActive       bool        `json:"isActive"`
}

func (in *UserInput) validate() error {
	in.Email = strings.TrimSpace(strings.ToLower(in.Email))
	in.FullName = strings.TrimSpace(in.FullName)
	v := domain.NewValidation()
	if _, err := mail.ParseAddress(in.Email); err != nil {
		v.Add("email", "כתובת אימייל לא תקינה")
	}
	if in.FullName == "" {
		v.Add("fullName", "יש להזין שם מלא")
	}
	if !in.Role.Valid() {
		v.Add("role", "תפקיד לא תקין")
	}
	if in.Role != domain.RoleStaff {
		in.StaffKind = nil
	}
	return v.OrNil()
}

func (s *Service) ListUsers(ctx context.Context, actor Actor, f store.UserFilter) ([]domain.User, error) {
	// Coordinators need the people pickers (coordinators, staff) but not user admin.
	if !actor.Can(domain.ActManageUsers) && !actor.Can(domain.ActManageBookings) {
		return nil, domain.ErrForbidden
	}
	return s.store.ListUsers(ctx, f)
}

func (s *Service) SaveUser(ctx context.Context, actor Actor, id uuid.UUID, in UserInput) (domain.User, error) {
	if err := actor.Require(domain.ActManageUsers); err != nil {
		return domain.User{}, err
	}
	if err := in.validate(); err != nil {
		return domain.User{}, err
	}
	if id == actor.User.ID && (in.Role != domain.RoleAdmin || !in.IsActive) {
		v := domain.NewValidation()
		v.Add("role", "לא ניתן להסיר הרשאות ניהול מעצמך")
		return domain.User{}, v
	}
	si := store.UserInput{Email: in.Email, FullName: in.FullName, Phone: in.Phone, Role: in.Role,
		StaffKind: in.StaffKind, TelegramChatID: in.TelegramChatID, IsActive: in.IsActive}
	var u domain.User
	var err error
	if id == uuid.Nil {
		u, err = s.store.CreateUser(ctx, si)
	} else {
		u, err = s.store.UpdateUser(ctx, id, si)
	}
	if err != nil {
		return u, err
	}
	s.audit(ctx, actor, "user", u.ID, "save", map[string]any{"email": u.Email, "role": u.Role, "active": u.IsActive})
	return u, nil
}

type ProfileInput struct {
	FullName       string  `json:"fullName"`
	Phone          *string `json:"phone"`
	TelegramChatID *string `json:"telegramChatId"`
}

func (s *Service) UpdateProfile(ctx context.Context, actor Actor, in ProfileInput) (domain.User, error) {
	u := actor.User
	if name := strings.TrimSpace(in.FullName); name != "" {
		u.FullName = name
	}
	u.Phone, u.TelegramChatID = in.Phone, in.TelegramChatID
	return s.store.UpdateUser(ctx, u.ID, store.UserInput{
		Email: u.Email, FullName: u.FullName, Phone: u.Phone, Role: u.Role, StaffKind: u.StaffKind,
		TelegramChatID: u.TelegramChatID, IsActive: u.IsActive,
	})
}
