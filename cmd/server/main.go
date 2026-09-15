package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/FernasFragas/dashboard-app/internal/api"
	"github.com/FernasFragas/dashboard-app/internal/backup"
	"github.com/FernasFragas/dashboard-app/internal/plan"
	"github.com/FernasFragas/dashboard-app/internal/seed"
	"github.com/FernasFragas/dashboard-app/internal/store"
	appweb "github.com/FernasFragas/dashboard-app/internal/web"
	"github.com/FernasFragas/dashboard-app/migrations"
)

var version = "dev"

type config struct {
	addr      string
	dataDir   string
	token     string
	logFormat string
	timezone  string
	publicURL string
	planPath  string
	planReset bool
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "plan" {
		os.Exit(runPlanCommand(os.Args[2:]))
	}

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

	if cfg.planReset {
		if err := resetPlanDatabase(dataDir); err != nil {
			slog.Error("reset plan database", "error", err)
			os.Exit(1)
		}
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

	publicURL, err := resolvePublicURL(cfg.publicURL, cfg.addr)
	if err != nil {
		slog.Error("resolve public url", "public_url", cfg.publicURL, "addr", cfg.addr, "error", err)
		os.Exit(2)
	}

	// Startup failures - migration error, locked database, unparseable seed - exit non-zero.
	// There is no degraded mode: a half-started app serving 500s is worse than one plainly
	// down, because you would trust data that isn't there (docs/ARCHITECTURE.md section 5).
	doc, err := loadSeedDocument(cfg.planPath)
	if err != nil {
		slog.Error("load plan", "error", err)
		os.Exit(1)
	}

	db, err := openStore(dataDir, doc)
	if err != nil {
		slog.Error("open database", "error", err)
		os.Exit(1)
	}

	if swept, err := db.SweepExpiredPairingCodes(context.Background(), time.Now()); err != nil {
		slog.Error("sweep expired pairing codes", "error", err)
		os.Exit(1)
	} else if swept > 0 {
		slog.Info("expired pairing codes swept", "rows", swept)
	}

	defer func() {
		if err := db.Close(); err != nil {
			slog.Error("close database", "error", err)
		}
	}()

	server := &http.Server{
		Addr: cfg.addr,
		Handler: newHTTPHandler(
			api.NewMux(api.Config{
				Store:     db,
				Version:   version,
				Token:     cfg.token,
				PublicURL: publicURL,
				Location:  location,
				Logger:    logger,
			}),
			appweb.Handler(),
		),
		ReadHeaderTimeout: 5 * time.Second,
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	backup.Start(ctx, backup.Config{
		DB:        db.DB(),
		Dir:       filepath.Join(dataDir, "backups"),
		Retention: 14,
		Location:  location,
		Logger:    logger,
	})

	go func() {
		<-ctx.Done()

		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			slog.Error("shutdown server", "error", err)
		}
	}()

	slog.Info("server starting",
		"addr", cfg.addr, "data", dataDir, "tz", location.String(), "version", version,
		"public_url", publicURL, "web_embedded", appweb.Embedded())
	if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}
}

func newHTTPHandler(apiHandler http.Handler, webHandler http.Handler) http.Handler {
	mux := http.NewServeMux()
	mux.Handle("/api", apiHandler)
	mux.Handle("/api/", apiHandler)
	mux.Handle("/pair/", apiHandler)
	mux.Handle("/", webHandler)
	return mux
}

// openStore opens the database, applies migrations, and runs the additive seed loader.
//
// The loader is an additive reference upsert: it inserts what is missing and refreshes reference
// data (labels, help text, week windows), but never deletes rows or overwrites user progress, so
// running it at every boot picks up plan amendments without touching real progress
// (docs/DATABASE.md section 8).
func openStore(dataDir string, doc seed.Document) (*store.Store, error) {
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

	res, err := seed.Apply(ctx, db.DB(), doc)
	if err != nil {
		_ = db.Close()
		if errors.Is(err, seed.ErrPlanMismatch) {
			return nil, fmt.Errorf(
				"%w; use a different -data directory, or create %s.backup and start again with -plan-reset",
				err, path,
			)
		}
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
			"projects", res.Projects, "phases", res.Phases, "skill_tiers", res.SkillTiers,
			"weeks", res.Weeks, "rhythm", res.Rhythm, "categories", res.Categories,
			"metric_defs", res.MetricDefs, "goals", res.Goals, "tasks", res.Tasks,
			"checkpoints", res.Checkpoints, "skills", res.Skills)
	}

	return db, nil
}

