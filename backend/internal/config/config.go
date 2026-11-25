// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"net/mail"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env      string
	AppName  string
	HTTPAddr string

	// Client branding used only to initialise an empty organization.
	OrgName       string
	OrgShortName  string
	OrgSubtitle   string
	OrgLogoURL    string
	OrgMarkURL    string
	OrgBrandColor string
	CORSOrigins   []string
	Location      *time.Location

	DatabaseURL string
	RedisURL    string

	AuthMode       string // "dev" (demo user picker) | "password" (password + e-mail code)
	DemoAccessCode string // optional: required by the dev login when set (public demo / staging)
	JWTSecret      string
	// AuthCodeSecret is the HMAC key protecting stored e-mail codes (never in the database).
	AuthCodeSecret string
	// PublicURL is the CRM's public origin, used to build links in e-mails.
	PublicURL string
	// BootstrapSuperAdminEmail receives the first invitation of a new instance.
	BootstrapSuperAdminEmail string
	Auth                     AuthPolicy

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string

	ReminderLead time.Duration
	SMTPHost     string
	SMTPPort     int
	SMTPTLS      string // ssl | starttls | none
	SMTPUser     string
	SMTPPassword string
	MailFrom     string

	// OpenSign (docs/opensign.md): enabled when OPENSIGN_PUBLIC_URL is set
	// and the master key written by the opensign-init service is readable.
	OpenSignURL       string // private address of OpenSign's proxy
	OpenSignPublicURL string // address signers open
	OpenSignKeyFile   string
	OpenSignEmail     string // the CRM's account inside OpenSign
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

// PasswordAuth reports whether logins use password + e-mail code.
func (c *Config) PasswordAuth() bool { return c.AuthMode == "password" }

// AuthPolicy holds the lifetimes and limits of the login flows.
type AuthPolicy struct {
	InviteTTL       time.Duration // invitation link
	ResetTTL        time.Duration // "forgot password" link
	CodeTTL         time.Duration // e-mail code
	ChallengeTTL    time.Duration // the whole "enter your code" step
	SessionTTL      time.Duration
	CodeMaxAttempts int           // wrong codes allowed per code
	ResendCooldown  time.Duration // between two e-mails of the same flow
	MaxSendsPerFlow int           // codes sent within one login attempt
	// Sliding window for the per-account and per-IP limits below.
	Window                  time.Duration
	MaxLoginFailsPerAccount int
	MaxLoginFailsPerIP      int
	MaxCodeFailsPerAccount  int
	MaxCodeFailsPerIP       int
	MaxSendsPerAccount      int
	MaxSendsPerIP           int
}

// DefaultAuthPolicy is used for anything not overridden by AUTH_* variables.
func DefaultAuthPolicy() AuthPolicy {
	return AuthPolicy{
		InviteTTL: 24 * time.Hour, ResetTTL: time.Hour, CodeTTL: 10 * time.Minute, ChallengeTTL: 30 * time.Minute,
		SessionTTL: 7 * 24 * time.Hour, CodeMaxAttempts: 5, ResendCooldown: 60 * time.Second, MaxSendsPerFlow: 5,
		Window: 15 * time.Minute, MaxLoginFailsPerAccount: 10, MaxLoginFailsPerIP: 50,
		MaxCodeFailsPerAccount: 10, MaxCodeFailsPerIP: 50, MaxSendsPerAccount: 8, MaxSendsPerIP: 40,
	}
}

func loadAuthPolicy() AuthPolicy {
	p := DefaultAuthPolicy()
	p.InviteTTL = getDuration("AUTH_INVITE_TTL", p.InviteTTL)
	p.ResetTTL = getDuration("AUTH_RESET_TTL", p.ResetTTL)
	p.CodeTTL = getDuration("AUTH_CODE_TTL", p.CodeTTL)
	p.ChallengeTTL = getDuration("AUTH_CHALLENGE_TTL", p.ChallengeTTL)
	p.SessionTTL = getDuration("AUTH_SESSION_TTL", p.SessionTTL)
	p.CodeMaxAttempts = getInt("AUTH_CODE_MAX_ATTEMPTS", p.CodeMaxAttempts)
	p.ResendCooldown = getDuration("AUTH_RESEND_COOLDOWN", p.ResendCooldown)
	p.MaxSendsPerFlow = getInt("AUTH_MAX_SENDS_PER_FLOW", p.MaxSendsPerFlow)
	p.Window = getDuration("AUTH_RATE_WINDOW", p.Window)
	p.MaxLoginFailsPerAccount = getInt("AUTH_MAX_LOGIN_FAILS_PER_ACCOUNT", p.MaxLoginFailsPerAccount)
	p.MaxLoginFailsPerIP = getInt("AUTH_MAX_LOGIN_FAILS_PER_IP", p.MaxLoginFailsPerIP)
	p.MaxCodeFailsPerAccount = getInt("AUTH_MAX_CODE_FAILS_PER_ACCOUNT", p.MaxCodeFailsPerAccount)
	p.MaxCodeFailsPerIP = getInt("AUTH_MAX_CODE_FAILS_PER_IP", p.MaxCodeFailsPerIP)
	p.MaxSendsPerAccount = getInt("AUTH_MAX_SENDS_PER_ACCOUNT", p.MaxSendsPerAccount)
	p.MaxSendsPerIP = getInt("AUTH_MAX_SENDS_PER_IP", p.MaxSendsPerIP)
	return p
}

