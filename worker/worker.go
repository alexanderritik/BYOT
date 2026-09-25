package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
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
	workerID     string
	queue        *queue.Queue
	testRepo     repository.TestRepository
	testRunRepo  repository.TestRunRepository
	storage      storage.Storage
	pollInterval time.Duration
}

func NewWorker(workerID string, q *queue.Queue, testRepo repository.TestRepository, testRunRepo repository.TestRunRepository, storage storage.Storage) *Worker {
	return &Worker{
		workerID:     workerID,
		queue:        q,
		testRepo:     testRepo,
		testRunRepo:  testRunRepo,
		storage:      storage,
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
		return
	}

	log.Info().Str("worker_id", w.workerID).Str("job_id", job.UUID).Str("test_id", job.TestID).Msg("processing job")

	current, err := w.queue.GetStatus(ctx, job.UUID)
	if err == nil && current.Status == "cancelled" {
		log.Info().Str("job_id", job.UUID).Msg("job already cancelled")
		return
	}

	if err := w.executeJob(ctx, job); err != nil {
		current, _ = w.queue.GetStatus(ctx, job.UUID)
		if current != nil && current.Status == "cancelled" {
			return
		}
		log.Error().Err(err).Str("worker_id", w.workerID).Str("job_id", job.UUID).Msg("job execution failed")
		errorMsg := err.Error()
		w.queue.Complete(ctx, job.UUID, "failed", &errorMsg)
	} else {
		current, _ = w.queue.GetStatus(ctx, job.UUID)
		if current != nil && current.Status == "cancelled" {
			return
		}
		w.queue.Complete(ctx, job.UUID, "completed", nil)
	}
}

func (w *Worker) executeJob(ctx context.Context, job *model.Job) error {
	test, err := w.testRepo.GetByID(ctx, job.TestID)
	if err != nil {
		return err
	}

	artifactKey := test.ArtifactKey
	if artifactKey == "" {
		artifactKey = test.UUID + "/artifact"
	}

	reader, err := w.storage.DownloadBlob(artifactKey)
	if err != nil {
		return err
	}

	workspace, err := os.MkdirTemp("/tmp", "run-"+test.UUID+"-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(workspace)
	if err := os.Chmod(workspace, 0755); err != nil {
		return err
	}

	artifactPath := filepath.Join(workspace, "artifact")
	dst, err := os.OpenFile(artifactPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(dst, reader); err != nil {
		dst.Close()
		return err
	}
	if err := dst.Close(); err != nil {
		return err
	}

	baseSpec, ok := runtime.GetRuntime(test.Runtime)
	if !ok {
		return fmt.Errorf("unsupported runtime: %s", test.Runtime)
	}

	command := test.Command
	if command == "" {
		command = baseSpec.Command
	}
	spec := runtime.RuntimeSpec{
		Image:          baseSpec.Image,
		Command:        command,
		NetworkEnabled: baseSpec.NetworkEnabled,
	}

	timeout := time.Duration(test.TimeoutSeconds) * time.Second
	if test.TimeoutSeconds <= 0 {
		timeout = 30 * time.Second
	}

	startedAt := time.Now().UTC()
	result, execErr := runtime.Execute(job.UUID, workspace, spec, timeout)
	finishedAt := time.Now().UTC()
	duration := finishedAt.Sub(startedAt)

	status := "pass"
	if result.ExitCode != 0 || execErr != nil {
		status = "fail"
	}

	logReader := bytes.NewReader(result.Output)
	logURL, uploadErr := w.storage.UploadLog(test.UUID, logReader, int64(len(result.Output)))
	if uploadErr != nil {
		log.Error().Err(uploadErr).Msg("failed to upload logs")
	}

	testRun := &model.TestRun{
		UUID:         uuid.NewString(),
		TestID:       test.UUID,
		StartedAt:    startedAt,
		DurationMs:   duration.Milliseconds(),
		LogURL:       logURL,
		FinishedAt:   finishedAt,
		Status:       status,
		LogSizeBytes: int64(len(result.Output)),
	}

	if err := w.testRunRepo.Create(ctx, testRun); err != nil {
		return err
	}

	log.Info().
		Str("worker_id", w.workerID).
		Str("job_id", job.UUID).
		Str("status", status).
		Int64("duration_ms", duration.Milliseconds()).
		Msg("job execution completed")

	return execErr
}
