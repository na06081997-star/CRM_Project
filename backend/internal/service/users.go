package service

import (
	"context"
	"net/mail"
	"strings"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/auth"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// ResolveIdentity maps a verified session token to an active account. A
// token issued before the account's sessions were revoked is refused.
func (s *Service) ResolveIdentity(ctx context.Context, id auth.Identity) (domain.User, error) {
	if id.UserID == uuid.Nil {
		return domain.User{}, domain.ErrUnauthorized
	}
	u, err := s.store.UserByID(ctx, id.UserID)
	if err != nil {
		if err == domain.ErrNotFound {
			return u, domain.ErrUnauthorized
		}
		return u, err
	}
	if id.Version != u.TokenVersion {
		return u, domain.ErrUnauthorized
	}
	if !u.IsActive {
		return u, domain.ErrForbidden
	}
	return u, nil
}

type UserInput struct {
	Username       string      `json:"username"`
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
	in.Username = strings.TrimSpace(strings.ToLower(in.Username))
	v := domain.NewValidation()
	if a, err := mail.ParseAddress(in.Email); err != nil || a.Address != in.Email {
		v.Add("email", "כתובת אימייל לא תקינה")
	}
	if in.Username != "" && !ValidUsername(in.Username) {
		v.Add("username", "3–32 תווים: אותיות לטיניות קטנות, ספרות, נקודה, מקף או קו תחתון")
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

// unique checks username / e-mail case-insensitively against other accounts.
func (s *Service) unique(ctx context.Context, id uuid.UUID, username, email string) error {
	v := domain.NewValidation()
	if username != "" {
		if taken, err := s.store.UsernameTaken(ctx, username, id); err != nil {
			return err
		} else if taken {
			v.Add("username", "שם המשתמש כבר תפוס")
		}
	}
	if taken, err := s.store.EmailTaken(ctx, email, id); err != nil {
		return err
	} else if taken {
		v.Add("email", "כבר קיים משתמש עם אימייל זה")
	}
	return v.OrNil()
}

// ListUsers serves both the user administration (with access details) and
// the people pickers of managers (without them).
func (s *Service) ListUsers(ctx context.Context, actor Actor, f store.UserFilter) ([]domain.User, error) {
	if !actor.Can(domain.ActManageUsers) && !actor.Can(domain.ActManageBookings) {
		return nil, domain.ErrForbidden
	}
	users, err := s.store.ListUsers(ctx, f)
	if err != nil {
		return nil, err
	}
	if !actor.Can(domain.ActManageUsers) {
		for i := range users {
			users[i].FirstLoginAt, users[i].LastLoginAt, users[i].ActivatedAt = nil, nil, nil
		}
		return users, nil
	}
	invites, err := s.store.LatestTokens(ctx, purposeInvite)
	if err != nil {
		return nil, err
	}
	now := s.now()
	for i := range users {
		if t, ok := invites[users[i].ID]; ok {
			inv := invitationOf(t, now)
			users[i].Invitation = &inv
		}
	}
	return users, nil
}

// SaveUser edits an account. New accounts go through InviteUser when logins
// use passwords; demo instances create them active right away.
func (s *Service) SaveUser(ctx context.Context, actor Actor, id uuid.UUID, in UserInput) (domain.User, error) {
	if err := actor.Require(domain.ActManageUsers); err != nil {
		return domain.User{}, err
	}
	if err := in.validate(); err != nil {
		return domain.User{}, err
	}
	if id == uuid.Nil && s.cfg.PasswordAuth() {
		return s.InviteUser(ctx, actor, InviteInput{Username: in.Username, Email: in.Email, FullName: in.FullName,
			Role: in.Role, Phone: in.Phone, StaffKind: in.StaffKind})
	}
	v := domain.NewValidation()
	if id == actor.User.ID && (in.Role != actor.User.Role || !in.IsActive) {
		v.Add("role", "לא ניתן להסיר הרשאות ניהול מעצמך")
		return domain.User{}, v
	}
	if !assignable(actor, in.Role) {
		v.Add("role", "אין לך הרשאה להעניק תפקיד זה")
		return domain.User{}, v
	}
	if err := s.unique(ctx, id, in.Username, in.Email); err != nil {
		return domain.User{}, err
	}
	si := store.UserInput{Username: in.Username, Email: in.Email, FullName: in.FullName, Phone: in.Phone, Role: in.Role,
		StaffKind: in.StaffKind, TelegramChatID: in.TelegramChatID, IsActive: in.IsActive}
	var u domain.User
	var err error
	if id == uuid.Nil {
		u, err = s.store.CreateUser(ctx, si)
	} else {
		var before domain.User
		if before, err = s.store.UserByID(ctx, id); err != nil {
			return u, err
		}
		if u, err = s.store.UpdateUser(ctx, id, si); err == nil && (before.IsActive && !u.IsActive || before.Role != u.Role) {
			// Turning an account off or changing its role ends its sessions.
			err = s.store.RevokeSessions(ctx, id)
		}
	}
	if err != nil {
		return u, err
	}
	s.audit(ctx, actor, "user", u.ID, "save", map[string]any{"email": u.Email, "username": u.Username, "role": u.Role, "active": u.IsActive})
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
		Username: u.Username, Email: u.Email, FullName: u.FullName, Phone: u.Phone, Role: u.Role, StaffKind: u.StaffKind,
		TelegramChatID: u.TelegramChatID, IsActive: u.IsActive,
	})
}
