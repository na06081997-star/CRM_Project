package store

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

// ── Buildings ────────────────────────────────────────────────

const buildingCols = `id, name, address, to_char(opens_at, 'HH24:MI'), to_char(closes_at, 'HH24:MI')`

func scanBuilding(r pgx.Row) (domain.Building, error) {
	var b domain.Building
	err := r.Scan(&b.ID, &b.Name, &b.Address, &b.OpensAt, &b.ClosesAt)
	return b, mapErr(err)
}

func (s *Store) ListBuildings(ctx context.Context) ([]domain.Building, error) {
	rows, err := s.q.Query(ctx, `SELECT `+buildingCols+` FROM buildings ORDER BY name`)
	return collect(rows, err, scanBuilding)
}

func (s *Store) BuildingByID(ctx context.Context, id uuid.UUID) (domain.Building, error) {
	return scanBuilding(s.q.QueryRow(ctx, `SELECT `+buildingCols+` FROM buildings WHERE id = $1`, id))
}

func (s *Store) CreateBuilding(ctx context.Context, b domain.Building) (domain.Building, error) {
	return scanBuilding(s.q.QueryRow(ctx, `
		INSERT INTO buildings (name, address, opens_at, closes_at) VALUES ($1, $2, $3::time, $4::time)
		RETURNING `+buildingCols, b.Name, b.Address, b.OpensAt, b.ClosesAt))
}

func (s *Store) UpdateBuilding(ctx context.Context, b domain.Building) (domain.Building, error) {
	return scanBuilding(s.q.QueryRow(ctx, `
		UPDATE buildings SET name = $2, address = $3, opens_at = $4::time, closes_at = $5::time
		WHERE id = $1 RETURNING `+buildingCols, b.ID, b.Name, b.Address, b.OpensAt, b.ClosesAt))
}

// ── Rooms ────────────────────────────────────────────────────

const roomCols = `id, building_id, name, kind, capacity, color, description, rules, equipment, hourly_rate,
	min_hours, buffer_minutes, is_bookable, is_active, image_key, sort_order`

func scanRoom(r pgx.Row) (domain.Room, error) {
	var m domain.Room
	err := r.Scan(&m.ID, &m.BuildingID, &m.Name, &m.Kind, &m.Capacity, &m.Color, &m.Description, &m.Rules,
		&m.Equipment, &m.HourlyRate, &m.MinHours, &m.BufferMinutes, &m.IsBookable, &m.IsActive, &m.ImageKey, &m.SortOrder)
	return m, mapErr(err)
}

type RoomFilter struct {
	BuildingID *uuid.UUID
	OnlyActive bool
	IDs        []uuid.UUID
}

func (s *Store) ListRooms(ctx context.Context, f RoomFilter) ([]domain.Room, error) {
	w := &where{}
	if f.BuildingID != nil {
		w.add("building_id = $%d", *f.BuildingID)
	}
	if f.OnlyActive {
		w.raw("is_active")
	}
	if len(f.IDs) > 0 {
		w.add("id = ANY($%d)", f.IDs)
	}
	rows, err := s.q.Query(ctx, `SELECT `+roomCols+` FROM rooms`+w.sql()+` ORDER BY sort_order, name`, w.args...)
	return collect(rows, err, scanRoom)
}

func (s *Store) RoomByID(ctx context.Context, id uuid.UUID) (domain.Room, error) {
	return scanRoom(s.q.QueryRow(ctx, `SELECT `+roomCols+` FROM rooms WHERE id = $1`, id))
}

func (s *Store) CreateRoom(ctx context.Context, m domain.Room) (domain.Room, error) {
	return scanRoom(s.q.QueryRow(ctx, `
		INSERT INTO rooms (building_id, name, kind, capacity, color, description, rules, equipment, hourly_rate,
		                   min_hours, buffer_minutes, is_bookable, is_active, image_key, sort_order)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15)
		RETURNING `+roomCols,
		m.BuildingID, m.Name, m.Kind, m.Capacity, m.Color, m.Description, m.Rules, nonNil(m.Equipment), m.HourlyRate,
		m.MinHours, m.BufferMinutes, m.IsBookable, m.IsActive, m.ImageKey, m.SortOrder))
}

