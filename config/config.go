package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	StorageProvider string

	// MinIO
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	MinioRegion    string

	// Supabase
	SupabaseURL    string
	SupabaseKey    string
	SupabaseBucket string

	DBUrl             string
	MaxDockerMemoryMB int
	MaxDockerCPUs     float64
}

func LoadConfig() Config {

	if err := godotenv.Load(); err != nil {
		log.Printf(
			"warning: .env file not found: %v",
			err,
		)
	}

	maxMemory := 2048

	if value := os.Getenv(
		"MAX_DOCKER_MEMORY_MB",
	); value != "" {

		if parsed, err := strconv.Atoi(value); err == nil &&
			parsed > 0 {

			maxMemory = parsed
		}
	}

	maxCPUs := 4.0

	if value := os.Getenv(
		"MAX_DOCKER_CPUS",
	); value != "" {

		if parsed, err := strconv.ParseFloat(
			value,
			64,
		); err == nil && parsed > 0 {

			maxCPUs = parsed
		}
	}

	return Config{

		// Storage
		StorageProvider: getEnv(
			"STORAGE_PROVIDER",
			"minio",
		),

		// MinIO
		MinioEndpoint: os.Getenv(
			"MINIO_ENDPOINT",
		),
		MinioAccessKey: os.Getenv(
			"MINIO_ACCESS_KEY",
		),
		MinioSecretKey: os.Getenv(
			"MINIO_SECRET_KEY",
		),
		MinioBucket: os.Getenv(
			"MINIO_BUCKET",
		),
		MinioRegion: getEnv(
			"MINIO_REGION",
			"us-east-1",
		),

		// Supabase
		SupabaseURL: os.Getenv(
			"SUPABASE_URL",
		),
		SupabaseKey: os.Getenv(
			"SUPABASE_KEY",
		),
		SupabaseBucket: os.Getenv(
			"SUPABASE_BUCKET",
		),

		// Database
		DBUrl: os.Getenv(
			"DB_URL",
		),

		// Docker
		MaxDockerMemoryMB: maxMemory,
		MaxDockerCPUs:     maxCPUs,
	}
}

func getEnv(
	key string,
	fallback string,
) string {

	value := os.Getenv(key)

	if value == "" {
		return fallback
	}

	return value
}
