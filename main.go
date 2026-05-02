package main

import (
	"net/http"

	"github.com/alexanderritik/mini-lambda/config"
	"github.com/alexanderritik/mini-lambda/handler"
	"github.com/alexanderritik/mini-lambda/storage"

	"github.com/rs/zerolog/log"
)

func main() {
	cfg := config.LoadConfig()

	store, err := storage.NewMinioStorage(
		cfg.MinioEndpoint,
		cfg.MinioAccessKey,
		cfg.MinioSecretKey,
		cfg.MinioBucket,
	)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to connect to minio")
	}

	handle := handler.NewHandler(store)

	// pass store to handler
	http.HandleFunc("/health", handle.IsHealth)
	http.HandleFunc("/uploadBinary", handle.UploadBinary) // needs store
	http.HandleFunc("/run/", handle.Run)
	http.ListenAndServe(":3000", nil)
}
