package repository

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ExpiredRun identifies a run that retention has selected for deletion.
type ExpiredRun struct {
	RunID  string
	TestID string
	LogURL string
}

type RetentionRepository interface {
	// ListExpiredRuns returns up to limit runs that fall outside the policy:
	// beyond the newest maxRuns for their test (when maxRuns > 0) or started
	// before cutoff (when non-nil). The newest run of every test is always kept.
	ListExpiredRuns(ctx context.Context, maxRuns int, cutoff *time.Time, limit int) ([]ExpiredRun, error)
	DeleteRuns(ctx context.Context, runIDs []string) error
	// DeleteFinishedJobsBefore removes completed/failed/cancelled jobs that
	// finished before cutoff. Queued and running jobs are never touched.
	DeleteFinishedJobsBefore(ctx context.Context, cutoff time.Time) (int64, error)
}

type postgresRetentionRepository struct {
	pool *pgxpool.Pool
}

func NewRetentionRepository(pool *pgxpool.Pool) RetentionRepository {
	return &postgresRetentionRepository{pool: pool}
}

func (r *postgresRetentionRepository) ListExpiredRuns(ctx context.Context, maxRuns int, cutoff *time.Time, limit int) ([]ExpiredRun, error) {
	var cut *time.Time
	if cutoff != nil {
		u := cutoff.UTC()
		cut = &u
	}
	rows, err := r.pool.Query(ctx, `
		SELECT uuid::text, test_id::text, log_url FROM (
			SELECT uuid, test_id, log_url, started_at,
			       ROW_NUMBER() OVER (PARTITION BY test_id ORDER BY started_at DESC, uuid) AS rn
			FROM tests_runs
		) x
		WHERE rn > 1
		  AND (($1 > 0 AND rn > $1) OR ($2::timestamp IS NOT NULL AND started_at < $2::timestamp))
		ORDER BY started_at
		LIMIT $3`, maxRuns, cut, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ExpiredRun
	for rows.Next() {
		var e ExpiredRun
		if err := rows.Scan(&e.RunID, &e.TestID, &e.LogURL); err != nil {
			return nil, err
		}
		out = append(out, e)
	}
	return out, rows.Err()
}

// DeleteRuns removes the runs; their screenshot rows go with them (ON DELETE CASCADE).
func (r *postgresRetentionRepository) DeleteRuns(ctx context.Context, runIDs []string) error {
	if len(runIDs) == 0 {
		return nil
	}
	_, err := r.pool.Exec(ctx, `DELETE FROM tests_runs WHERE uuid = ANY($1::uuid[])`, runIDs)
	return err
}

func (r *postgresRetentionRepository) DeleteFinishedJobsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	tag, err := r.pool.Exec(ctx, `
		DELETE FROM jobs
		WHERE status IN ('completed', 'failed', 'cancelled')
		  AND finished_at < $1::timestamp`, cutoff.UTC())
	if err != nil {
		return 0, err
	}
	return tag.RowsAffected(), nil
}
