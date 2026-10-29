package domain

import "testing"

func TestCan(t *testing.T) {
	tests := []struct {
		role Role
		act  Action
		want bool
	}{
		{RoleSuperAdmin, ActManageUsers, true},
		{RoleSuperAdmin, ActManageSettings, true},
		{RoleAdmin, ActReviewRequests, true},
		{RoleAdmin, ActManagePayments, true},
		{RoleAdmin, ActViewCalendar, true},
		{RoleAdmin, ActManageUsers, false},
		{RoleAdmin, ActManageSettings, false},
		{RoleCoordinator, ActViewAvailability, true},
		{RoleCoordinator, ActRequestActivity, true},
		{RoleCoordinator, ActViewCalendar, false},
		{RoleCoordinator, ActReviewRequests, false},
		{RoleCoordinator, ActViewPayments, false},
		{RoleCoordinator, ActViewDashboard, false},
		{RoleStaff, ActWorkTasks, true},
		{RoleStaff, ActManageBookings, false},
		{RoleUser, ActSubmitRequest, true},
		{RoleUser, ActRequestActivity, false},
		{RoleUser, ActViewAvailability, false},
		{Role("hacker"), ActSubmitRequest, false},
	}
	for _, tt := range tests {
		if got := Can(tt.role, tt.act); got != tt.want {
			t.Errorf("Can(%s, %s) = %v, want %v", tt.role, tt.act, got, tt.want)
		}
	}
}

func TestPermissionsListMatchesCan(t *testing.T) {
	for _, r := range []Role{RoleSuperAdmin, RoleAdmin, RoleCoordinator, RoleStaff, RoleUser} {
		for _, a := range Permissions(r) {
			if !Can(r, a) {
				t.Errorf("%s lists %s but Can is false", r, a)
			}
		}
	}
	if len(Permissions(RoleSuperAdmin)) != len(allActions) {
		t.Error("super admin must hold every permission")
	}
	if len(Permissions(RoleUser)) != 1 {
		t.Errorf("user should have exactly one permission, got %v", Permissions(RoleUser))
	}
}
