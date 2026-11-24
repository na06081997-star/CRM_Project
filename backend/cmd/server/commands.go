package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/na06081997-star/CRM_Project/backend/internal/domain"
	"github.com/na06081997-star/CRM_Project/backend/internal/mailer"
)

const usage = `Operator commands (run inside the backend container, e.g.
  docker exec -it <backend-container> /app/server <command> [flags]):

  bootstrap-resend      e-mail a new invitation to the initial super administrator,
                        while that account is still pending (SMTP refused the first one…)
  auth-report           list accounts that need attention before / after the switch to
                        password + e-mail code login (no deliverable e-mail, duplicates…)
  recover-super-admin   the only super administrator lost their mailbox:
                        -user <username|e-mail> -email <new address> -yes
                        sets the new address, clears the password, ends every session and
                        e-mails a new invitation to the new address
  mail-test -to <addr>  send a test e-mail with the configured SMTP settings

Links and codes are never printed.`

func command(name string, args []string) int {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	switch name {
	case "help", "-h", "--help":
		fmt.Println(usage)
		return 0
	case "bootstrap-resend":
		return withApp(ctx, func(a *app) error {
			u, inv, err := a.svc.ResendBootstrapInvitation(ctx)
			if err != nil {
				return err
			}
			return printInvitation(u, inv)
		})
	case "auth-report":
		return withApp(ctx, func(a *app) error { return authReport(ctx, a) })
	case "recover-super-admin":
		fs := flag.NewFlagSet(name, flag.ContinueOnError)
		who := fs.String("user", "", "username or current e-mail of the super administrator")
		email := fs.String("email", "", "new e-mail address (a mailbox they control)")
		yes := fs.Bool("yes", false, "confirm: the current password and every session are revoked")
		if err := fs.Parse(args); err != nil {
			return 2
		}
		if *who == "" || *email == "" || !*yes {
			fmt.Fprintln(os.Stderr, "usage: server recover-super-admin -user <username|e-mail> -email <new address> -yes")
			return 2
		}
		return withApp(ctx, func(a *app) error {
			u, inv, err := a.svc.RecoverSuperAdmin(ctx, *who, *email)
			if err != nil {
				return err
			}
			fmt.Printf("Account %s now uses %s. Password cleared, sessions revoked.\n", u.Username, u.Email)
			return printInvitation(u, inv)
		})
	case "mail-test":
		fs := flag.NewFlagSet(name, flag.ContinueOnError)
		to := fs.String("to", "", "recipient")
		if err := fs.Parse(args); err != nil || *to == "" {
			fmt.Fprintln(os.Stderr, "usage: server mail-test -to <address>")
			return 2
		}
		return withApp(ctx, func(a *app) error {
			if a.cfg.SMTPHost == "" {
				return fmt.Errorf("SMTP_HOST is not set")
			}
			m, err := mailer.NewSMTP(mailer.Config{Host: a.cfg.SMTPHost, Port: a.cfg.SMTPPort, TLS: a.cfg.SMTPTLS,
				Username: a.cfg.SMTPUser, Password: a.cfg.SMTPPassword, From: a.cfg.MailFrom})
			if err != nil {
				return err
			}
			b := a.svc.MailBrand(ctx)
			msg := mailer.Notice(b, *to, "בדיקת שליחת אימייל – "+b.AppName,
				"זהו אימייל בדיקה. אם קיבלתם אותו, הגדרות השליחה של המערכת תקינות.")
			if err := m.Send(ctx, msg); err != nil {
				return fmt.Errorf("not sent (%s:%d, %s): %w", a.cfg.SMTPHost, a.cfg.SMTPPort, a.cfg.SMTPTLS, err)
			}
			fmt.Printf("Sent to %s via %s:%d (%s) as %s\n", *to, a.cfg.SMTPHost, a.cfg.SMTPPort, a.cfg.SMTPTLS, a.cfg.MailFrom)
			return nil
		})
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n\n%s\n", name, usage)
		return 2
	}
}

func withApp(ctx context.Context, fn func(*app) error) int {
	a, err := open(ctx, nil)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	defer a.close()
	if err := fn(a); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		return 1
	}
	return 0
}

func printInvitation(u domain.User, inv domain.Invitation) error {
	if inv.Status == "failed" {
		reason := ""
		if inv.DeliveryError != nil {
			reason = *inv.DeliveryError
		}
		return fmt.Errorf("invitation for %s (%s) was NOT sent: %s – fix SMTP and run the command again", u.Username, u.Email, reason)
	}
	fmt.Printf("Invitation e-mailed to %s (username %s), valid until %s.\n", u.Email, u.Username, inv.ExpiresAt.Format(time.RFC3339))
	return nil
}

func authReport(ctx context.Context, a *app) error {
	r, err := a.svc.AuthReport(ctx)
	if err != nil {
		return err
	}
	line := func(u domain.User) string {
		return fmt.Sprintf("  - %-24s %-32s %-12s %s", u.Username, u.Email, u.Role, u.Status)
	}
	section := func(title, fix string, users []domain.User) {
		fmt.Printf("\n%s: %d\n", title, len(users))
		for _, u := range users {
			fmt.Println(line(u))
		}
		if len(users) > 0 && fix != "" {
			fmt.Println("  fix:", fix)
		}
	}
	fmt.Printf("Auth mode: %s · public URL: %s · sender: %s\n", a.cfg.AuthMode, a.cfg.PublicURL, a.cfg.MailFrom)
	section("Accounts without a deliverable e-mail (cannot receive codes)",
		"users page → edit the account's e-mail, then \"send invitation\"", r.Unusable)
	fmt.Printf("\nE-mails shared by several accounts (login by e-mail refused, username still works): %d\n", len(r.Duplicates))
	for _, g := range r.Duplicates {
		var names []string
		for _, u := range g {
			names = append(names, u.Username+" <"+u.Email+">")
		}
		fmt.Println("  - " + strings.Join(names, " | "))
	}
	if len(r.Duplicates) > 0 {
		fmt.Println("  fix: give each account its own address (users page), or disable the duplicate")
	}
	section("Active accounts without a password",
		"users page → \"send invitation\" (or the user clicks \"forgot password\")", r.NoPassword)
	section("Pending invitations", "", r.Pending)
	return nil
}