func (s *Store) UpdateRoom(ctx context.Context, m domain.Room) (domain.Room, error) {
	return scanRoom(s.q.QueryRow(ctx, `
		UPDATE rooms SET building_id = $2, name = $3, kind = $4, capacity = $5, color = $6, description = $7,
		       rules = $8, equipment = $9, hourly_rate = $10, min_hours = $11, buffer_minutes = $12,
		       is_bookable = $13, is_active = $14, image_key = $15, sort_order = $16, updated_at = now()
		WHERE id = $1 RETURNING `+roomCols,
		m.ID, m.BuildingID, m.Name, m.Kind, m.Capacity, m.Color, m.Description, m.Rules, nonNil(m.Equipment),
		m.HourlyRate, m.MinHours, m.BufferMinutes, m.IsBookable, m.IsActive, m.ImageKey, m.SortOrder))
}

// ── Equipment catalogue ──────────────────────────────────────

func scanEquipment(r pgx.Row) (domain.EquipmentItem, error) {
	var e domain.EquipmentItem
	err := r.Scan(&e.ID, &e.Code, &e.Label, &e.FlatPrice)
	return e, mapErr(err)
}

func (s *Store) ListEquipment(ctx context.Context) ([]domain.EquipmentItem, error) {
	rows, err := s.q.Query(ctx, `SELECT id, code, label, flat_price FROM equipment_items ORDER BY label`)
	return collect(rows, err, scanEquipment)
}

func (s *Store) UpsertEquipment(ctx context.Context, e domain.EquipmentItem) (domain.EquipmentItem, error) {
	return scanEquipment(s.q.QueryRow(ctx, `
		INSERT INTO equipment_items (code, label, flat_price) VALUES ($1, $2, $3)
		ON CONFLICT (code) DO UPDATE SET label = EXCLUDED.label, flat_price = EXCLUDED.flat_price
		RETURNING id, code, label, flat_price`, e.Code, e.Label, e.FlatPrice))
}

// ── Activities ───────────────────────────────────────────────

func scanActivity(r pgx.Row) (domain.Activity, error) {
	var a domain.Activity
	err := r.Scan(&a.ID, &a.Name, &a.Description, &a.CoordinatorID, &a.DefaultRoomID, &a.Color, &a.Audience,
		&a.IsActive, &a.SessionsCount)
	return a, mapErr(err)
}

const activitySelect = `
	SELECT a.id, a.name, a.description, a.coordinator_id, a.default_room_id, a.color, a.audience, a.is_active,
	       (SELECT count(*) FROM bookings b WHERE b.activity_id = a.id AND b.status = 'confirmed')
	FROM activities a`

func (s *Store) ListActivities(ctx context.Context) ([]domain.Activity, error) {
	rows, err := s.q.Query(ctx, activitySelect+` ORDER BY a.is_active DESC, a.name`)
	return collect(rows, err, scanActivity)
}

func (s *Store) ActivityByID(ctx context.Context, id uuid.UUID) (domain.Activity, error) {
	return scanActivity(s.q.QueryRow(ctx, activitySelect+` WHERE a.id = $1`, id))
}

func (s *Store) SaveActivity(ctx context.Context, a domain.Activity) (domain.Activity, error) {
	var id uuid.UUID
	var err error
	if a.ID == uuid.Nil {
		err = s.q.QueryRow(ctx, `
			INSERT INTO activities (name, description, coordinator_id, default_room_id, color, audience, is_active)
			VALUES ($1, $2, $3, $4, $5, $6, $7) RETURNING id`,
			a.Name, a.Description, a.CoordinatorID, a.DefaultRoomID, a.Color, a.Audience, a.IsActive).Scan(&id)
	} else {
		err = s.q.QueryRow(ctx, `
			UPDATE activities SET name = $2, description = $3, coordinator_id = $4, default_room_id = $5,
			       color = $6, audience = $7, is_active = $8 WHERE id = $1 RETURNING id`,
			a.ID, a.Name, a.Description, a.CoordinatorID, a.DefaultRoomID, a.Color, a.Audience, a.IsActive).Scan(&id)
	}
	if err != nil {
		return domain.Activity{}, mapErr(err)
	}
	return s.ActivityByID(ctx, id)
}

func nonNil(s []string) []string {
	if s == nil {
		return []string{}
	}
	return s
}
