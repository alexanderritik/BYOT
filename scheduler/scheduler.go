// Package scheduler implements the control-plane cron loop.
//
// It does not execute tests. It only enqueues jobs when a test is due.
// Execution is always handled by worker.Worker (same path as POST /run).
//
// Lifecycle
//
//  1. Upload (handler.UploadBinary): if form field "cron" is set,
//     schedule_enabled=true and next_run_at is computed (UTC, schedule.NextRun).
//
//  2. Start (main): Scheduler runs in a goroutine with a 30s ticker.
//
//  3. tick: ListDueScheduled(now) → tests where next_run_at <= now.
//
//  4. fire (per due test):
//     a. next := NextRun(cron, now); UpdateNextRunAt(test, next)
//     b. if HasActiveJob(test) → skip enqueue (overlap forbid)
//     c. queue.Enqueue(testID, trigger=scheduled)
//
//  5. worker: Dequeue → executeJob → tests_runs + jobs completed/failed.
//
// Manual runs use queue.Enqueue(..., trigger=manual) from handler.Run only.
package scheduler

import (
	"context"
	"sync/atomic"
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
	paused       atomic.Bool
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

func (s *Scheduler) Pause() {
	s.paused.Store(true)
	log.Info().Msg("scheduler paused")
}

func (s *Scheduler) Resume() {
	s.paused.Store(false)
	log.Info().Msg("scheduler resumed")
}

func (s *Scheduler) IsPaused() bool {
	return s.paused.Load()
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
	if s.paused.Load() {
		return
	}
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
	next, err := schedule.NextRun(test.ScheduleCron, now)
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
