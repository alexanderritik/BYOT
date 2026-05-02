package main

import (
	"net/http"

	"github.com/alexanderritik/mini-lambda/config"
	"github.com/alexanderritik/mini-lambda/db"
	"github.com/alexanderritik/mini-lambda/handler"
	"github.com/alexanderritik/mini-lambda/storage"

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
	defer pool.Close()

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

	handle := handler.NewHandler(store, pool)

	// pass store to handler
	http.HandleFunc("/health", handle.IsHealth)
	http.HandleFunc("/uploadBinary", handle.UploadBinary) // needs store
	http.HandleFunc("/run/", handle.Run)
	http.ListenAndServe(":3000", nil)
}
