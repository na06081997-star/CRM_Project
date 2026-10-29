// Command server runs the CRM API, WebSocket hub and background scheduler.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/na06081997-star/CRM_Project/backend/internal/api"
	"github.com/na06081997-star/CRM_Project/backend/internal/auth"
	"github.com/na06081997-star/CRM_Project/backend/internal/config"
	"github.com/na06081997-star/CRM_Project/backend/internal/db"
	"github.com/na06081997-star/CRM_Project/backend/internal/notify"
	"github.com/na06081997-star/CRM_Project/backend/internal/realtime"
	"github.com/na06081997-star/CRM_Project/backend/internal/service"
	"github.com/na06081997-star/CRM_Project/backend/internal/storage"
	"github.com/na06081997-star/CRM_Project/backend/internal/store"
)

// version is set at build time: -ldflags "-X main.version=v1.2.3"
var version = "dev"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local /api/health endpoint and exit")
	flag.Parse()
	if *healthcheck {
		os.Exit(probe())
	}

	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))
	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	pool, err := db.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := db.Migrate(ctx, pool); err != nil {
		return fmt.Errorf("migrate: %w", err)
	}
	st := store.New(pool)

	var rdb *redis.Client
	if cfg.RedisURL != "" {
		opts, err := redis.ParseURL(cfg.RedisURL)
		if err != nil {
			return fmt.Errorf("redis url: %w", err)
		}
		rdb = redis.NewClient(opts)
		if err := rdb.Ping(ctx).Err(); err != nil {
			slog.Warn("redis unavailable, realtime runs in single-instance mode", "err", err)
			rdb = nil
		}
	}
	hub := realtime.NewHub(rdb, cfg.CORSOrigins)
	go hub.Run(ctx)

	files, err := storage.NewS3(cfg.S3Endpoint, cfg.S3AccessKey, cfg.S3SecretKey, cfg.S3Bucket, cfg.S3Region)
	if err != nil {
		return err
	}
	dispatcher := notify.NewDispatcher(cfg, st)
	svc := service.New(cfg, st, hub, dispatcher, files)

	if err := svc.BootstrapOrganization(ctx); err != nil {
		return fmt.Errorf("organization: %w", err)
	}
	if !cfg.IsProduction() || os.Getenv("SEED_DEMO") == "true" {
		if err := svc.SeedDemo(ctx); err != nil {
			return fmt.Errorf("seed: %w", err)
		}
	}

	local := auth.NewLocal(cfg.JWTSecret)
	var verifier auth.Verifier = local
	if cfg.AuthMode == "firebase" {
		verifier = auth.Chain{auth.NewFirebase(cfg.FirebaseProjectID), local}
	}

	go runScheduler(ctx, svc)

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           api.NewServer(cfg, svc, verifier, local, hub, files, version).Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		IdleTimeout:       120 * time.Second,
	}
	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		srv.Shutdown(shutdownCtx) //nolint:errcheck
	}()

	slog.Info("server listening", "addr", cfg.HTTPAddr, "version", version, "auth", cfg.AuthMode, "env", cfg.Env)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Info("server stopped")
	return nil
}

// runScheduler drives recurring background jobs.
func runScheduler(ctx context.Context, svc *service.Service) {
	svc.MaterializeSchedules(ctx)
	minute := time.NewTicker(time.Minute)
	hourly := time.NewTicker(time.Hour)
	defer minute.Stop()
	defer hourly.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-minute.C:
			svc.SendDueReminders(ctx)
			svc.SignageTick(ctx)
		case <-hourly.C:
			svc.MaterializeSchedules(ctx)
		}
	}
}

func probe() int {
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" || addr[0] == ':' {
		addr = "127.0.0.1" + addr
	}
	if addr == "127.0.0.1" {
		addr += ":8080"
	}
	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get("http://" + addr + "/api/health")
	if err != nil {
		return 1
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return 1
	}
	return 0
}
