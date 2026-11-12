package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

func (s *Service) ListTasks(ctx context.Context, actor Actor, f store.TaskFilter) ([]domain.Task, error) {
	uid := actor.User.ID
	switch {
	case actor.Can(domain.ActManageTasks):
	case actor.Can(domain.ActWorkTasks):
		f.AssigneeID, f.ReportedBy, f.Unassigned = &uid, nil, false
	case actor.Can(domain.ActReportIssue):
		// Coordinators follow the tickets they opened.
		f.ReportedBy, f.AssigneeID, f.Unassigned = &uid, nil, false
	default:
		return nil, domain.ErrForbidden
	}
	return s.store.ListTasks(ctx, f)
}

// IssueInput is an operational ticket: something broken, supplies (toner…),
// a cleaning or maintenance need.
type IssueInput struct {
	Kind        string     `json:"kind"` // repair | supplies | cleaning | other
	Title       string     `json:"title"`
	Description string     `json:"description"`
	RoomID      *uuid.UUID `json:"roomId"`
}

var issueTitles = map[string]string{"repair": "תקלה לתיקון", "supplies": "בקשת ציוד מתכלה", "cleaning": "בקשת ניקיון", "other": "פנייה לצוות התפעול"}

// ReportIssue opens an unassigned ticket; a manager assigns it to a worker.
func (s *Service) ReportIssue(ctx context.Context, actor Actor, in IssueInput) (domain.Task, error) {
	if err := actor.Require(domain.ActReportIssue); err != nil {
		return domain.Task{}, err
	}
	in.Title, in.Description = strings.TrimSpace(in.Title), strings.TrimSpace(in.Description)
	v := domain.NewValidation()
	def, ok := issueTitles[in.Kind]
	if !ok {
		v.Add("kind", "יש לבחור סוג פנייה")
	}
	if in.Title == "" {
		in.Title = def
	}
	if in.Description == "" && in.Title == def {
		v.Add("description", "יש לתאר את הבעיה או הבקשה")
	}
	if len([]rune(in.Title)) > 120 {
		v.Add("title", "כותרת ארוכה מדי")
	}
	if in.RoomID != nil {
		if _, err := s.store.RoomByID(ctx, *in.RoomID); err != nil {
			v.Add("roomId", "חדר לא נמצא")
		}
	}
	if err := v.OrNil(); err != nil {
		return domain.Task{}, err
	}
	t, err := s.store.CreateTask(ctx, domain.Task{
		Title: in.Title, Description: in.Description, Kind: in.Kind, RoomID: in.RoomID,
		DueAt: s.now(), ReportedBy: &actor.User.ID,
	}, actor.ID())
	if err != nil {
		return t, err
	}
	s.audit(ctx, actor, "task", t.ID, "report", map[string]any{"title": t.Title, "kind": t.Kind})
	s.taskChanged(ctx, t)
	where := ""
	if t.RoomName != nil {
		where = " – " + *t.RoomName
	}
	s.notifyManagers(ctx, "פנייה חדשה לצוות התפעול", fmt.Sprintf("%s פתח/ה פנייה: %s%s\nיש לשבץ אותה לאיש צוות במסך משימות תפעול.", actor.User.FullName, t.Title, where))
	return t, nil
}

// AssignTask gives a task (typically a coordinator's ticket) to a worker.
func (s *Service) AssignTask(ctx context.Context, actor Actor, id uuid.UUID, assignee *uuid.UUID) (domain.Task, error) {
	if err := actor.Require(domain.ActManageTasks); err != nil {
		return domain.Task{}, err
	}
	var worker domain.User
	if assignee != nil {
		var err error
		worker, err = s.store.UserByID(ctx, *assignee)
		if err != nil || !worker.IsActive || worker.Role != domain.RoleStaff {
			v := domain.NewValidation()
			v.Add("assigneeId", "יש לבחור איש צוות פעיל")
			return domain.Task{}, v
		}
	}
	t, err := s.store.AssignTask(ctx, id, assignee)
	if err != nil {
		return t, err
	}
	s.audit(ctx, actor, "task", id, "assign", map[string]any{"title": t.Title, "assignee": t.AssigneeName})
	s.taskChanged(ctx, t)
	if assignee != nil {
		msg := fmt.Sprintf("שובצה לך משימה: %s", t.Title)
		if t.RoomName != nil {
			msg += "\nמיקום: " + *t.RoomName
		}
		if t.Description != "" {
			msg += "\n" + t.Description
		}
		if t.ReporterName != nil {
			msg += "\nפנה/תה: " + *t.ReporterName
			if t.ReporterPhone != nil {
				msg += " · " + *t.ReporterPhone
			}
		}
		s.notifyUser(worker, "משימה חדשה", msg)
	}
	return t, nil
}

