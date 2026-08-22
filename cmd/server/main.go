package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/api"
	"github.com/FernasFragas/dashboard-app/internal/seed"
	"github.com/FernasFragas/dashboard-app/internal/store"
	"github.com/FernasFragas/dashboard-app/migrations"
)

var version = "dev"

type config struct {
	addr      string
	dataDir   string
	token     string
	logFormat string
	timezone  string
}

func main() {
	cfg := parseFlags()

	logger, err := newLogger(cfg.logFormat)
	if err != nil {
		slog.Error("invalid log format", "format", cfg.logFormat, "error", err)
		os.Exit(2)
	}
	slog.SetDefault(logger)

	dataDir, err := expandHome(cfg.dataDir)
	if err != nil {
		slog.Error("resolve data dir", "path", cfg.dataDir, "error", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		slog.Error("create data dir", "path", dataDir, "error", err)
		os.Exit(1)
	}

	// The day boundary governs streaks, log-feed grouping and the daily-review key. A bad zone
	// name would silently shift every one of them, so it is resolved before anything else runs.
	location, err := time.LoadLocation(cfg.timezone)
	if err != nil {
		slog.Error("load timezone", "tz", cfg.timezone, "error", err)
		os.Exit(2)
	}

	if cfg.token != "" {
		slog.Info("auth token configured")
	}

	// Startup failures - migration error, locked database, unparseable seed - exit non-zero.
	// There is no degraded mode: a half-started app serving 500s is worse than one plainly
	// down, because you would trust data that isn't there (docs/ARCHITECTURE.md section 5).
	db, err := openStore(dataDir)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}

	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	server := &http.Server{
		Addr: cfg.addr,
		Handler: api.NewMux(api.Config{
			Store:    db,
			Version:  version,
			Token:    cfg.token,
			Location: location,
			Logger:   logger,
		}),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown server", "error", err)
		}
	}()

	slog.Info("server starting",
		"addr", cfg.addr, "data", dataDir, "tz", location.String(), "version", version)
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

// openStore opens the database, applies migrations, and runs the additive seed loader.
//
// The loader is additive-upsert: it inserts what is missing and never updates or deletes, so
// running it at every boot picks up plan amendments without touching real progress
// (docs/DATABASE.md section 8).
func openStore(dataDir string) (*store.Store, error) {
	ctx := context.Background()
	path := filepath.Join(dataDir, "dashboard.db")

	db, err := store.Open(path)
	if err != nil {
		return nil, err
	}

	if err := db.Migrate(ctx, migrations.FS); err != nil {
		_ = db.Close()
		return nil, err
	}

	doc, err := seed.Load()
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	res, err := seed.Apply(ctx, db.DB(), doc)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	version, err := db.SchemaVersion(ctx)
	if err != nil {
		_ = db.Close()
		return nil, err
	}

	slog.Info("database ready", "path", path, "schema_version", version)

	if res.Total() > 0 {
		slog.Info("seed applied",
			"weeks", res.Weeks, "rhythm", res.Rhythm, "categories", res.Categories,
			"metric_defs", res.MetricDefs, "goals", res.Goals, "tasks", res.Tasks,
			"checkpoints", res.Checkpoints)
	}

	return db, nil
}

func parseFlags() config {
	defaultDataDir := filepath.Join("~", "dashboard-data")

	cfg := config{}
	flag.StringVar(&cfg.addr, "addr", ":8484", "HTTP listen address")
	flag.StringVar(&cfg.dataDir, "data", defaultDataDir, "application data directory")
	flag.StringVar(&cfg.token, "token", "", "optional API token")
	flag.StringVar(&cfg.logFormat, "log-format", "text", "log format: text or json")
	flag.StringVar(&cfg.timezone, "tz", "Europe/Lisbon",
		"IANA timezone defining day boundaries for streaks, the log feed and daily reviews")
	flag.Parse()

	return cfg
}

func newLogger(format string) (*slog.Logger, error) {
	opts := &slog.HandlerOptions{Level: slog.LevelInfo}

	switch format {
	case "text":
		return slog.New(slog.NewTextHandler(os.Stdout, opts)), nil
	case "json":
		return slog.New(slog.NewJSONHandler(os.Stdout, opts)), nil
	default:
		return nil, errors.New("must be text or json")
	}
}

func expandHome(path string) (string, error) {
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return home, nil
	}

	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, strings.TrimPrefix(path, "~/")), nil
	}

	return path, nil
}
