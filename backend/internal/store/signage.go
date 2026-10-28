package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const screenCols = `id, slug, name, building_id, theme, welcome_text, rotate_seconds, room_ids, is_active`

func scanScreen(r pgx.Row) (domain.SignageScreen, error) {
	var s domain.SignageScreen
	err := r.Scan(&s.ID, &s.Slug, &s.Name, &s.BuildingID, &s.Theme, &s.WelcomeText, &s.RotateSeconds, &s.RoomIDs, &s.IsActive)
	return s, mapErr(err)
}

func (s *Store) ListScreens(ctx context.Context) ([]domain.SignageScreen, error) {
	rows, err := s.q.Query(ctx, `SELECT `+screenCols+` FROM signage_screens ORDER BY name`)
	return collect(rows, err, scanScreen)
}

func (s *Store) ScreenBySlug(ctx context.Context, slug string) (domain.SignageScreen, error) {
	return scanScreen(s.q.QueryRow(ctx, `SELECT `+screenCols+` FROM signage_screens WHERE slug = $1 AND is_active`, slug))
}

func (s *Store) SaveScreen(ctx context.Context, sc domain.SignageScreen) (domain.SignageScreen, error) {
	if sc.RoomIDs == nil {
		sc.RoomIDs = []uuid.UUID{}
	}
	if sc.ID == uuid.Nil {
		return scanScreen(s.q.QueryRow(ctx, `
			INSERT INTO signage_screens (slug, name, building_id, theme, welcome_text, rotate_seconds, room_ids, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+screenCols,
			sc.Slug, sc.Name, sc.BuildingID, sc.Theme, sc.WelcomeText, sc.RotateSeconds, sc.RoomIDs, sc.IsActive))
	}
	return scanScreen(s.q.QueryRow(ctx, `
		UPDATE signage_screens SET slug = $2, name = $3, building_id = $4, theme = $5, welcome_text = $6,
		       rotate_seconds = $7, room_ids = $8, is_active = $9
		WHERE id = $1 RETURNING `+screenCols,
		sc.ID, sc.Slug, sc.Name, sc.BuildingID, sc.Theme, sc.WelcomeText, sc.RotateSeconds, sc.RoomIDs, sc.IsActive))
}

func (s *Store) DeleteScreen(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.Exec(ctx, `DELETE FROM signage_screens WHERE id = $1`, id)
	return mapErr(err)
}

const announcementCols = `id, title, body, image_key, starts_at, ends_at, priority, screen_id, created_at`

func scanAnnouncement(r pgx.Row) (domain.Announcement, error) {
	var a domain.Announcement
	err := r.Scan(&a.ID, &a.Title, &a.Body, &a.ImageKey, &a.StartsAt, &a.EndsAt, &a.Priority, &a.ScreenID, &a.CreatedAt)
	return a, mapErr(err)
}

func (s *Store) ListAnnouncements(ctx context.Context) ([]domain.Announcement, error) {
	rows, err := s.q.Query(ctx, `SELECT `+announcementCols+` FROM announcements ORDER BY starts_at DESC`)
	return collect(rows, err, scanAnnouncement)
}

// ActiveAnnouncements returns announcements visible at `now` on a screen
// (global ones included).
func (s *Store) ActiveAnnouncements(ctx context.Context, screenID *uuid.UUID, now time.Time) ([]domain.Announcement, error) {
	rows, err := s.q.Query(ctx, `SELECT `+announcementCols+` FROM announcements
		WHERE starts_at <= $1 AND (ends_at IS NULL OR ends_at > $1)
		  AND (screen_id IS NULL OR screen_id = $2)
		ORDER BY priority DESC, starts_at DESC LIMIT 20`, now, screenID)
	return collect(rows, err, scanAnnouncement)
}

func (s *Store) SaveAnnouncement(ctx context.Context, a domain.Announcement, by *uuid.UUID) (domain.Announcement, error) {
	if a.ID == uuid.Nil {
		return scanAnnouncement(s.q.QueryRow(ctx, `
			INSERT INTO announcements (title, body, image_key, starts_at, ends_at, priority, screen_id, created_by)
			VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING `+announcementCols,
			a.Title, a.Body, a.ImageKey, a.StartsAt, a.EndsAt, a.Priority, a.ScreenID, by))
	}
	return scanAnnouncement(s.q.QueryRow(ctx, `
		UPDATE announcements SET title = $2, body = $3, image_key = $4, starts_at = $5, ends_at = $6,
		       priority = $7, screen_id = $8
		WHERE id = $1 RETURNING `+announcementCols,
		a.ID, a.Title, a.Body, a.ImageKey, a.StartsAt, a.EndsAt, a.Priority, a.ScreenID))
}

func (s *Store) DeleteAnnouncement(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.Exec(ctx, `DELETE FROM announcements WHERE id = $1`, id)
	return mapErr(err)
}
