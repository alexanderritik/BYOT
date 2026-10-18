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
	SupabaseURL string
	SupabaseKey string
	// Public publishable key (sb_publishable_...); with SupabaseURL it enables auth on upload.
	SupabasePublishableKey string
	SupabaseBucket         string

	// Base64 32-byte key encrypting per-test secrets; empty disables secrets.
	SecretsKey string

	DBUrl string
	// Jobs run in parallel per instance (WORKER_CONCURRENCY, default 3).
	WorkerConcurrency int
	// Retention: 0 disables that limit. The newest run of each test is always kept.
	RetentionMaxRuns  int
	RetentionDays     int
	MaxDockerMemoryMB int
	MaxDockerCPUs     float64
	// Upper bound for a test's timeout (MAX_TIMEOUT_SECONDS, default 900).
	MaxTimeoutSeconds int
}

func LoadConfig() Config {

	if err := godotenv.Load(); err != nil {
		log.Printf(
			"warning: .env file not found: %v",
			err,
		)
	}

	concurrency := 3
	if value := os.Getenv("WORKER_CONCURRENCY"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			concurrency = min(parsed, 32)
		}
	}

	retentionMaxRuns := envInt("RETENTION_MAX_RUNS_PER_TEST", 100)
	retentionDays := envInt("RETENTION_DAYS", 30)
	maxTimeout := envInt("MAX_TIMEOUT_SECONDS", 900)
	if maxTimeout == 0 {
		maxTimeout = 900
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
		SupabasePublishableKey: os.Getenv(
			"SUPABASE_PUBLISHABLE_KEY",
		),

		SecretsKey: os.Getenv("SECRETS_KEY"),

		// Database
		DBUrl: os.Getenv(
			"DB_URL",
		),

		// Docker
		WorkerConcurrency: concurrency,
		RetentionMaxRuns:  retentionMaxRuns,
		RetentionDays:     retentionDays,
		MaxDockerMemoryMB: maxMemory,
		MaxDockerCPUs:     maxCPUs,
		MaxTimeoutSeconds: maxTimeout,
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

// envInt reads a non-negative integer; unset or invalid falls back to def.
func envInt(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n >= 0 {
			return n
		}
	}
	return def
}