// notifyManagers messages every active admin.
func (s *Service) notifyManagers(ctx context.Context, subject, body string) {
	for _, role := range []string{string(domain.RoleAdmin), string(domain.RoleSuperAdmin)} {
		users, err := s.store.ListUsers(ctx, store.UserFilter{Role: role})
		if err != nil {
			continue
		}
		for _, u := range users {
			if u.IsActive {
				s.notifyUser(u, subject, body)
			}
		}
	}
}

func (s *Service) SaveTask(ctx context.Context, actor Actor, t domain.Task) (domain.Task, error) {
	if err := actor.Require(domain.ActManageTasks); err != nil {
		return domain.Task{}, err
	}
	t.Title = strings.TrimSpace(t.Title)
	v := domain.NewValidation()
	if t.Title == "" {
		v.Add("title", "יש להזין כותרת")
	}
	if t.DueAt.IsZero() {
		v.Add("dueAt", "יש לבחור מועד")
	}
	if t.Kind == "" {
		t.Kind = "other"
	}
	if t.Status == "" {
		t.Status = domain.TaskOpen
	}
	if err := v.OrNil(); err != nil {
		return domain.Task{}, err
	}
	var saved domain.Task
	var err error
	var before *uuid.UUID
	if t.ID == uuid.Nil {
		saved, err = s.store.CreateTask(ctx, t, actor.ID())
	} else {
		if cur, err := s.store.TaskByID(ctx, t.ID); err == nil {
			before = cur.AssigneeID
		}
		saved, err = s.store.UpdateTask(ctx, t)
	}
	if err != nil {
		return saved, err
	}
	s.audit(ctx, actor, "task", saved.ID, "save", map[string]any{"title": saved.Title, "assignee": saved.AssigneeName})
	s.taskChanged(ctx, saved)
	newAssignee := saved.AssigneeID != nil && (before == nil || *before != *saved.AssigneeID)
	if newAssignee {
		if u, err := s.store.UserByID(ctx, *saved.AssigneeID); err == nil {
			s.notifyUser(u, "משימה חדשה", fmt.Sprintf("שובצה לך משימה: %s\nמועד: %s", saved.Title, saved.DueAt.In(s.loc).Format("02/01 15:04")))
		}
	}
	return saved, nil
}

// UpdateTaskStatus lets staff move their own tasks (e.g. mark "בוצע").
func (s *Service) UpdateTaskStatus(ctx context.Context, actor Actor, id uuid.UUID, status domain.TaskStatus, note string) (domain.Task, error) {
	t, err := s.store.TaskByID(ctx, id)
	if err != nil {
		return t, err
	}
	own := t.AssigneeID != nil && *t.AssigneeID == actor.User.ID && actor.Can(domain.ActWorkTasks)
	if !own && !actor.Can(domain.ActManageTasks) {
		return t, domain.ErrForbidden
	}
	switch status {
	case domain.TaskOpen, domain.TaskInProgress, domain.TaskDone, domain.TaskCancelled:
	default:
		v := domain.NewValidation()
		v.Add("status", "סטטוס לא תקין")
		return t, v
	}
	t, err = s.store.SetTaskStatus(ctx, id, status, strings.TrimSpace(note))
	if err != nil {
		return t, err
	}
	s.audit(ctx, actor, "task", id, "status:"+string(status), map[string]any{"title": t.Title, "note": note})
	s.taskChanged(ctx, t)
	if status == domain.TaskDone && t.ReportedBy != nil && *t.ReportedBy != actor.User.ID {
		if u, err := s.store.UserByID(ctx, *t.ReportedBy); err == nil {
			msg := fmt.Sprintf("הפנייה \"%s\" טופלה.", t.Title)
			if t.CompletionNote != "" {
				msg += "\n" + t.CompletionNote
			}
			s.notifyUser(u, "הפנייה שלך טופלה ✅", msg)
		}
	}
	return t, nil
}

