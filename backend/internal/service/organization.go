package service

import (
	"context"
	"log/slog"
	"strings"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
)

// OrganizationEntityID anchors uploaded client logos in the attachments table.
var OrganizationEntityID = uuid.MustParse("00000000-0000-0000-0000-000000000001")

func (s *Service) Organization(ctx context.Context) (domain.Organization, error) {
	return s.store.Organization(ctx)
}

func (s *Service) SaveOrganization(ctx context.Context, actor Actor, o domain.Organization) (domain.Organization, error) {
	if err := actor.Require(domain.ActManageSettings); err != nil {
		return o, err
	}
	for _, f := range []*string{&o.Name, &o.ShortName, &o.Subtitle, &o.LogoURL, &o.MarkURL, &o.BrandColor,
		&o.Phone, &o.Email, &o.Website, &o.Address} {
		*f = strings.TrimSpace(*f)
	}
	v := domain.NewValidation()
	if o.Name == "" {
		v.Add("name", "יש להזין את שם הארגון")
	}
	if len([]rune(o.Name)) > 120 || len([]rune(o.ShortName)) > 40 || len([]rune(o.Subtitle)) > 80 {
		v.Add("name", "השם ארוך מדי")
	}
	if !validAssetURL(o.LogoURL) {
		v.Add("logoUrl", "כתובת לוגו לא תקינה")
	}
	if !validAssetURL(o.MarkURL) {
		v.Add("markUrl", "כתובת סמל לא תקינה")
	}
	if o.BrandColor != "" && !hexColor.MatchString(o.BrandColor) {
		v.Add("brandColor", "צבע לא תקין")
	}
	if o.Website != "" && !strings.HasPrefix(o.Website, "https://") && !strings.HasPrefix(o.Website, "http://") {
		v.Add("website", "כתובת אתר צריכה להתחיל ב-https://")
	}
	if err := v.OrNil(); err != nil {
		return o, err
	}
	saved, err := s.store.SaveOrganization(ctx, o)
	if err != nil {
		return saved, err
	}
	s.audit(ctx, actor, "organization", OrganizationEntityID, "update", map[string]any{"name": saved.Name})
	s.emit(ctx, "organization.changed", saved, realtime.TopicStaff, realtime.TopicAvailability, realtime.TopicSignage)
	return saved, nil
}

// validAssetURL accepts same-origin paths and absolute https URLs only.
func validAssetURL(u string) bool {
	return u == "" || (strings.HasPrefix(u, "/") && !strings.HasPrefix(u, "//")) || strings.HasPrefix(u, "https://")
}

// BootstrapOrganization fills an unconfigured organization from ORG_* variables,
// so a fresh client deployment shows its name before anyone opens the settings.
func (s *Service) BootstrapOrganization(ctx context.Context) error {
	o, err := s.store.Organization(ctx)
	if err != nil || o.Name != "" || s.cfg.OrgName == "" {
		return err
	}
	o.Name, o.ShortName, o.Subtitle = s.cfg.OrgName, s.cfg.OrgShortName, s.cfg.OrgSubtitle
	o.LogoURL, o.MarkURL, o.BrandColor = s.cfg.OrgLogoURL, s.cfg.OrgMarkURL, s.cfg.OrgBrandColor
	if !validAssetURL(o.LogoURL) || !validAssetURL(o.MarkURL) || (o.BrandColor != "" && !hexColor.MatchString(o.BrandColor)) {
		slog.Warn("ignoring invalid ORG_LOGO_URL / ORG_MARK_URL / ORG_BRAND_COLOR")
		o.LogoURL, o.MarkURL, o.BrandColor = "", "", ""
	}
	_, err = s.store.SaveOrganization(ctx, o)
	return err
}
