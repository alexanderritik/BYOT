package model

import (
	"time"
)

type Test struct {
	UUID             string    `db:"uuid"`
	OriginalFilename string    `db:"original_filename"`
	Runtime          string    `db:"runtime"`
	Severity         string    `db:"severity"`
	BinaryURL        string    `db:"binary_url"`
	TimeoutSeconds   int       `db:"timeout_seconds"`
	CreatedAt        time.Time `db:"created_at"`
}
type TestRun struct {
	UUID         string    `db:"uuid"`
	TestID       string    `db:"test_id"`
	Status       string    `db:"status"`
	DurationMs   int64     `db:"duration_ms"`
	StartedAt    time.Time `db:"started_at"`
	FinishedAt   time.Time `db:"finished_at"`
	LogURL       string    `db:"log_url"`
	LogSizeBytes int64     `db:"log_size_bytes"`
}
