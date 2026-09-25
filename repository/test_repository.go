package repository

import (
	"context"
	"time"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TestRepository interface {
	Create(ctx context.Context, test *model.Test) error
	GetByID(ctx context.Context, uuid string) (*model.Test, error)
	ListDueScheduled(ctx context.Context, now time.Time) ([]*model.Test, error)
	UpdateNextRunAt(ctx context.Context, testID string, next time.Time) error
}

type postgresTestRepository struct {
	pool *pgxpool.Pool
}

func NewTestRepository(pool *pgxpool.Pool) TestRepository {
	return &postgresTestRepository{pool: pool}
}

func (r *postgresTestRepository) Create(ctx context.Context, test *model.Test) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tests (
			uuid, name, runtime, original_filename, severity, command, artifact_key, timeout_seconds,
			schedule_cron, schedule_enabled, schedule_timezone, next_run_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12)`,
		test.UUID,
		test.Name,
		test.Runtime,
		test.OriginalFilename,
		test.Severity,
		test.Command,
		test.ArtifactKey,
		test.TimeoutSeconds,
		nullIfEmpty(test.ScheduleCron),
		test.ScheduleEnabled,
		nullIfEmpty(test.ScheduleTimezone),
		test.NextRunAt,
	)
	return err
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (r *postgresTestRepository) GetByID(ctx context.Context, uuid string) (*model.Test, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT uuid, name, runtime, original_filename, severity, command, artifact_key, created_at, timeout_seconds,
		        schedule_cron, schedule_enabled, schedule_timezone, next_run_at
		 FROM tests WHERE uuid = $1`,
		uuid,
	)
	if err != nil {
		return nil, err
	}
	test, err := pgx.CollectOneRow(rows, pgx.RowToStructByName[model.Test])
	if err != nil {
		return nil, err
	}
	return &test, nil
}

func (r *postgresTestRepository) ListDueScheduled(ctx context.Context, now time.Time) ([]*model.Test, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT uuid, name, runtime, original_filename, severity, command, artifact_key, created_at, timeout_seconds,
		        schedule_cron, schedule_enabled, schedule_timezone, next_run_at
		 FROM tests
		 WHERE schedule_enabled = true
		   AND schedule_cron IS NOT NULL
		   AND next_run_at IS NOT NULL
		   AND next_run_at <= $1
		 ORDER BY next_run_at ASC
		 LIMIT 100`,
		now,
	)
	if err != nil {
		return nil, err
	}

	tests, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.Test])
	if err != nil {
		return nil, err
	}

	out := make([]*model.Test, len(tests))
	for i := range tests {
		out[i] = &tests[i]
	}
	return out, nil
}

func (r *postgresTestRepository) UpdateNextRunAt(ctx context.Context, testID string, next time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET next_run_at = $1 WHERE uuid = $2`,
		next.UTC(), testID,
	)
	return err
}
