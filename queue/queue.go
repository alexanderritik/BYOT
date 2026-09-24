package queue

import (
	"context"
	"time"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Queue struct {
	jobRepo *repository.JobRepository
}

func NewQueue(jobRepo *repository.JobRepository) *Queue {
	return &Queue{jobRepo: jobRepo}
}

func (q *Queue) Enqueue(ctx context.Context, testID string) (*model.Job, error) {
	job := &model.Job{
		UUID:     uuid.NewString(),
		TestID:   testID,
		Status:   "queued",
		QueuedAt: time.Now(),
	}
	
	if err := q.jobRepo.Create(ctx, job); err != nil {
		log.Error().Err(err).Str("test_id", testID).Msg("failed to enqueue job")
		return nil, err
	}
	
	log.Info().Str("job_id", job.UUID).Str("test_id", testID).Msg("job enqueued")
	return job, nil
}

func (q *Queue) Dequeue(ctx context.Context, workerID string) (*model.Job, error) {
	job, err := q.jobRepo.Dequeue(ctx, workerID)
	if err != nil {
		return nil, err
	}
	
	if job == nil {
		return nil, nil // No jobs available
	}
	
	log.Info().Str("job_id", job.UUID).Str("worker_id", workerID).Msg("job dequeued")
	return job, nil
}

func (q *Queue) Complete(ctx context.Context, jobID string, status string, errorMessage *string) error {
	if err := q.jobRepo.UpdateStatus(ctx, jobID, status, errorMessage); err != nil {
		log.Error().Err(err).Str("job_id", jobID).Msg("failed to update job status")
		return err
	}
	
	log.Info().Str("job_id", jobID).Str("status", status).Msg("job completed")
	return nil
}

func (q *Queue) GetStatus(ctx context.Context, jobID string) (*model.Job, error) {
	return q.jobRepo.GetByID(ctx, jobID)
}
