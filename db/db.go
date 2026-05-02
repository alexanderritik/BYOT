package db

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Connect(dbURL string) (*pgxpool.Pool, error) {

	ctx := context.Background()

	config, _ := pgxpool.ParseConfig(dbURL)
	config.MaxConns = 25 // max 25 connections
	config.MinConns = 5  // keep 5 warm
	config.MaxConnLifetime = time.Hour
	config.MaxConnIdleTime = 30 * time.Minute

	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}

	pingErr := pool.Ping(ctx)
	if pingErr != nil {
		return nil, pingErr
	}
	return pool, nil
}
