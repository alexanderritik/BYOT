package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	MinioEndpoint  string
	MinioAccessKey string
	MinioSecretKey string
	MinioBucket    string
	DBUrl          string
}

func LoadConfig() Config {
	err := godotenv.Load()
	if err != nil {
		log.Fatal(err)
	}

	println("Loading config...", os.Getenv("MINIO_ENDPOINT"), os.Getenv("MINIO_ACCESS_KEY"), os.Getenv("MINIO_SECRET_KEY"), os.Getenv("MINIO_BUCKET"), os.Getenv("DB_URL"))
	return Config{
		MinioEndpoint:  os.Getenv("MINIO_ENDPOINT"),
		MinioAccessKey: os.Getenv("MINIO_ACCESS_KEY"),
		MinioSecretKey: os.Getenv("MINIO_SECRET_KEY"),
		MinioBucket:    os.Getenv("MINIO_BUCKET"),
		DBUrl:          os.Getenv("DB_URL"),
	}
}
