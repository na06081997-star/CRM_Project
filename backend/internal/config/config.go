// Package config loads runtime configuration from environment variables.
package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Env         string
	AppName     string
	HTTPAddr    string
	CORSOrigins []string
	Location    *time.Location

	DatabaseURL string
	RedisURL    string

	AuthMode             string // "dev" | "firebase"
	JWTSecret            string
	FirebaseProjectID    string
	FirebaseAPIKey       string
	FirebaseAuthDomain   string
	FirebaseAppID        string
	BootstrapAdminEmails []string

	S3Endpoint  string
	S3AccessKey string
	S3SecretKey string
	S3Bucket    string
	S3Region    string

	NotifyChannels   []string
	ReminderLead     time.Duration
	TelegramBotToken string
	SMTPHost         string
	SMTPPort         int
	SMTPUser         string
	SMTPPassword     string
	SMTPFrom         string
	TwilioAccountSID string
	TwilioAuthToken  string
	TwilioFrom       string
	SMS019Username   string
	SMS019Token      string
	SMS019Source     string
}

func (c *Config) IsProduction() bool { return c.Env == "production" }

func Load() (*Config, error) {
	tz := get("TZ", "Asia/Jerusalem")
	loc, err := time.LoadLocation(tz)
	if err != nil {
		return nil, fmt.Errorf("invalid TZ %q: %w", tz, err)
	}

	c := &Config{
		Env:         get("APP_ENV", "development"),
		AppName:     get("APP_NAME", "קהילה+"),
		HTTPAddr:    get("HTTP_ADDR", ":8080"),
		CORSOrigins: list("CORS_ORIGINS"),
		Location:    loc,

		DatabaseURL: os.Getenv("DATABASE_URL"),
		RedisURL:    os.Getenv("REDIS_URL"),

		AuthMode:             get("AUTH_MODE", "dev"),
		JWTSecret:            os.Getenv("JWT_SECRET"),
		FirebaseProjectID:    os.Getenv("FIREBASE_PROJECT_ID"),
		FirebaseAPIKey:       os.Getenv("FIREBASE_API_KEY"),
		FirebaseAuthDomain:   os.Getenv("FIREBASE_AUTH_DOMAIN"),
		FirebaseAppID:        os.Getenv("FIREBASE_APP_ID"),
		BootstrapAdminEmails: list("BOOTSTRAP_ADMIN_EMAILS"),

		S3Endpoint:  os.Getenv("S3_ENDPOINT"),
		S3AccessKey: os.Getenv("S3_ACCESS_KEY"),
		S3SecretKey: os.Getenv("S3_SECRET_KEY"),
		S3Bucket:    get("S3_BUCKET", "crm-files"),
		S3Region:    get("S3_REGION", "garage"),

		NotifyChannels:   list("NOTIFY_CHANNELS"),
		ReminderLead:     time.Duration(getInt("REMINDER_LEAD_MINUTES", 60)) * time.Minute,
		TelegramBotToken: os.Getenv("TELEGRAM_BOT_TOKEN"),
		SMTPHost:         os.Getenv("SMTP_HOST"),
		SMTPPort:         getInt("SMTP_PORT", 587),
		SMTPUser:         os.Getenv("SMTP_USER"),
		SMTPPassword:     os.Getenv("SMTP_PASSWORD"),
		SMTPFrom:         os.Getenv("SMTP_FROM"),
		TwilioAccountSID: os.Getenv("TWILIO_ACCOUNT_SID"),
		TwilioAuthToken:  os.Getenv("TWILIO_AUTH_TOKEN"),
		TwilioFrom:       os.Getenv("TWILIO_FROM"),
		SMS019Username:   os.Getenv("SMS019_USERNAME"),
		SMS019Token:      os.Getenv("SMS019_TOKEN"),
		SMS019Source:     os.Getenv("SMS019_SOURCE"),
	}
	if len(c.NotifyChannels) == 0 {
		c.NotifyChannels = []string{"console"}
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
	case "firebase":
		if c.FirebaseProjectID == "" {
			errs = append(errs, errors.New("FIREBASE_PROJECT_ID is required when AUTH_MODE=firebase"))
		}
	default:
		errs = append(errs, fmt.Errorf("unknown AUTH_MODE %q", c.AuthMode))
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
	if v, err := strconv.Atoi(os.Getenv(key)); err == nil {
		return v
	}
	return def
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
