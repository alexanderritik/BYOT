package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/alexanderritik/mini-lambda/alert"
	"github.com/alexanderritik/mini-lambda/bundle"
	"github.com/alexanderritik/mini-lambda/model"
	"github.com/alexanderritik/mini-lambda/queue"
	"github.com/alexanderritik/mini-lambda/repository"
	"github.com/alexanderritik/mini-lambda/runtime"
	"github.com/alexanderritik/mini-lambda/secrets"
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
	envRepo        repository.EnvRepository
	secrets        *secrets.Box
	concurrency    int
	pollInterval   time.Duration
}

func NewWorker(workerID string, q *queue.Queue, testRepo repository.TestRepository, testRunRepo repository.TestRunRepository, screenshotRepo repository.ScreenshotRepository, storage storage.Storage, envRepo repository.EnvRepository, box *secrets.Box, concurrency int) *Worker {
	return &Worker{
		workerID:       workerID,
		queue:          q,
		testRepo:       testRepo,
		testRunRepo:    testRunRepo,
		screenshotRepo: screenshotRepo,
		storage:        storage,
		envRepo:        envRepo,
		secrets:        box,
		concurrency:    concurrency,
		pollInterval:   5 * time.Second,
	}
}

// Start claims and runs jobs, up to w.concurrency at a time, until ctx is
// cancelled. It then waits for in-flight jobs to finish before returning.
func (w *Worker) Start(ctx context.Context) {
	log.Info().Str("worker_id", w.workerID).Int("concurrency", w.concurrency).Msg("worker started")

	go w.sweepStale(ctx)

	runPool(ctx, w.concurrency, w.pollInterval,
		func(ctx context.Context) (*model.Job, error) { return w.queue.Dequeue(ctx, w.workerID) },
		w.handleJob,
	)

	log.Info().Str("worker_id", w.workerID).Msg("worker shut down")
}

const (
	staleGrace      = 5 * time.Minute
	staleSweepEvery = time.Minute
)

// sweepStale fails jobs stuck in 'running' after a crash or restart and removes
// any container they left behind. It runs once at startup, then periodically.
func (w *Worker) sweepStale(ctx context.Context) {
	sweep := func() {
		ids, err := w.queue.FailStale(ctx, staleGrace)
		if err != nil {
			if ctx.Err() == nil {
				log.Error().Err(err).Msg("stale job sweep failed")
			}
			return
		}
		for _, id := range ids {
			runtime.KillContainer(runtime.ContainerName(id))
			log.Warn().Str("job_id", id).Msg("marked stale job as failed")
		}
	}
	sweep()
	t := time.NewTicker(staleSweepEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			sweep()
		}
	}
}

func (w *Worker) handleJob(ctx context.Context, job *model.Job) {
	// A panic in one job must not take down the server or its sibling jobs.
	defer func() {
		if r := recover(); r != nil {
			log.Error().Interface("panic", r).Str("job_id", job.UUID).Msg("job panicked")
			msg := fmt.Sprintf("internal error: %v", r)
			w.queue.Complete(ctx, job.UUID, "failed", &msg)
		}
	}()

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

	if runtime.IsBundle(test.Runtime, test.OriginalFilename) {
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
			return fmt.Errorf("unpack %s bundle: %w", test.Runtime, err)
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
	if err := runtime.StageSupportFiles(test.Runtime, workspace); err != nil {
		return fmt.Errorf("stage %s support files: %w", test.Runtime, err)
	}

	command := test.Command
	if command == "" {
		command = baseSpec.Command
	}
	env, secretValues, err := w.loadEnv(ctx, test.UUID)
	if err != nil {
		return err
	}

	spec := runtime.RuntimeSpec{
		Image:           baseSpec.Image,
		Command:         command,
		NetworkEnabled:  baseSpec.NetworkEnabled,
		MemoryMB:        test.DockerMemoryMB,
		CPUs:            test.DockerCPUs,
		NetworkOverride: test.DockerNetworkEnabled,
		Env:             env,
	}

	timeout := time.Duration(test.TimeoutSeconds) * time.Second
	if test.TimeoutSeconds <= 0 {
		timeout = 30 * time.Second
	}

	// Lets the test budget its own run time (the locust runtime uses these).
	env["BYOT_TIMEOUT_SECONDS"] = strconv.Itoa(int(timeout.Seconds()))
	env["BYOT_STARTED_AT"] = strconv.FormatInt(time.Now().Unix(), 10)

	var preRunImages imageSnapshot
	if test.Runtime == "playwright" {
		preRunImages = snapshotImages(workspace)
	}

	startedAt := time.Now().UTC()
	result, execErr := runtime.Execute(job.UUID, workspace, spec, timeout)
	finishedAt := time.Now().UTC()
	result.Output = secrets.Redact(result.Output, secretValues)
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
		w.uploadScreenshots(ctx, workspace, preRunImages, test.UUID, runID)
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

// loadEnv returns the test's environment (secrets decrypted) and the list of
// secret values to redact from output. A secret that cannot be decrypted fails
// the run rather than running the test with it silently missing.
func (w *Worker) loadEnv(ctx context.Context, testID string) (map[string]string, []string, error) {
	vars, err := w.envRepo.List(ctx, testID)
	if err != nil {
		return nil, nil, fmt.Errorf("load env vars: %w", err)
	}
	env := make(map[string]string, len(vars))
	var secretValues []string
	for _, v := range vars {
		val := v.Value
		if v.IsSecret {
			if val, err = w.secrets.Decrypt(v.Value); err != nil {
				return nil, nil, fmt.Errorf("secret %s: %w", v.Name, err)
			}
			secretValues = append(secretValues, val)
		}
		env[v.Name] = val
	}
	return env, secretValues, nil
}

// uploadScreenshots uploads only images the run produced; images shipped in
// the bundle or under node_modules are ignored.
func (w *Worker) uploadScreenshots(ctx context.Context, workspace string, before imageSnapshot, testUUID, runID string) {
	paths, truncated := newScreenshots(workspace, before)
	if truncated > 0 {
		log.Warn().Str("run_id", runID).Int("skipped", truncated).Int("limit", maxScreenshotsPerRun).Msg("too many screenshots, extra ones not uploaded")
	}
	for _, rel := range paths {
		filePath := filepath.Join(workspace, filepath.FromSlash(rel))
		if err := w.uploadScreenshotFile(ctx, filePath, rel, testUUID, runID); err != nil {
			log.Error().Err(err).Str("file", filePath).Msg("failed to upload screenshot")
		}
	}
}

// uploadScreenshotFile stores one image. relPath is unique within the run
// (Playwright reuses names like test-finished-1.png in every test folder).
func (w *Worker) uploadScreenshotFile(ctx context.Context, filePath, relPath, testUUID, runID string) error {
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

	// Upload to storage: {testUUID}/log/{runUUID}/{relPath}
	storageKey := fmt.Sprintf("%s/log/%s/%s", testUUID, runID, relPath)
	_, err = w.storage.UploadBlob(storageKey, file, fileInfo.Size())
	if err != nil {
		return fmt.Errorf("upload screenshot to storage: %w", err)
	}

	// Create database record
	screenshot := &model.Screenshot{
		UUID:       uuid.NewString(),
		RunID:      runID,
		Filename:   relPath,
		StorageKey: storageKey,
		CreatedAt:  time.Now().UTC(),
	}

	if err := w.screenshotRepo.Create(ctx, screenshot); err != nil {
		return fmt.Errorf("create screenshot record: %w", err)
	}

	log.Info().Str("screenshot_id", screenshot.UUID).Str("filename", relPath).Msg("screenshot uploaded")
	return nil
}
