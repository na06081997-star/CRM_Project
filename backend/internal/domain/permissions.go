package domain

type Action string

const (
	ActViewDashboard   Action = "dashboard:view"
	ActViewCalendar    Action = "calendar:view"
	ActManageBookings  Action = "bookings:manage"
	ActReviewRequests  Action = "requests:review"
	ActSubmitRequest   Action = "requests:submit"
	ActManageRooms     Action = "rooms:manage"
	ActManagePayments  Action = "payments:manage"
	ActViewPayments    Action = "payments:view"
	ActManageCustomers Action = "customers:manage"
	ActManageUsers     Action = "users:manage"
	ActManageTasks     Action = "tasks:manage"
	ActWorkTasks       Action = "tasks:work"
	ActManageSignage   Action = "signage:manage"
	ActManageActivity  Action = "activities:manage"
	ActViewReports     Action = "reports:view"
	ActViewAudit       Action = "audit:view"
	ActUploadFiles     Action = "files:upload"
)

var rolePermissions = map[Role]map[Action]bool{
	RoleAdmin: nil, // admins can do everything
	RoleCoordinator: set(
		ActViewDashboard, ActViewCalendar, ActManageBookings, ActReviewRequests,
		ActManageCustomers, ActManageActivity, ActManageTasks, ActViewPayments,
		ActSubmitRequest, ActUploadFiles, ActViewReports,
	),
	RoleStaff: set(ActWorkTasks, ActViewCalendar, ActSubmitRequest),
	RoleUser:  set(ActSubmitRequest),
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
	all := []Action{
		ActViewDashboard, ActViewCalendar, ActManageBookings, ActReviewRequests, ActSubmitRequest,
		ActManageRooms, ActManagePayments, ActViewPayments, ActManageCustomers, ActManageUsers,
		ActManageTasks, ActWorkTasks, ActManageSignage, ActManageActivity, ActViewReports,
		ActViewAudit, ActUploadFiles,
	}
	var out []Action
	for _, a := range all {
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
