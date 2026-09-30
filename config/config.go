package config

import (
	"log"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	MinioEndpoint     string
	MinioAccessKey    string
	MinioSecretKey    string
	MinioBucket       string
	MinioRegion       string
	DBUrl             string
	MaxDockerMemoryMB int
	MaxDockerCPUs     float64
}

func LoadConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	println("Loading config...", os.Getenv("MINIO_ENDPOINT"), os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), os.Getenv("MINIO_BUCKET"), os.Getenv("DB_URL"))

	maxMemory := 2048 // default 2GB
	if val := os.Getenv("MAX_DOCKER_MEMORY_MB"); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
			maxMemory = parsed
		}
	}

	maxCPUs := 4.0 // default 4 CPUs
	if val := os.Getenv("MAX_DOCKER_CPUS"); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil && parsed > 0 {
			maxCPUs = parsed
		}
	}

	return Config{
		MinioEndpoint:     os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey:    os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey:    os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:       os.Getenv("MINIO_BUCKET"),
		MinioRegion:       os.Getenv("MINIO_REGION"),
		DBUrl:             os.Getenv("DB_URL"),
		MaxDockerMemoryMB: maxMemory,
		MaxDockerCPUs:     maxCPUs,
	}
}
