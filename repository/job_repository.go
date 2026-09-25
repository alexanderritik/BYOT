package repository

import (
	"context"
	"errors"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type JobRepository struct {
	pool *pgxpool.Pool
}

func NewJobRepository(pool *pgxpool.Pool) *JobRepository {
	return &JobRepository{pool: pool}
}

func (r *JobRepository) Create(ctx context.Context, job *model.Job) error {
	query := `
		INSERT INTO jobs (uuid, test_id, status, queued_at, trigger)
		VALUES ($1, $2, $3, $4, $5)
	`
	_, err := r.pool.Exec(ctx, query, job.UUID, job.TestID, job.Status, job.QueuedAt, job.Trigger)
	return err
}

func (r *JobRepository) HasActiveJob(ctx context.Context, testID string) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx,
		`SELECT EXISTS(
			SELECT 1 FROM jobs WHERE test_id = $1 AND status IN ('queued', 'running')
		)`,
		testID,
	).Scan(&exists)
	return exists, err
}

func (r *JobRepository) GetByID(ctx context.Context, id string) (*model.Job, error) {
	query := `
		SELECT uuid, test_id, status, trigger, queued_at, started_at, finished_at, worker_id, error_message
		FROM jobs WHERE uuid = $1
	`
	row := r.pool.QueryRow(ctx, query, id)

	var job model.Job
	err := row.Scan(
		&job.UUID,
		&job.TestID,
		&job.Status,
		&job.Trigger,
		&job.QueuedAt,
		&job.StartedAt,
		&job.FinishedAt,
		&job.WorkerID,
		&job.ErrorMessage,
	)
	if err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) Dequeue(ctx context.Context, workerID string) (*model.Job, error) {
	query := `
		UPDATE jobs
		SET status = 'running',
		    started_at = NOW(),
		    worker_id = $1
		WHERE uuid = (
			SELECT uuid FROM jobs
			WHERE status = 'queued'
			ORDER BY queued_at ASC
			FOR UPDATE SKIP LOCKED
			LIMIT 1
		)
		RETURNING uuid, test_id, status, trigger, queued_at, started_at, finished_at, worker_id, error_message
	`

	row := r.pool.QueryRow(ctx, query, workerID)

	var job model.Job
	err := row.Scan(
		&job.UUID,
		&job.TestID,
		&job.Status,
		&job.Trigger,
		&job.QueuedAt,
		&job.StartedAt,
		&job.FinishedAt,
		&job.WorkerID,
		&job.ErrorMessage,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, err
	}
	return &job, nil
}

func (r *JobRepository) UpdateStatus(ctx context.Context, jobID string, status string, errorMessage *string) error {
	query := `
		UPDATE jobs
		SET status = $1,
		    finished_at = NOW(),
		    error_message = $2
		WHERE uuid = $3
	`
	_, err := r.pool.Exec(ctx, query, status, errorMessage, jobID)
	return err
}

func (r *JobRepository) ListByTestID(ctx context.Context, testID string) ([]*model.Job, error) {
	query := `
		SELECT uuid, test_id, status, trigger, queued_at, started_at, finished_at, worker_id, error_message
		FROM jobs WHERE test_id = $1
		ORDER BY queued_at DESC
	`
	rows, err := r.pool.Query(ctx, query, testID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var jobs []*model.Job
	for rows.Next() {
		var job model.Job
		err := rows.Scan(
			&job.UUID,
			&job.TestID,
			&job.Status,
			&job.Trigger,
			&job.QueuedAt,
			&job.StartedAt,
			&job.FinishedAt,
			&job.WorkerID,
			&job.ErrorMessage,
		)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, &job)
	}
	return jobs, nil
}
