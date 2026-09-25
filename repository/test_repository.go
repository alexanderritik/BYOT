package repository

import (
	"context"
	"time"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TestConfigUpdate struct {
	Command         string
	Severity        string
	TimeoutSeconds  int
	ScheduleCron    string
	ScheduleEnabled bool
	NextRunAt       *time.Time
}

type TestRepository interface {
	Create(ctx context.Context, test *model.Test) error
	GetByID(ctx context.Context, uuid string) (*model.Test, error)
	UpdateConfig(ctx context.Context, testID string, cfg TestConfigUpdate) error
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
			schedule_cron, schedule_enabled, next_run_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)`,
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
	var test model.Test
	err := r.pool.QueryRow(ctx,
		`SELECT uuid::text, COALESCE(name, ''), runtime, original_filename, severity,
		        COALESCE(command, ''), COALESCE(artifact_key, ''), created_at, timeout_seconds,
		        COALESCE(schedule_cron, ''), schedule_enabled, next_run_at
		 FROM tests WHERE uuid = $1`,
		uuid,
	).Scan(
		&test.UUID,
		&test.Name,
		&test.Runtime,
		&test.OriginalFilename,
		&test.Severity,
		&test.Command,
		&test.ArtifactKey,
		&test.CreatedAt,
		&test.TimeoutSeconds,
		&test.ScheduleCron,
		&test.ScheduleEnabled,
		&test.NextRunAt,
	)
	if err != nil {
		return nil, err
	}
	return &test, nil
}

func (r *postgresTestRepository) UpdateConfig(ctx context.Context, testID string, cfg TestConfigUpdate) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET
			command = $1,
			severity = $2,
			timeout_seconds = $3,
			schedule_cron = $4,
			schedule_enabled = $5,
			next_run_at = $6
		 WHERE uuid = $7`,
		cfg.Command,
		cfg.Severity,
		cfg.TimeoutSeconds,
		nullIfEmpty(cfg.ScheduleCron),
		cfg.ScheduleEnabled,
		cfg.NextRunAt,
		testID,
	)
	return err
}

func (r *postgresTestRepository) ListDueScheduled(ctx context.Context, now time.Time) ([]*model.Test, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT uuid::text, COALESCE(name, ''), runtime, original_filename, severity,
		        COALESCE(command, ''), COALESCE(artifact_key, ''), created_at, timeout_seconds,
		        COALESCE(schedule_cron, ''), schedule_enabled, next_run_at
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
	defer rows.Close()

	var out []*model.Test
	for rows.Next() {
		var test model.Test
		if err := rows.Scan(
			&test.UUID,
			&test.Name,
			&test.Runtime,
			&test.OriginalFilename,
			&test.Severity,
			&test.Command,
			&test.ArtifactKey,
			&test.CreatedAt,
			&test.TimeoutSeconds,
			&test.ScheduleCron,
			&test.ScheduleEnabled,
			&test.NextRunAt,
		); err != nil {
			return nil, err
		}
		out = append(out, &test)
	}
	return out, rows.Err()
}

func (r *postgresTestRepository) UpdateNextRunAt(ctx context.Context, testID string, next time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET next_run_at = $1 WHERE uuid = $2`,
		next.UTC(), testID,
	)
	return err
}
