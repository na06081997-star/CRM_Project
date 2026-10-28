package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
)

const taskSelect = `
	SELECT t.id, t.title, t.description, t.kind, t.status, t.room_id, t.booking_id, t.schedule_id, t.assignee_id,
	       t.due_at, t.completed_at, t.completion_note, t.reminder_sent_at, t.created_at, r.name, u.full_name
	FROM tasks t
	LEFT JOIN rooms r ON r.id = t.room_id
	LEFT JOIN users u ON u.id = t.assignee_id`

func scanTask(r pgx.Row) (domain.Task, error) {
	var t domain.Task
	err := r.Scan(&t.ID, &t.Title, &t.Description, &t.Kind, &t.Status, &t.RoomID, &t.BookingID, &t.ScheduleID,
		&t.AssigneeID, &t.DueAt, &t.CompletedAt, &t.CompletionNote, &t.ReminderSentAt, &t.CreatedAt,
		&t.RoomName, &t.AssigneeName)
	return t, mapErr(err)
}

type TaskFilter struct {
	AssigneeID *uuid.UUID
	Statuses   []string
	From, To   time.Time
}

func (s *Store) ListTasks(ctx context.Context, f TaskFilter) ([]domain.Task, error) {
	w := &where{}
	if f.AssigneeID != nil {
		w.add("t.assignee_id = $%d", *f.AssigneeID)
	}
	if len(f.Statuses) > 0 {
		w.add("t.status::text = ANY($%d)", f.Statuses)
	}
	if !f.From.IsZero() {
		w.add("t.due_at >= $%d", f.From)
	}
	if !f.To.IsZero() {
		w.add("t.due_at < $%d", f.To)
	}
	rows, err := s.q.Query(ctx, taskSelect+w.sql()+` ORDER BY t.due_at`, w.args...)
	return collect(rows, err, scanTask)
}

func (s *Store) TaskByID(ctx context.Context, id uuid.UUID) (domain.Task, error) {
	return scanTask(s.q.QueryRow(ctx, taskSelect+` WHERE t.id = $1`, id))
}

func (s *Store) CreateTask(ctx context.Context, t domain.Task, createdBy *uuid.UUID) (domain.Task, error) {
	var id uuid.UUID
	err := s.q.QueryRow(ctx, `
		INSERT INTO tasks (title, description, kind, room_id, booking_id, schedule_id, assignee_id, due_at, created_by)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9) RETURNING id`,
		t.Title, t.Description, t.Kind, t.RoomID, t.BookingID, t.ScheduleID, t.AssigneeID, t.DueAt, createdBy).Scan(&id)
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	return s.TaskByID(ctx, id)
}

func (s *Store) UpdateTask(ctx context.Context, t domain.Task) (domain.Task, error) {
	tag, err := s.q.Exec(ctx, `
		UPDATE tasks SET title = $2, description = $3, kind = $4, room_id = $5, assignee_id = $6, due_at = $7,
		       status = $8, reminder_sent_at = CASE WHEN due_at <> $7 THEN NULL ELSE reminder_sent_at END,
		       updated_at = now()
		WHERE id = $1`,
		t.ID, t.Title, t.Description, t.Kind, t.RoomID, t.AssigneeID, t.DueAt, t.Status)
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Task{}, domain.ErrNotFound
	}
	return s.TaskByID(ctx, t.ID)
}

func (s *Store) SetTaskStatus(ctx context.Context, id uuid.UUID, status domain.TaskStatus, note string) (domain.Task, error) {
	tag, err := s.q.Exec(ctx, `
		UPDATE tasks SET status = $2::task_status,
		       completed_at = CASE WHEN $2::task_status = 'done' THEN now() ELSE NULL END,
		       completion_note = CASE WHEN $3::text <> '' THEN $3::text ELSE completion_note END,
		       updated_at = now()
		WHERE id = $1`, id, status, note)
	if err != nil {
		return domain.Task{}, mapErr(err)
	}
	if tag.RowsAffected() == 0 {
		return domain.Task{}, domain.ErrNotFound
	}
	return s.TaskByID(ctx, id)
}

