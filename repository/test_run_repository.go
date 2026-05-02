// repository/test_run_repository.go
package repository

import (
	"context"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TestRunRepository interface {
	Create(ctx context.Context, run *model.TestRun) error
	ListByTestID(ctx context.Context, testID string) ([]*model.TestRun, error)
}

type postgresTestRunRepository struct {
	pool *pgxpool.Pool
}

func NewTestRunRepository(pool *pgxpool.Pool) TestRunRepository {
	return &postgresTestRunRepository{pool: pool}
}

func (r *postgresTestRunRepository) Create(ctx context.Context, run *model.TestRun) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tests_runs (uuid, test_id, status, duration_ms, started_at, finished_at, log_url, log_size_bytes)
         VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
		run.UUID, run.TestID, run.Status, run.DurationMs,
		run.StartedAt, run.FinishedAt, run.LogURL, run.LogSizeBytes,
	)
	return err
}

func (r *postgresTestRunRepository) ListByTestID(ctx context.Context, testID string) ([]*model.TestRun, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT uuid, test_id, status, duration_ms, started_at, finished_at, log_url, log_size_bytes
         FROM tests_runs WHERE test_id = $1 ORDER BY started_at DESC`,
		testID,
	)
	if err != nil {
		return nil, err
	}
	runs, err := pgx.CollectRows(rows, pgx.RowToStructByName[model.TestRun])
	if err != nil {
		return nil, err
	}
	result := make([]*model.TestRun, len(runs))
	for i := range runs {
		result[i] = &runs[i]
	}
	return result, nil
}
