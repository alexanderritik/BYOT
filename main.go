package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/alexanderritik/mini-lambda/config"
	"github.com/alexanderritik/mini-lambda/db"
	"github.com/alexanderritik/mini-lambda/handler"
	"github.com/alexanderritik/mini-lambda/queue"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/storage"
	"github.com/alexanderritik/mini-lambda/worker"
	"github.com/google/uuid"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/rs/zerolog/log"
)

func runMigrations(dbURL string) error {
	m, err := migrate.New(
		"file://migrations",
		dbURL,
	)
	if err != nil {
		return err
	}
	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return err
	}
	return nil
}

func main() {
	cfg := config.LoadConfig()

	pool, err := db.Connect(cfg.DBUrl)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to postgres")
	}

	if err := runMigrations(cfg.DBUrl); err != nil {
		log.Fatal().Err(err).Msg("migration failed")
	}

	store, err := storage.NewMinioStorage(
		cfg.MinioEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioBucket,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to minio")
	}

	testRepo := repository.NewTestRepository(pool)
	testRunRepo := repository.NewTestRunRepository(pool)
	jobRepo := repository.NewJobRepository(pool)

	q := queue.NewQueue(jobRepo)

	handle := handler.NewHandler(store, testRepo, testRunRepo, q)

	// Create and start worker
	workerID := uuid.NewString()
	w := worker.NewWorker(workerID, q, testRepo, testRunRepo, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go w.Start(ctx)

	// pass store to handler
	mux := http.NewServeMux()
	mux.HandleFunc("/health", handle.IsHealth)
	mux.HandleFunc("/uploadBinary", handle.UploadBinary) // needs store
	mux.HandleFunc("/run", handle.Run)
	mux.HandleFunc("/status/", handle.JobStatus)

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

	// Close postgres pool
	pool.Close()

	log.Info().Msg("shutdown complete")
}
