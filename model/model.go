package model

import (
	"time"
)

type Test struct {
	UUID             string     `db:"uuid" json:"uuid"`
	Name             string     `db:"name" json:"name"`
	OriginalFilename string     `db:"original_filename" json:"original_filename"`
	Runtime          string     `db:"runtime" json:"runtime"`
	Command          string     `db:"command" json:"command"`
	Severity         string     `db:"severity" json:"severity"`
	ArtifactKey      string     `db:"artifact_key" json:"-"`
	TimeoutSeconds   int        `db:"timeout_seconds" json:"timeout_seconds"`
	ScheduleCron     string     `db:"schedule_cron" json:"schedule_cron"`
	ScheduleEnabled  bool       `db:"schedule_enabled" json:"schedule_enabled"`
	NextRunAt            *time.Time `db:"next_run_at" json:"next_run_at"`
	WebhookURL           string     `db:"webhook_url" json:"webhook_url"`
	FailureThreshold     int        `db:"failure_threshold" json:"failure_threshold"`
	ConsecutiveFailures  int        `db:"consecutive_failures" json:"consecutive_failures"`
	AlertActive          bool       `db:"alert_active" json:"alert_active"`
	AlertsEnabled        bool       `db:"alerts_enabled" json:"alerts_enabled"`
	CreatedAt            time.Time  `db:"created_at" json:"created_at"`
}

// RunOutcomeAlert describes webhook notifications to send after a run completes.
type RunOutcomeAlert struct {
	SendFailure  bool
	SendRecovery bool
	WebhookURL   string
	TestName     string
	Severity     string
	Consecutive  int
	Threshold    int
}

type TestRun struct {
	UUID         string    `db:"uuid" json:"uuid"`
	TestID       string    `db:"test_id" json:"test_id"`
	Status       string    `db:"status" json:"status"`
	DurationMs   int64     `db:"duration_ms" json:"duration_ms"`
	StartedAt    time.Time `db:"started_at" json:"started_at"`
	FinishedAt   time.Time `db:"finished_at" json:"finished_at"`
	LogURL       string    `db:"log_url" json:"-"`
	LogSizeBytes int64     `db:"log_size_bytes" json:"log_size_bytes"`
}

type LastRunSummary struct {
	Status     string    `json:"status"`
	StartedAt  time.Time `json:"started_at"`
	DurationMs int64     `json:"duration_ms"`
}

type TestListItem struct {
	Test      Test            `json:"test"`
	LastRun   *LastRunSummary `json:"last_run"`
	ActiveJob bool            `json:"active_job"`
}

type Job struct {
	UUID         string     `db:"uuid"`
	TestID       string     `db:"test_id"`
	Status       string     `db:"status"`
	Trigger      string     `db:"trigger"`
	QueuedAt     time.Time  `db:"queued_at"`
	StartedAt    *time.Time `db:"started_at"`
	FinishedAt   *time.Time `db:"finished_at"`
	WorkerID     *string    `db:"worker_id"`
	ErrorMessage *string    `db:"error_message"`
}
