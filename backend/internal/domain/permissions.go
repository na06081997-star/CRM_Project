package domain

type Action string

const (
	ActViewDashboard    Action = "dashboard:view"
	ActViewCalendar     Action = "calendar:view" // full calendar with every detail
	ActViewAvailability Action = "availability:view"
	ActManageBookings   Action = "bookings:manage"
	ActReviewRequests   Action = "requests:review"
	ActSubmitRequest    Action = "requests:submit"
	ActRequestActivity  Action = "requests:activity" // "פעילות באמצעות רכז" route
	ActRequestRental    Action = "requests:rental"   // paid rental (not for coordinators)
	ActReportIssue      Action = "issues:report"     // open an operational ticket (broken item, supplies, cleaning)
	ActManageRooms      Action = "rooms:manage"
	ActManagePayments   Action = "payments:manage"
	ActViewPayments     Action = "payments:view"
	ActManageCustomers  Action = "customers:manage"
	ActManageUsers      Action = "users:manage"
	ActManageCoords     Action = "coordinators:manage" // add coordinators and send their contracts
	ActManageSettings   Action = "settings:manage"
	ActManageTasks      Action = "tasks:manage"
	ActWorkTasks        Action = "tasks:work"
	ActManageSignage    Action = "signage:manage"
	ActManageActivity   Action = "activities:manage"
	ActViewReports      Action = "reports:view"
	ActViewAudit        Action = "audit:view"
	ActUploadFiles      Action = "files:upload"
)

var allActions = []Action{
	ActViewDashboard, ActViewCalendar, ActViewAvailability, ActManageBookings, ActReviewRequests,
	ActSubmitRequest, ActRequestActivity, ActRequestRental, ActReportIssue, ActManageRooms, ActManagePayments,
	ActViewPayments, ActManageCustomers, ActManageUsers, ActManageCoords, ActManageSettings, ActManageTasks, ActWorkTasks,
	ActManageSignage, ActManageActivity, ActViewReports, ActViewAudit, ActUploadFiles,
}

// except returns every action but the given ones.
func except(skip ...Action) map[Action]bool {
	m := set(allActions...)
	for _, a := range skip {
		delete(m, a)
	}
	return m
}

var rolePermissions = map[Role]map[Action]bool{
	// Owner of the system: everything, including users, permissions and settings.
	RoleSuperAdmin: nil,
	// מנהל: runs the matnas day to day — approvals, every coordinator's
	// requests, payments, the global calendar — but not users or system settings.
	RoleAdmin: except(ActManageUsers, ActManageSettings),
	// רכז: requests slots for their activities from what the availability
	// calendar shows (never paid rentals); sees only their own requests and
	// bookings; reports operational issues.
	RoleCoordinator: set(ActViewAvailability, ActSubmitRequest, ActRequestActivity, ActReportIssue, ActUploadFiles),
	RoleStaff:       set(ActWorkTasks, ActViewAvailability),
	RoleUser:        set(ActSubmitRequest, ActRequestRental, ActViewAvailability),
}

// Can reports whether a role is allowed to perform an action.
func Can(r Role, a Action) bool {
	perms, ok := rolePermissions[r]
	if !ok {
		return false
	}
	if perms == nil {
		return true
	}
	return perms[a]
}

// Permissions lists the actions granted to a role (for the frontend).
func Permissions(r Role) []Action {
	var out []Action
	for _, a := range allActions {
		if Can(r, a) {
			out = append(out, a)
		}
	}
	return out
}

func set(actions ...Action) map[Action]bool {
	m := make(map[Action]bool, len(actions))
	for _, a := range actions {
		m[a] = true
	}
	return m
}
