package api

import (
	"net/http"

	"github.com/google/uuid"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// optionalID returns the {id} path param, or uuid.Nil on create routes.
func optionalID(r *http.Request) (uuid.UUID, error) {
	if r.Method == http.MethodPost {
		return uuid.Nil, nil
	}
	return pathID(r)
}

// saveHandler builds a POST/PUT handler that decodes T, sets its ID and saves it.
func saveHandler[T any](setID func(*T, uuid.UUID), save func(r *http.Request, a service.Actor, v T) (T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := optionalID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		var v T
		if err := decode(r, &v); err != nil {
			fail(w, r, err)
			return
		}
		setID(&v, id)
		saved, err := save(r, mustActor(r), v)
		if err != nil {
			fail(w, r, err)
			return
		}
		if id == uuid.Nil {
			created(w, saved)
		} else {
			ok(w, saved)
		}
	}
}

func listHandler[T any](list func(r *http.Request, a service.Actor) ([]T, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		v, err := list(r, mustActor(r))
		if err != nil {
			fail(w, r, err)
			return
		}
		ok(w, v)
	}
}

func deleteHandler(del func(r *http.Request, a service.Actor, id uuid.UUID) error) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id, err := pathID(r)
		if err != nil {
			fail(w, r, err)
			return
		}
		if err := del(r, mustActor(r), id); err != nil {
			fail(w, r, err)
			return
		}
		noContent(w)
	}
}

// requireAny lists data for any authenticated user holding one of the actions.
func requireAny(a service.Actor, acts ...domain.Action) error {
	for _, act := range acts {
		if a.Can(act) {
			return nil
		}
	}
	return domain.ErrForbidden
}

// ── Buildings / rooms / equipment / activities ───────────────

func (s *Server) listBuildings(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, _ service.Actor) ([]domain.Building, error) {
		return s.svc.Store().ListBuildings(r.Context())
	})(w, r)
}

func (s *Server) saveBuilding(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(b *domain.Building, id uuid.UUID) { b.ID = id },
		func(r *http.Request, a service.Actor, b domain.Building) (domain.Building, error) {
			return s.svc.SaveBuilding(r.Context(), a, b)
		})(w, r)
}

func (s *Server) listRooms(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Room, error) {
		// Everyone can browse rooms (portal search); inactive ones only for managers.
		return s.svc.Store().ListRooms(r.Context(), store.RoomFilter{
			BuildingID: qUUID(r, "buildingId"), OnlyActive: !a.Can(domain.ActManageRooms),
		})
	})(w, r)
}

func (s *Server) saveRoom(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(m *domain.Room, id uuid.UUID) { m.ID = id },
		func(r *http.Request, a service.Actor, m domain.Room) (domain.Room, error) {
			return s.svc.SaveRoom(r.Context(), a, m)
		})(w, r)
}

func (s *Server) listEquipment(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, _ service.Actor) ([]domain.EquipmentItem, error) {
		return s.svc.Store().ListEquipment(r.Context())
	})(w, r)
}

func (s *Server) saveEquipment(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(*domain.EquipmentItem, uuid.UUID) {},
		func(r *http.Request, a service.Actor, e domain.EquipmentItem) (domain.EquipmentItem, error) {
			return s.svc.SaveEquipment(r.Context(), a, e)
		})(w, r)
}

func (s *Server) listActivities(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Activity, error) {
		if err := requireAny(a, domain.ActViewCalendar, domain.ActRequestActivity); err != nil {
			return nil, err
		}
		list, err := s.svc.Store().ListActivities(r.Context())
		if err != nil || a.Can(domain.ActViewCalendar) {
			return list, err
		}
		// Coordinators only see the activities they run.
		mine := []domain.Activity{}
		for _, act := range list {
			if act.CoordinatorID != nil && *act.CoordinatorID == a.User.ID {
				mine = append(mine, act)
			}
		}
		return mine, nil
	})(w, r)
}

func (s *Server) saveActivity(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(v *domain.Activity, id uuid.UUID) { v.ID = id },
		func(r *http.Request, a service.Actor, v domain.Activity) (domain.Activity, error) {
			return s.svc.SaveActivity(r.Context(), a, v)
		})(w, r)
}

// ── Customers ────────────────────────────────────────────────

type customerBody struct {
	ID          uuid.UUID `json:"-"`
	Name        string    `json:"name"`
	Kind        string    `json:"kind"`
	ContactName *string   `json:"contactName"`
	Phone       *string   `json:"phone"`
	Email       *string   `json:"email"`
	TaxID       *string   `json:"taxId"`
	Notes       string    `json:"notes"`
}

func (s *Server) listCustomers(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Customer, error) {
		if err := requireAny(a, domain.ActManageCustomers, domain.ActViewPayments); err != nil {
			return nil, err
		}
		return s.svc.Store().ListCustomers(r.Context(), r.URL.Query().Get("q"))
	})(w, r)
}

