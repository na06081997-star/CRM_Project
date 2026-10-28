package domain

import "testing"

func TestCan(t *testing.T) {
	tests := []struct {
		role Role
		act  Action
		want bool
	}{
		{RoleAdmin, ActManageUsers, true},
		{RoleAdmin, ActManagePayments, true},
		{RoleCoordinator, ActReviewRequests, true},
		{RoleCoordinator, ActManagePayments, false},
		{RoleCoordinator, ActViewPayments, true},
		{RoleCoordinator, ActManageUsers, false},
		{RoleStaff, ActWorkTasks, true},
		{RoleStaff, ActManageBookings, false},
		{RoleUser, ActSubmitRequest, true},
		{RoleUser, ActViewDashboard, false},
		{Role("hacker"), ActSubmitRequest, false},
	}
	for _, tt := range tests {
		if got := Can(tt.role, tt.act); got != tt.want {
			t.Errorf("Can(%s, %s) = %v, want %v", tt.role, tt.act, got, tt.want)
		}
	}
}

func TestPermissionsListMatchesCan(t *testing.T) {
	for _, r := range []Role{RoleAdmin, RoleCoordinator, RoleStaff, RoleUser} {
		for _, a := range Permissions(r) {
			if !Can(r, a) {
				t.Errorf("%s lists %s but Can is false", r, a)
			}
		}
	}
	if len(Permissions(RoleUser)) != 1 {
		t.Errorf("user should have exactly one permission, got %v", Permissions(RoleUser))
	}
}