func loadSeedDocument(path string) (seed.Document, error) {
	if strings.TrimSpace(path) == "" {
		return seed.Load()
	}

	expanded, err := expandHome(path)
	if err != nil {
		return seed.Document{}, err
	}

	return plan.ParseFile(expanded)
}

func resetPlanDatabase(dataDir string) error {
	dbPath := filepath.Join(dataDir, "dashboard.db")
	if _, err := os.Stat(dbPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat %s: %w", dbPath, err)
	}

	backupPath := dbPath + ".backup"
	if _, err := os.Stat(backupPath); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("-plan-reset requires an existing backup at %s", backupPath)
		}
		return fmt.Errorf("stat %s: %w", backupPath, err)
	}

	for _, path := range []string{dbPath, dbPath + "-wal", dbPath + "-shm"} {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove %s: %w", path, err)
		}
	}

	return nil
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
	flag.StringVar(&cfg.publicURL, "public-url", "",
		"public base URL used in phone pairing QR codes; defaults from -addr")
	flag.StringVar(&cfg.planPath, "plan", "",
		"markdown plan to parse and seed at boot; defaults to the embedded master plan")
	flag.BoolVar(&cfg.planReset, "plan-reset", false,
		"delete dashboard.db before seeding; requires dashboard.db.backup to exist")
	flag.Parse()

	return cfg
}

func resolvePublicURL(explicit, addr string) (string, error) {
	explicit = strings.TrimSpace(explicit)
	if explicit != "" {
		u, err := url.Parse(explicit)
		if err != nil {
			return "", err
		}
		if u.Scheme != "http" && u.Scheme != "https" {
			return "", errors.New("public url scheme must be http or https")
		}
		if u.Host == "" {
			return "", errors.New("public url must include a host")
		}
		u.Path = strings.TrimRight(u.Path, "/")
		u.RawQuery = ""
		u.Fragment = ""
		return strings.TrimRight(u.String(), "/"), nil
	}

	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "", fmt.Errorf("split listen address: %w", err)
	}
	if port == "" {
		return "", errors.New("listen address must include a port")
	}

	if isWildcardHost(host) {
		host, err = os.Hostname()
		if err != nil {
			return "", fmt.Errorf("hostname: %w", err)
		}
	}
	if strings.TrimSpace(host) == "" {
		return "", errors.New("public host is empty")
	}

	return "http://" + net.JoinHostPort(host, port), nil
}

func isWildcardHost(host string) bool {
	switch strings.Trim(host, "[]") {
	case "", "0.0.0.0", "::":
		return true
	default:
		return false
	}
}

func runPlanCommand(args []string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: dashboard plan validate [flags] PLAN.md")
		return 2
	}

	switch args[0] {
	case "validate":
		return runPlanValidate(args[1:])
	default:
		fmt.Fprintf(os.Stderr, "unknown plan command %q\n", args[0])
		fmt.Fprintln(os.Stderr, "usage: dashboard plan validate [flags] PLAN.md")
		return 2
	}
}

func runPlanValidate(args []string) int {
	fs := flag.NewFlagSet("dashboard plan validate", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	year := fs.Int("year", 0, "start year override")

	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: dashboard plan validate [flags] PLAN.md")
		return 2
	}

	path, err := expandHome(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan invalid: %v\n", err)
		return 1
	}

	body, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan invalid: read %s: %v\n", path, err)
		return 1
	}

	profile, err := plan.LoadProfileForPlan(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan invalid: %v\n", err)
		return 1
	}
	if *year != 0 {
		profile.Plan.StartYear = *year
	}

	doc, err := plan.ParseWithProfile(string(body), profile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "plan invalid: %v\n", err)
		return 1
	}

	if _, err := fmt.Fprintf(os.Stdout,
		"plan valid: %d projects, %d weeks, %d tasks, %d goals, %d skills\n",
		len(doc.Projects), len(doc.Weeks), len(doc.Tasks), len(doc.Goals), len(doc.Skills),
	); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "write validation result: %v\n", err)
		return 1
	}
	return 0
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