func (s *Service) DeleteTask(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := actor.Require(domain.ActManageTasks); err != nil {
		return err
	}
	if err := s.store.DeleteTask(ctx, id); err != nil {
		return err
	}
	s.audit(ctx, actor, "task", id, "delete", nil)
	s.emit(ctx, EvTaskChanged, map[string]any{"id": id, "action": "deleted"}, realtime.TopicStaff)
	return nil
}

func (s *Service) taskChanged(ctx context.Context, t domain.Task) {
	topics := []string{realtime.TopicStaff}
	if t.AssigneeID != nil {
		topics = append(topics, realtime.UserTopic(t.AssigneeID.String()))
	}
	if t.ReportedBy != nil {
		topics = append(topics, realtime.UserTopic(t.ReportedBy.String()))
	}
	s.emit(ctx, EvTaskChanged, map[string]any{"id": t.ID, "status": t.Status}, topics...)
}

func (s *Service) ListSchedules(ctx context.Context, actor Actor) ([]domain.TaskSchedule, error) {
	if err := actor.Require(domain.ActManageTasks); err != nil {
		return nil, err
	}
	return s.store.ListSchedules(ctx, false)
}

func (s *Service) SaveSchedule(ctx context.Context, actor Actor, t domain.TaskSchedule) (domain.TaskSchedule, error) {
	if err := actor.Require(domain.ActManageTasks); err != nil {
		return t, err
	}
	v := domain.NewValidation()
	if strings.TrimSpace(t.Title) == "" {
		v.Add("title", "יש להזין כותרת")
	}
	if _, err := time.Parse("15:04", t.TimeOfDay); err != nil {
		v.Add("timeOfDay", "שעה לא תקינה")
	}
	if len(t.Weekdays) == 0 {
		v.Add("weekdays", "יש לבחור לפחות יום אחד")
	}
	if t.DurationMinutes <= 0 {
		t.DurationMinutes = 60
	}
	if t.Kind == "" {
		t.Kind = "cleaning"
	}
	if err := v.OrNil(); err != nil {
		return t, err
	}
	saved, err := s.store.SaveSchedule(ctx, t)
	if err != nil {
		return saved, err
	}
	s.audit(ctx, actor, "task_schedule", saved.ID, "save", map[string]any{"title": saved.Title})
	go s.MaterializeSchedules(context.Background())
	return saved, nil
}

func (s *Service) DeleteSchedule(ctx context.Context, actor Actor, id uuid.UUID) error {
	if err := actor.Require(domain.ActManageTasks); err != nil {
		return err
	}
	return s.store.DeleteSchedule(ctx, id)
}

// MaterializeSchedules creates concrete tasks for the next 7 days.
func (s *Service) MaterializeSchedules(ctx context.Context) {
	schedules, err := s.store.ListSchedules(ctx, true)
	if err != nil {
		slog.Error("scheduler: list schedules", "err", err)
		return
	}
	from := s.now()
	to := s.startOfDay(from).AddDate(0, 0, 8)
	created := 0
	for _, sch := range schedules {
		for _, due := range domain.Occurrences(sch, from, to, s.loc) {
			ok, err := s.store.MaterializeTask(ctx, sch, due)
			if err != nil {
				slog.Error("scheduler: materialize", "schedule", sch.ID, "err", err)
				continue
			}
			if ok {
				created++
			}
		}
	}
	if created > 0 {
		slog.Info("scheduler: tasks created", "count", created)
		s.emit(ctx, EvTaskChanged, map[string]any{"action": "generated"}, realtime.TopicStaff)
	}
}

// SendDueReminders notifies assignees of tasks starting within the lead time.
func (s *Service) SendDueReminders(ctx context.Context) {
	tasks, err := s.store.DueForReminder(ctx, s.now().Add(s.cfg.ReminderLead))
	if err != nil {
		slog.Error("scheduler: due reminders", "err", err)
		return
	}
	for _, t := range tasks {
		u, err := s.store.UserByID(ctx, *t.AssigneeID)
		if err != nil {
			continue
		}
		where := ""
		if t.RoomName != nil {
			where = " – " + *t.RoomName
		}
		s.notifyUser(u, "תזכורת משימה",
			fmt.Sprintf("שלום %s, תזכורת: %s%s בשעה %s.\nבסיום יש לסמן \"בוצע\" במערכת.", u.FullName, t.Title, where, t.DueAt.In(s.loc).Format("15:04")))
		if err := s.store.MarkReminded(ctx, t.ID); err != nil {
			slog.Error("scheduler: mark reminded", "task", t.ID, "err", err)
		}
	}
}