func Load() (*Config, error) {
	tz := get("TZ", "Asia/Jerusalem")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid TZ %q: %w", tz, err)
	}

	c := &Config{
		Env:      get("APP_ENV", "development"),
		AppName:  get("APP_NAME", "קהילה+"),
		HTTPAddr: get("HTTP_ADDR", ":8080"),

		OrgName:       os.Getenv("ORG_NAME"),
		OrgShortName:  os.Getenv("ORG_SHORT_NAME"),
		OrgSubtitle:   os.Getenv("ORG_SUBTITLE"),
		OrgLogoURL:    os.Getenv("ORG_LOGO_URL"),
		OrgMarkURL:    os.Getenv("ORG_MARK_URL"),
		OrgBrandColor: os.Getenv("ORG_BRAND_COLOR"),
		CORSOrigins:   list("CORS_ORIGINS"),
		Location:      loc,

		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),

		AuthMode:                 get("AUTH_MODE", "dev"),
		DemoAccessCode:           os.Getenv("DEMO_ACCESS_CODE"),
		JWTSecret:                os.Getenv("JWT_SECRET"),
		AuthCodeSecret:           os.Getenv("AUTH_CODE_SECRET"),
		PublicURL:                strings.TrimRight(get("PUBLIC_URL", first(list("CORS_ORIGINS"))), "/"),
		BootstrapSuperAdminEmail: strings.ToLower(get("BOOTSTRAP_SUPER_ADMIN_EMAIL", "")),
		Auth:                     loadAuthPolicy(),

		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Bucket:    get("S3_BUCKET", "crm-files"),
		S3Region:    get("S3_REGION", "garage"),

		ReminderLead: time.Duration(getInt("REMINDER_LEAD_MINUTES", 60)) * time.Minute,
		SMTPHost:     os.Getenv("SMTP_HOST"),
		SMTPPort:     getInt("SMTP_PORT", 465),
		SMTPTLS:      strings.ToLower(os.Getenv("SMTP_TLS")),
		SMTPUser:     os.Getenv("SMTP_USER"),
		SMTPPassword: os.Getenv("SMTP_PASSWORD"),
		MailFrom:     get("MAIL_FROM", os.Getenv("SMTP_FROM")),

		OpenSignURL:       get("OPENSIGN_URL", "http://opensign"),
		OpenSignPublicURL: strings.TrimRight(os.Getenv("OPENSIGN_PUBLIC_URL"), "/"),
		OpenSignKeyFile:   get("OPENSIGN_MASTER_KEY_FILE", "/run/opensign/master_key"),
		OpenSignEmail:     strings.ToLower(os.Getenv("OPENSIGN_ACCOUNT_EMAIL")),
	}
	if c.OpenSignEmail == "" {
		c.OpenSignEmail = "crm@opensign.local"
		if a, err := mail.ParseAddress(c.MailFrom); err == nil {
			c.OpenSignEmail = strings.ToLower(a.Address)
		}
	}
	if c.SMTPTLS == "" {
		c.SMTPTLS = "starttls"
		if c.SMTPPort == 465 {
			c.SMTPTLS = "ssl"
		}
	}
	return c, c.validate()
}

func (c *Config) validate() error {
	var errs []error
	if c.DatabaseURL == "" {
		errs = append(errs, errors.New("DATABASE_URL is required"))
	}
	switch c.AuthMode {
	case "dev":
		if c.IsProduction() {
			errs = append(errs, errors.New("AUTH_MODE=dev is not allowed when APP_ENV=production"))
		}
	case "password":
		if c.SMTPHost == "" || c.MailFrom == "" {
			errs = append(errs, errors.New("SMTP_HOST and MAIL_FROM are required when AUTH_MODE=password"))
		}
		if len(c.AuthCodeSecret) < 32 {
			errs = append(errs, errors.New("AUTH_CODE_SECRET must be at least 32 characters when AUTH_MODE=password"))
		}
		if c.AuthCodeSecret != "" && c.AuthCodeSecret == c.JWTSecret {
			errs = append(errs, errors.New("AUTH_CODE_SECRET must differ from JWT_SECRET"))
		}
		if c.PublicURL == "" || (c.IsProduction() && !strings.HasPrefix(c.PublicURL, "https://")) {
			errs = append(errs, errors.New("PUBLIC_URL must be the CRM's https:// address when AUTH_MODE=password"))
		}
	case "firebase":
		errs = append(errs, errors.New("AUTH_MODE=firebase was removed: use AUTH_MODE=password (see docs/authentication.md)"))
	default:
		errs = append(errs, fmt.Errorf("unknown AUTH_MODE %q", c.AuthMode))
	}
	switch c.SMTPTLS {
	case "ssl", "starttls":
	case "none":
		if c.IsProduction() {
			errs = append(errs, errors.New("SMTP_TLS=none is not allowed when APP_ENV=production"))
		}
	default:
		errs = append(errs, fmt.Errorf("unknown SMTP_TLS %q (ssl | starttls | none)", c.SMTPTLS))
	}
	if len(c.JWTSecret) < 32 {
		errs = append(errs, errors.New("JWT_SECRET must be at least 32 characters"))
	}
	return errors.Join(errs...)
}

func get(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}

func getInt(key string, def int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(os.Getenv(key))); err == nil && v > 0 {
		return v
	}
	return def
}

func getDuration(key string, def time.Duration) time.Duration {
	if d, err := time.ParseDuration(strings.TrimSpace(os.Getenv(key))); err == nil && d > 0 {
		return d
	}
	return def
}

func first(l []string) string {
	if len(l) == 0 {
		return ""
	}
	return l[0]
}

func list(key string) []string {
	var out []string
	for _, p := range strings.Split(os.Getenv(key), ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
