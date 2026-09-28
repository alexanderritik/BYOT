package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/alexanderritik/mini-lambda/alert"
	"github.com/alexanderritik/mini-lambda/bundle"
	"github.com/alexanderritik/mini-lambda/model"
	"github.com/alexanderritik/mini-lambda/queue"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/runtime"
	"github.com/alexanderritik/mini-lambda/storage"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type Worker struct {
	workerID       string
	queue          *queue.Queue
	testRepo       repository.TestRepository
	testRunRepo    repository.TestRunRepository
	screenshotRepo repository.ScreenshotRepository
	storage        storage.Storage
	pollInterval   time.Duration
}

func NewWorker(workerID string, q *queue.Queue, testRepo repository.TestRepository, testRunRepo repository.TestRunRepository, screenshotRepo repository.ScreenshotRepository, storage storage.Storage) *Worker {
	return &Worker{
		workerID:       workerID,
		queue:          q,
		testRepo:       testRepo,
		testRunRepo:    testRunRepo,
		screenshotRepo: screenshotRepo,
		storage:        storage,
		pollInterval:   5 * time.Second,
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

	if test.Runtime == "playwright" {
		zipPath := filepath.Join(workspace, "bundle.zip")
		dst, err := os.OpenFile(zipPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
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
		if err := bundle.UnzipFile(workspace, zipPath); err != nil {
			return fmt.Errorf("unpack playwright bundle: %w", err)
		}
		_ = os.Remove(zipPath)
	} else {
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
		Image:           baseSpec.Image,
		Command:         command,
		NetworkEnabled:  baseSpec.NetworkEnabled,
		MemoryMB:        test.DockerMemoryMB,
		CPUs:            test.DockerCPUs,
		NetworkOverride: test.DockerNetworkEnabled,
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

	// Generate run ID before uploading log
	runID := uuid.NewString()

	logReader := bytes.NewReader(result.Output)
	logURL, uploadErr := w.storage.UploadLog(test.UUID, runID, logReader, int64(len(result.Output)))
	if uploadErr != nil {
		log.Error().Err(uploadErr).Msg("failed to upload logs")
	}

	testRun := &model.TestRun{
		UUID:         runID,
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

	// Upload screenshots for Playwright runs
	if test.Runtime == "playwright" {
		if err := w.uploadScreenshots(ctx, workspace, test.UUID, runID); err != nil {
			log.Error().Err(err).Str("run_id", runID).Msg("failed to upload screenshots")
		}
	}

	if alertInfo, err := w.testRepo.RecordRunOutcome(ctx, test.UUID, status == "pass"); err != nil {
		log.Error().Err(err).Str("test_id", test.UUID).Msg("failed to update alert state")
	} else if alertInfo != nil {
		payload := alert.Payload{
			TestID:              test.UUID,
			TestName:            alertInfo.TestName,
			ConsecutiveFailures: alertInfo.Consecutive,
			FailureThreshold:    alertInfo.Threshold,
			RunID:               testRun.UUID,
		}
		if alertInfo.SendFailure {
			if err := alert.SendFailure(ctx, alertInfo.WebhookURL, payload); err != nil {
				log.Error().Err(err).Str("test_id", test.UUID).Msg("failure webhook failed")
			} else {
				log.Info().Str("test_id", test.UUID).Int("consecutive", alertInfo.Consecutive).Msg("failure alert sent")
			}
		}
		if alertInfo.SendRecovery {
			if err := alert.SendRecovery(ctx, alertInfo.WebhookURL, payload); err != nil {
				log.Error().Err(err).Str("test_id", test.UUID).Msg("recovery webhook failed")
			} else {
				log.Info().Str("test_id", test.UUID).Msg("recovery alert sent")
			}
		}
	}

	log.Info().
		Str("worker_id", w.workerID).
		Str("job_id", job.UUID).
		Str("status", status).
		Int64("duration_ms", duration.Milliseconds()).
		Msg("job execution completed")

	return execErr
}

func (w *Worker) uploadScreenshots(ctx context.Context, workspace string, testUUID, runID string) error {
	// Scan workspace root and common screenshot directories
	// Users may save screenshots with relative paths that end up in root
	screenshotDirs := []string{
		workspace, // Scan root for images saved with relative paths
		filepath.Join(workspace, "test-results"),
		filepath.Join(workspace, "screenshots"),
		filepath.Join(workspace, "playwright-report"),
	}

	for _, dir := range screenshotDirs {
		if err := w.uploadScreenshotsFromDir(ctx, dir, testUUID, runID); err != nil {
			log.Debug().Err(err).Str("dir", dir).Msg("failed to upload screenshots from directory")
		}
	}
	return nil
}

func (w *Worker) uploadScreenshotsFromDir(ctx context.Context, dir string, testUUID, runID string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist, that's fine
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// Recursively process subdirectories
			subDir := filepath.Join(dir, entry.Name())
			if err := w.uploadScreenshotsFromDir(ctx, subDir, testUUID, runID); err != nil {
				log.Debug().Err(err).Str("dir", subDir).Msg("failed to upload screenshots from subdirectory")
			}
			continue
		}

		// Check if file is an image
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".png" && ext != ".jpg" && ext != ".jpeg" && ext != ".webp" {
			continue
		}

		filePath := filepath.Join(dir, entry.Name())
		if err := w.uploadScreenshotFile(ctx, filePath, entry.Name(), testUUID, runID); err != nil {
			log.Error().Err(err).Str("file", filePath).Msg("failed to upload screenshot")
		}
	}
	return nil
}

func (w *Worker) uploadScreenshotFile(ctx context.Context, filePath, filename, testUUID, runID string) error {
	file, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get file info to determine size
	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	// Upload to storage: {testUUID}/log/{runUUID}/{filename}
	storageKey := fmt.Sprintf("%s/log/%s/%s", testUUID, runID, filename)
	_, err = w.storage.UploadBlob(storageKey, file, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("upload screenshot to storage: %w", err)
	}

	// Create database record
	screenshot := &model.Screenshot{
		UUID:       uuid.NewString(),
		RunID:      runID,
		Filename:   filename,
		StorageKey: storageKey,
		CreatedAt:  time.Now().UTC(),
	}

	if err := w.screenshotRepo.Create(ctx, screenshot); err != nil {
		return fmt.Errorf("create screenshot record: %w", err)
	}

	log.Info().Str("screenshot_id", screenshot.UUID).Str("filename", filename).Msg("screenshot uploaded")
	return nil
}