func (s *Store) DeleteTask(ctx context.Context, id uuid.UUID) error {
	tag, err := s.q.Exec(ctx, `DELETE FROM tasks WHERE id = $1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return domain.ErrNotFound
	}
	return mapErr(err)
}

// DueForReminder returns open, assigned tasks starting before `until` that
// have not been reminded yet.
func (s *Store) DueForReminder(ctx context.Context, until time.Time) ([]domain.Task, error) {
	rows, err := s.q.Query(ctx, taskSelect+`
		WHERE t.status = 'open' AND t.reminder_sent_at IS NULL AND t.assignee_id IS NOT NULL
		  AND t.due_at <= $1 AND t.due_at > now() - interval '1 hour'
		ORDER BY t.due_at LIMIT 200`, until)
	return collect(rows, err, scanTask)
}

func (s *Store) MarkReminded(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.Exec(ctx, `UPDATE tasks SET reminder_sent_at = now() WHERE id = $1`, id)
	return mapErr(err)
}

// ── Schedules ────────────────────────────────────────────────

func scanSchedule(r pgx.Row) (domain.TaskSchedule, error) {
	var t domain.TaskSchedule
	err := r.Scan(&t.ID, &t.Title, &t.Kind, &t.RoomID, &t.AssigneeID, &t.Weekdays, &t.TimeOfDay,
		&t.DurationMinutes, &t.IsActive)
	return t, mapErr(err)
}

const scheduleCols = `id, title, kind, room_id, assignee_id, weekdays, to_char(time_of_day, 'HH24:MI'),
	duration_minutes, is_active`

func (s *Store) ListSchedules(ctx context.Context, onlyActive bool) ([]domain.TaskSchedule, error) {
	sql := `SELECT ` + scheduleCols + ` FROM task_schedules`
	if onlyActive {
		sql += ` WHERE is_active`
	}
	rows, err := s.q.Query(ctx, sql+` ORDER BY time_of_day`)
	return collect(rows, err, scanSchedule)
}

func (s *Store) SaveSchedule(ctx context.Context, t domain.TaskSchedule) (domain.TaskSchedule, error) {
	if t.ID == uuid.Nil {
		return scanSchedule(s.q.QueryRow(ctx, `
			INSERT INTO task_schedules (title, kind, room_id, assignee_id, weekdays, time_of_day, duration_minutes, is_active)
			VALUES ($1, $2, $3, $4, $5, $6::time, $7, $8) RETURNING `+scheduleCols,
			t.Title, t.Kind, t.RoomID, t.AssigneeID, t.Weekdays, t.TimeOfDay, t.DurationMinutes, t.IsActive))
	}
	return scanSchedule(s.q.QueryRow(ctx, `
		UPDATE task_schedules SET title = $2, kind = $3, room_id = $4, assignee_id = $5, weekdays = $6,
		       time_of_day = $7::time, duration_minutes = $8, is_active = $9
		WHERE id = $1 RETURNING `+scheduleCols,
		t.ID, t.Title, t.Kind, t.RoomID, t.AssigneeID, t.Weekdays, t.TimeOfDay, t.DurationMinutes, t.IsActive))
}

func (s *Store) DeleteSchedule(ctx context.Context, id uuid.UUID) error {
	_, err := s.q.Exec(ctx, `DELETE FROM task_schedules WHERE id = $1`, id)
	return mapErr(err)
}

// MaterializeTask inserts a task for a schedule occurrence, ignoring duplicates.
func (s *Store) MaterializeTask(ctx context.Context, sch domain.TaskSchedule, due time.Time) (bool, error) {
	tag, err := s.q.Exec(ctx, `
		INSERT INTO tasks (title, kind, room_id, schedule_id, assignee_id, due_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (schedule_id, due_at) WHERE schedule_id IS NOT NULL DO NOTHING`,
		sch.Title, sch.Kind, sch.RoomID, sch.ID, sch.AssigneeID, due)
	return tag.RowsAffected() > 0, mapErr(err)
}
