//go:build integration

package db_test

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/db"
	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// freshDatabase creates an empty database next to TEST_DATABASE_URL.
func freshDatabase(t *testing.T) *pgxpool.Pool {
	t.Helper()
	raw := os.Getenv("TEST_DATABASE_URL")
	if raw == "" {
		t.Skip("TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	admin, err := db.Connect(ctx, raw)
	if err != nil {
		t.Fatal(err)
	}
	name := fmt.Sprintf("crm_migtest_%d", time.Now().UnixNano())
	if _, err := admin.Exec(ctx, `CREATE DATABASE `+name); err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(raw)
	u.Path = "/" + name
	pool, err := db.Connect(ctx, u.String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		pool.Close()
		admin.Exec(context.Background(), `DROP DATABASE IF EXISTS `+name+` WITH (FORCE)`) //nolint:errcheck
		admin.Close()
	})
	return pool
}

// Accounts as they exist in production before the switch: Google sign-in,
// no username, no password, a few awkward e-mails.
func TestPasswordAuthMigrationKeepsExistingAccounts(t *testing.T) {
	ctx := context.Background()
	pool := freshDatabase(t)
	if err := db.MigrateTo(ctx, pool, "0005_tickets_contracts_recurring"); err != nil {
		t.Fatal(err)
	}
	type legacy struct {
		email, role, fb string
		active          bool
	}
	rows := []legacy{
		{"Nathan.Owner@Gmail.com", "super_admin", "fb-owner", true},
		{"nathan.owner@gmail.com", "user", "", true}, // same address, other case
		{"rakaz@matnas.org.il", "coordinator", "", false},
		{"avi@matnas.local", "staff", "", true},
		{"ab@x.co", "user", "", true},
		{"a+b.c@x.co", "user", "", true},
		{"_x_@y.co", "user", "", true},
		{"dana@a.co", "user", "", true},
		{"dana@b.co", "user", "", true},
	}
	ids := map[string]string{}
	for _, r := range rows {
		var fb *string
		if r.fb != "" {
			fb = &r.fb
		}
		var id string
		if err := pool.QueryRow(ctx, `INSERT INTO users (email, full_name, role, is_active, firebase_uid)
			VALUES ($1, 'x', $2, $3, $4) RETURNING id`, r.email, r.role, r.active, fb).Scan(&id); err != nil {
			t.Fatal(err)
		}
		ids[r.email] = id
	}
	var customer string
	if err := pool.QueryRow(ctx, `INSERT INTO customers (name, user_id) VALUES ('לקוח', $1) RETURNING id`, ids["dana@a.co"]).Scan(&customer); err != nil {
		t.Fatal(err)
	}

	if err := db.Migrate(ctx, pool); err != nil {
		t.Fatalf("migration on existing data: %v", err)
	}

	st := store.New(pool)
	users, err := st.ListUsers(ctx, store.UserFilter{})
	if err != nil || len(users) != len(rows) {
		t.Fatalf("accounts kept: %d %v", len(users), err)
	}
	seen := map[string]bool{}
	for _, u := range users {
		if !service.ValidUsername(u.Username) {
			t.Errorf("invalid username %q for %s", u.Username, u.Email)
		}
		if seen[u.Username] {
			t.Errorf("duplicate username %q", u.Username)
		}
		seen[u.Username] = true
		if u.HasPassword || u.LoginHistoryKnown || u.FirstLoginAt != nil {
			t.Errorf("%s: no password and unknown history expected: %+v", u.Email, u)
		}
		if ids[u.Email] != u.ID.String() {
			t.Errorf("%s: id changed", u.Email)
		}
		if u.IsActive != (u.Status == domain.AccountActive) {
			t.Errorf("%s: active accounts stay active, inactive ones become pending/disabled: %s", u.Email, u.Status)
		}
	}
	for _, want := range []string{"nathan.owner", "nathan.owner2", "rakaz", "avi", "userab", "ab.c", "userx_", "dana", "dana2"} {
		if !seen[want] {
			t.Errorf("expected username %q, got %v", want, seen)
		}
	}
	owner, _ := st.UserByUsername(ctx, "nathan.owner")
	if owner.Role != domain.RoleSuperAdmin || owner.FirebaseUID == nil || *owner.FirebaseUID != "fb-owner" {
		t.Fatalf("role and identifiers must be preserved: %+v", owner)
	}
	var linked string
	if err := pool.QueryRow(ctx, `SELECT user_id FROM customers WHERE id = $1`, customer).Scan(&linked); err != nil || linked != ids["dana@a.co"] {
		t.Fatalf("business data link lost: %v", err)
	}
	// Ambiguous e-mails never match a login by e-mail.
	if _, err := st.UserByEmail(ctx, "NATHAN.OWNER@gmail.com"); err == nil {
		t.Fatal("ambiguous e-mail must not resolve to one account")
	}

	// The pre-deployment report flags what the operator must fix.
	loc, _ := time.LoadLocation("Asia/Jerusalem")
	files, _ := storage.NewS3("", "", "", "", "")
	svc := service.New(&config.Config{Location: loc, JWTSecret: "x"}, st, nil, nil, files)
	rep, err := svc.AuthReport(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rep.Duplicates) != 1 || len(rep.Duplicates[0]) != 2 {
		t.Fatalf("duplicates: %+v", rep.Duplicates)
	}
	unusable := map[string]bool{}
	for _, u := range rep.Unusable {
		unusable[u.Email] = true
	}
	if !unusable["avi@matnas.local"] || unusable["dana@a.co"] {
		t.Fatalf("unusable: %v", unusable)
	}
	if len(rep.NoPassword) != 8 || len(rep.Pending) != 0 { // the inactive coordinator is "disabled"
		t.Fatalf("no password: %d, pending: %d", len(rep.NoPassword), len(rep.Pending))
	}
}