func (s *Server) getCustomer(w http.ResponseWriter, r *http.Request) {
	id, err := pathID(r)
	if err != nil {
		fail(w, r, err)
		return
	}
	a := mustActor(r)
	if err := requireAny(a, domain.ActManageCustomers, domain.ActViewPayments); err != nil {
		fail(w, r, err)
		return
	}
	c, err := s.svc.Store().CustomerByID(r.Context(), id)
	if err != nil {
		fail(w, r, err)
		return
	}
	bookings, err := s.svc.Store().ListBookings(r.Context(), store.BookingFilter{CustomerID: &id})
	if err != nil {
		fail(w, r, err)
		return
	}
	payments, err := s.svc.Store().ListPayments(r.Context(), store.PaymentFilter{CustomerID: &id})
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"customer": c, "bookings": bookings, "payments": payments})
}

func (s *Server) saveCustomer(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(c *customerBody, id uuid.UUID) { c.ID = id },
		func(r *http.Request, a service.Actor, c customerBody) (customerBody, error) {
			saved, err := s.svc.SaveCustomer(r.Context(), a, c.ID, store.CustomerInput{
				Name: c.Name, Kind: c.Kind, ContactName: c.ContactName, Phone: c.Phone, Email: c.Email, TaxID: c.TaxID, Notes: c.Notes,
			})
			return customerBody{ID: saved.ID, Name: saved.Name, Kind: saved.Kind, ContactName: saved.ContactName,
				Phone: saved.Phone, Email: saved.Email, TaxID: saved.TaxID, Notes: saved.Notes}, err
		})(w, r)
}

func (s *Server) deleteCustomer(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		if err := a.Require(domain.ActManageCustomers); err != nil {
			return err
		}
		return s.svc.Store().DeleteCustomer(r.Context(), id)
	})(w, r)
}

// ── Users ────────────────────────────────────────────────────

func (s *Server) listUsers(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.User, error) {
		var active *bool
		if v := r.URL.Query().Get("active"); v != "" {
			b := v == "true"
			active = &b
		}
		return s.svc.ListUsers(r.Context(), a, store.UserFilter{Role: r.URL.Query().Get("role"), Search: r.URL.Query().Get("q"), Active: active})
	})(w, r)
}

type userBody struct {
	ID uuid.UUID `json:"id"`
	service.UserInput
}

func (s *Server) saveUser(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(u *userBody, id uuid.UUID) { u.ID = id },
		func(r *http.Request, a service.Actor, u userBody) (userBody, error) {
			saved, err := s.svc.SaveUser(r.Context(), a, u.ID, u.UserInput)
			return userBody{ID: saved.ID, UserInput: service.UserInput{Email: saved.Email, FullName: saved.FullName,
				Phone: saved.Phone, Role: saved.Role, StaffKind: saved.StaffKind, TelegramChatID: saved.TelegramChatID,
				IsActive: saved.IsActive}}, err
		})(w, r)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	a := mustActor(r)
	ok(w, map[string]any{"user": a.User, "permissions": domain.Permissions(a.User.Role)})
}

func (s *Server) updateMe(w http.ResponseWriter, r *http.Request) {
	var in service.ProfileInput
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	u, err := s.svc.UpdateProfile(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, map[string]any{"user": u, "permissions": domain.Permissions(u.Role)})
}

// ── Signage config ───────────────────────────────────────────

func (s *Server) listScreens(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.SignageScreen, error) {
		if err := a.Require(domain.ActManageSignage); err != nil {
			return nil, err
		}
		return s.svc.Store().ListScreens(r.Context())
	})(w, r)
}

func (s *Server) saveScreen(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(v *domain.SignageScreen, id uuid.UUID) { v.ID = id },
		func(r *http.Request, a service.Actor, v domain.SignageScreen) (domain.SignageScreen, error) {
			return s.svc.SaveScreen(r.Context(), a, v)
		})(w, r)
}

func (s *Server) deleteScreen(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		if err := a.Require(domain.ActManageSignage); err != nil {
			return err
		}
		return s.svc.Store().DeleteScreen(r.Context(), id)
	})(w, r)
}

func (s *Server) listAnnouncements(w http.ResponseWriter, r *http.Request) {
	listHandler(func(r *http.Request, a service.Actor) ([]domain.Announcement, error) {
		if err := a.Require(domain.ActManageSignage); err != nil {
			return nil, err
		}
		return s.svc.Store().ListAnnouncements(r.Context())
	})(w, r)
}

func (s *Server) saveAnnouncement(w http.ResponseWriter, r *http.Request) {
	saveHandler(func(v *domain.Announcement, id uuid.UUID) { v.ID = id },
		func(r *http.Request, a service.Actor, v domain.Announcement) (domain.Announcement, error) {
			return s.svc.SaveAnnouncement(r.Context(), a, v)
		})(w, r)
}

func (s *Server) deleteAnnouncement(w http.ResponseWriter, r *http.Request) {
	deleteHandler(func(r *http.Request, a service.Actor, id uuid.UUID) error {
		return s.svc.DeleteAnnouncement(r.Context(), a, id)
	})(w, r)
}

// ── Organization (client branding) ───────────────────────────

func (s *Server) saveOrganization(w http.ResponseWriter, r *http.Request) {
	var in domain.Organization
	if err := decode(r, &in); err != nil {
		fail(w, r, err)
		return
	}
	o, err := s.svc.SaveOrganization(r.Context(), mustActor(r), in)
	if err != nil {
		fail(w, r, err)
		return
	}
	ok(w, o)
}
