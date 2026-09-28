package repository

import (
	"context"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ScreenshotRepository interface {
	Create(ctx context.Context, screenshot *model.Screenshot) error
	ListByRunID(ctx context.Context, runID string) ([]model.Screenshot, error)
	GetByUUID(ctx context.Context, uuid string) (*model.Screenshot, error)
}

type postgresScreenshotRepository struct {
	pool *pgxpool.Pool
}

func NewScreenshotRepository(pool *pgxpool.Pool) ScreenshotRepository {
	return &postgresScreenshotRepository{pool: pool}
}

func (r *postgresScreenshotRepository) Create(ctx context.Context, screenshot *model.Screenshot) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO screenshots (uuid, run_id, filename, storage_key, created_at)
		 VALUES ($1, $2, $3, $4, $5)`,
		screenshot.UUID,
		screenshot.RunID,
		screenshot.Filename,
		screenshot.StorageKey,
		screenshot.CreatedAt,
	)
	return err
}

func (r *postgresScreenshotRepository) ListByRunID(ctx context.Context, runID string) ([]model.Screenshot, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT uuid::text, run_id::text, filename, storage_key, created_at
		 FROM screenshots
		 WHERE run_id = $1
		 ORDER BY created_at ASC`,
		runID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var screenshots []model.Screenshot
	for rows.Next() {
		var s model.Screenshot
		if err := rows.Scan(
			&s.UUID,
			&s.RunID,
			&s.Filename,
			&s.StorageKey,
			&s.CreatedAt,
		); err != nil {
			return nil, err
		}
		screenshots = append(screenshots, s)
	}
	return screenshots, rows.Err()
}

func (r *postgresScreenshotRepository) GetByUUID(ctx context.Context, uuid string) (*model.Screenshot, error) {
	var s model.Screenshot
	err := r.pool.QueryRow(ctx,
		`SELECT uuid::text, run_id::text, filename, storage_key, created_at
		 FROM screenshots
		 WHERE uuid = $1`,
		uuid,
	).Scan(
		&s.UUID,
		&s.RunID,
		&s.Filename,
		&s.StorageKey,
		&s.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &s, nil
}
