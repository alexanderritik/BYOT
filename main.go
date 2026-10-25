package main

import (
	"context"
	"embed"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexanderritik/byot/auth"
	"github.com/alexanderritik/byot/config"
	"github.com/alexanderritik/byot/db"
	"github.com/alexanderritik/byot/handler"
	"github.com/alexanderritik/byot/queue"
	"github.com/alexanderritik/byot/repository"
	"github.com/alexanderritik/byot/retention"
	"github.com/alexanderritik/byot/runtime"
	"github.com/alexanderritik/byot/scheduler"
	"github.com/alexanderritik/byot/secrets"
	"github.com/alexanderritik/byot/storage"
	"github.com/alexanderritik/byot/web"
	"github.com/alexanderritik/byot/worker"
	"github.com/google/uuid"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/rs/zerolog/log"
)

// Migrations are embedded so the binary needs no migrations/ folder at runtime.
//
//go:embed migrations/*.sql
var migrationFiles embed.FS

func runMigrations(dbURL string) error {
	src, err := iofs.New(migrationFiles, "migrations")
	if err != nil {
		return err
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, dbURL)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func pullImages() {
	for _, img := range runtime.Images {
		log.Info().Str("image", img).Msg("pulling")
		cmd := exec.Command("docker", "pull", img)
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		if err := cmd.Run(); err != nil {
			log.Fatal().Err(err).Str("image", img).Msg("pull failed")
		}
	}
}

func main() {
	if len(os.Args) > 1 && os.Args[1] == "pull-images" {
		pullImages()
		return
	}

	cfg := config.LoadConfig()

	pool, err := db.Connect(cfg.DBUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}

	if os.Getenv("AUTO_MIGRATE") != "false" {
		if err := runMigrations(cfg.DBUrl); err != nil {
			log.Fatal().Err(err).Msg("migration failed (set AUTO_MIGRATE=false to manage migrations yourself)")
		}
		log.Info().Msg("database migrations up to date")
	}

	var store storage.Storage

	switch cfg.StorageProvider {

	case "minio":

		store, err = storage.NewMinioStorage(
			cfg.MinioEndpoint,
			cfg.MinioAccessKey,
			cfg.MinioSecretKey,
			cfg.MinioBucket,
			cfg.MinioRegion,
		)

	case "supabase":

		store, err = storage.NewSupabaseStorage(
			cfg.SupabaseURL,
			cfg.SupabaseKey,
			cfg.SupabaseBucket,
		)

	default:

		log.Fatal().
			Str(
				"provider",
				cfg.StorageProvider,
			).
			Msg("unsupported storage provider")
	}

	if err != nil {

		log.Fatal().
			Err(err).
			Str(
				"provider",
				cfg.StorageProvider,
			).
			Msg("failed to initialize storage")
	}

	log.Info().
		Str(
			"provider",
			cfg.StorageProvider,
		).
		Msg("storage initialized")

	verifier := auth.New(cfg.SupabaseURL, cfg.SupabasePublishableKey)
	if !verifier.Enabled() {
		log.Warn().Msg("SUPABASE_URL/SUPABASE_PUBLISHABLE_KEY not set: upload authentication is DISABLED")
	}

	box, err := secrets.New(cfg.SecretsKey)
	if err != nil {
		log.Fatal().Err(err).Msg("invalid SECRETS_KEY")
	}
	if !box.Enabled() {
		log.Warn().Msg("SECRETS_KEY not set: secret environment variables are DISABLED (plain variables still work)")
	}

	testRepo := repository.NewTestRepository(pool)
	envRepo := repository.NewEnvRepository(pool)
	testRunRepo := repository.NewTestRunRepository(pool)
	jobRepo := repository.NewJobRepository(pool)
	screenshotRepo := repository.NewScreenshotRepository(pool)

	q := queue.NewQueue(jobRepo)

	sched := scheduler.NewScheduler(testRepo, q, jobRepo, 30*time.Second)
	handle := handler.NewHandler(store, testRepo, testRunRepo, screenshotRepo, q, sched, cfg, envRepo, box, verifier.Require)

	// Create and start worker
	workerID := uuid.NewString()
	w := worker.NewWorker(workerID, q, testRepo, testRunRepo, screenshotRepo, store, envRepo, box, cfg.WorkerConcurrency)
	workerCtx, stopWorker := context.WithCancel(context.Background())
	defer stopWorker()

	workerDone := make(chan struct{})
	go func() {
		w.Start(workerCtx)
		close(workerDone)
	}()
	go sched.Start(workerCtx)

	sweeper := retention.New(repository.NewRetentionRepository(pool), store, retention.Policy{
		MaxRunsPerTest: cfg.RetentionMaxRuns,
		MaxAge:         time.Duration(cfg.RetentionDays) * 24 * time.Hour,
	})
	go sweeper.Run(workerCtx, time.Hour)

	mux := http.NewServeMux()
	mux.HandleFunc("/auth/config", verifier.PublicConfig)

	web.Register(mux)
	mux.HandleFunc("/health", verifier.RequireWrites(handle.IsHealth))
	mux.HandleFunc("/uploadBinary", verifier.RequireWrites(handle.UploadBinary))
	mux.HandleFunc("/run", verifier.RequireWrites(handle.Run))
	mux.HandleFunc("/stop", verifier.RequireWrites(handle.Stop))
	mux.HandleFunc("/scheduler/status", verifier.RequireWrites(handle.SchedulerStatus))
	mux.HandleFunc("/scheduler/stop", verifier.RequireWrites(handle.SchedulerStop))
	mux.HandleFunc("/scheduler/start", verifier.RequireWrites(handle.SchedulerStart))
	mux.HandleFunc("/tests", verifier.RequireWrites(handle.Tests))
	mux.HandleFunc("/tests/", verifier.RequireWrites(handle.Tests))
	mux.HandleFunc("/runs/", verifier.RequireWrites(handle.RunRoutes))
	mux.HandleFunc("/status/", verifier.RequireWrites(handle.JobStatus))
	mux.HandleFunc("/screenshots/", verifier.RequireWrites(handle.ListScreenshots))
	mux.HandleFunc("/screenshot/", verifier.RequireWrites(handle.DownloadScreenshot))

	server := &http.Server{
		Addr:    ":3000",
		Handler: mux,
	}

	go func() {
		log.Info().Msg("server listening on :3000")

		if err := server.ListenAndServe(); err != nil &&
			err != http.ErrServerClosed {
			log.Fatal().Err(err).Msg("server failed")
		}
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	sig := <-sigChan
	log.Info().Str("signal", sig.String()).Msg("shutdown initiated")

	// Allow active requests to finish
	shutdownCtx, cancel := context.WithTimeout(
		context.Background(),
		30*time.Second,
	)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Error().Err(err).Msg("http shutdown failed")
	}

	// Stop claiming jobs and let running ones finish (bounded by the same
	// deadline). Anything still running is left for stale-job recovery.
	stopWorker()
	select {
	case <-workerDone:
	case <-shutdownCtx.Done():
		log.Warn().Msg("shutdown deadline hit with jobs still running")
	}

	// Close postgres pool
	pool.Close()

	log.Info().Msg("shutdown complete")
}
