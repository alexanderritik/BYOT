package worker

import (
	"bytes"
	"context"
	"io"
	"os"
	"time"

	"github.com/alexanderritik/mini-lambda/model"
	"github.com/alexanderritik/mini-lambda/queue"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/runtime"
	"github.com/alexanderritik/mini-lambda/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Worker struct {
	workerID    string
	queue       *queue.Queue
	testRepo    repository.TestRepository
	testRunRepo repository.TestRunRepository
	storage     storage.Storage
	pollInterval time.Duration
}

func NewWorker(workerID string, q *queue.Queue, testRepo repository.TestRepository, testRunRepo repository.TestRunRepository, storage storage.Storage) *Worker {
	return &Worker{
		workerID:    workerID,
		queue:       q,
		testRepo:    testRepo,
		testRunRepo: testRunRepo,
		storage:     storage,
		pollInterval: 5 * time.Second,
	}
}

func (w *Worker) Start(ctx context.Context) {
	log.Info().Str("worker_id", w.workerID).Msg("worker started")

	ticker := time.NewTicker(w.pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Info().Str("worker_id", w.workerID).Msg("worker shutting down")
			return
		case <-ticker.C:
			w.processNextJob(ctx)
		}
	}
}

func (w *Worker) processNextJob(ctx context.Context) {
	job, err := w.queue.Dequeue(ctx, w.workerID)
	if err != nil {
		log.Error().Err(err).Str("worker_id", w.workerID).Msg("failed to dequeue job")
		return
	}

	if job == nil {
		// No jobs available
		return
	}

	log.Info().Str("worker_id", w.workerID).Str("job_id", job.UUID).Str("test_id", job.TestID).Msg("processing job")

	// Execute the job
	if err := w.executeJob(ctx, job); err != nil {
		log.Error().Err(err).Str("worker_id", w.workerID).Str("job_id", job.UUID).Msg("job execution failed")
		errorMsg := err.Error()
		w.queue.Complete(ctx, job.UUID, "failed", &errorMsg)
	} else {
		w.queue.Complete(ctx, job.UUID, "completed", nil)
	}
}

func (w *Worker) executeJob(ctx context.Context, job *model.Job) error {
	// Get test details
	test, err := w.testRepo.GetByID(ctx, job.TestID)
	if err != nil {
		return err
	}

	// Download binary from MinIO
	reader, err := w.storage.DownloadBlob(test.UUID + "/binary")
	if err != nil {
		return err
	}

	// Save to /tmp
	tmpPath := "/tmp/" + test.UUID
	dst, err := os.Create(tmpPath)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, reader); err != nil {
		dst.Close()
		return err
	}
	dst.Close()

	// Make executable + cleanup after
	os.Chmod(tmpPath, 0755)
	defer os.Remove(tmpPath)

	// Get runtime
	rt := runtime.GetRuntime(test.Runtime)
	if rt == nil {
		return err
	}

	// Execute
	start := time.Now()
	output, exitCode, err := rt.Run(test.UUID, test.TimeoutSeconds)
	duration := time.Since(start)

	// Determine status
	status := "pass"
	if exitCode != 0 {
		status = "fail"
	}

	// Upload logs to MinIO
	logReader := bytes.NewReader(output)
	logUrl, err := w.storage.UploadLog(test.UUID, logReader, int64(len(output)))
	if err != nil {
		log.Error().Err(err).Msg("failed to upload logs")
	}

	// Create TestRun
	testRun := &model.TestRun{
		UUID:         uuid.NewString(),
		TestID:       test.UUID,
		StartedAt:    start,
		DurationMs:   duration.Milliseconds(),
		LogURL:       logUrl,
		FinishedAt:   time.Now(),
		Status:       status,
		LogSizeBytes: int64(len(output)),
	}

	if err := w.testRunRepo.Create(ctx, testRun); err != nil {
		return err
	}

	log.Info().Str("worker_id", w.workerID).Str("job_id", job.UUID).Str("status", status).Int64("duration_ms", duration.Milliseconds()).Msg("job execution completed")

	return nil
}
