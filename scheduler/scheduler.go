package scheduler

import (
	"context"
	"time"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/alexanderritik/mini-lambda/queue"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/schedule"
	"github.com/rs/zerolog/log"
)

type Scheduler struct {
	tests        repository.TestRepository
	queue        *queue.Queue
	jobs         *repository.JobRepository
	tickInterval time.Duration
}

func NewScheduler(
	tests repository.TestRepository,
	q *queue.Queue,
	jobs *repository.JobRepository,
	tickInterval time.Duration,
) *Scheduler {
	if tickInterval <= 0 {
		tickInterval = 30 * time.Second
	}
	return &Scheduler{
		tests:        tests,
		queue:        q,
		jobs:         jobs,
		tickInterval: tickInterval,
	}
}

func (s *Scheduler) Start(ctx context.Context) {
	log.Info().Dur("tick_interval", s.tickInterval).Msg("scheduler started")

	ticker := time.NewTicker(s.tickInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Msg("scheduler shutting down")
			return
		case <-ticker.C:
			s.tick(ctx)
		}
	}
}

func (s *Scheduler) tick(ctx context.Context) {
	now := time.Now().UTC()
	due, err := s.tests.ListDueScheduled(ctx, now)
	if err != nil {
		log.Error().Err(err).Msg("scheduler: list due tests failed")
		return
	}

	for _, test := range due {
		s.fire(ctx, test, now)
	}
}

func (s *Scheduler) fire(ctx context.Context, test *model.Test, now time.Time) {
	next, err := schedule.NextRun(test.ScheduleCron, test.ScheduleTimezone, now)
	if err != nil {
		log.Error().Err(err).Str("test_id", test.UUID).Msg("scheduler: invalid cron")
		return
	}

	if err := s.tests.UpdateNextRunAt(ctx, test.UUID, next); err != nil {
		log.Error().Err(err).Str("test_id", test.UUID).Msg("scheduler: update next_run_at failed")
		return
	}

	active, err := s.jobs.HasActiveJob(ctx, test.UUID)
	if err != nil {
		log.Error().Err(err).Str("test_id", test.UUID).Msg("scheduler: active job check failed")
		return
	}
	if active {
		log.Info().Str("test_id", test.UUID).Msg("scheduler: skipped enqueue (run still active)")
		return
	}

	job, err := s.queue.Enqueue(ctx, test.UUID, queue.TriggerScheduled)
	if err != nil {
		log.Error().Err(err).Str("test_id", test.UUID).Msg("scheduler: enqueue failed")
		return
	}

	log.Info().
		Str("test_id", test.UUID).
		Str("job_id", job.UUID).
		Time("next_run_at", next).
		Msg("scheduler: enqueued scheduled run")
}
