package repository

import (
	"context"
	"time"

	"github.com/alexanderritik/mini-lambda/alert"
	"github.com/alexanderritik/mini-lambda/model"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type TestConfigUpdate struct {
	Command          string
	Severity         string
	TimeoutSeconds   int
	ScheduleCron     string
	ScheduleEnabled  bool
	NextRunAt        *time.Time
	WebhookURL       string
	FailureThreshold int
	AlertsEnabled    bool
}

type TestRepository interface {
	Create(ctx context.Context, test *model.Test) error
	GetByID(ctx context.Context, uuid string) (*model.Test, error)
	ListWithLastRun(ctx context.Context) ([]model.TestListItem, error)
	UpdateConfig(ctx context.Context, testID string, cfg TestConfigUpdate) error
	ListDueScheduled(ctx context.Context, now time.Time) ([]*model.Test, error)
	UpdateNextRunAt(ctx context.Context, testID string, next time.Time) error
	RecordRunOutcome(ctx context.Context, testID string, passed bool) (*model.RunOutcomeAlert, error)
	Delete(ctx context.Context, testID string) error
}

type postgresTestRepository struct {
	pool *pgxpool.Pool
}

func NewTestRepository(pool *pgxpool.Pool) TestRepository {
	return &postgresTestRepository{pool: pool}
}

func (r *postgresTestRepository) Create(ctx context.Context, test *model.Test) error {
	_, err := r.pool.Exec(ctx,
		`INSERT INTO tests (
			uuid, name, runtime, original_filename, severity, command, artifact_key, timeout_seconds,
			schedule_cron, schedule_enabled, next_run_at, webhook_url, failure_threshold, alerts_enabled
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14)`,
		test.UUID,
		test.Name,
		test.Runtime,
		test.OriginalFilename,
		test.Severity,
		test.Command,
		test.ArtifactKey,
		test.TimeoutSeconds,
		nullIfEmpty(test.ScheduleCron),
		test.ScheduleEnabled,
		test.NextRunAt,
		nullIfEmpty(test.WebhookURL),
		normalizeFailureThreshold(test.FailureThreshold),
		test.AlertsEnabled,
	)
	return err
}

func normalizeFailureThreshold(n int) int {
	if n <= 0 {
		return 3
	}
	return n
}

func nullIfEmpty(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func (r *postgresTestRepository) GetByID(ctx context.Context, uuid string) (*model.Test, error) {
	var test model.Test
	err := r.pool.QueryRow(ctx,
		`SELECT uuid::text, COALESCE(name, ''), runtime, original_filename, severity,
		        COALESCE(command, ''), COALESCE(artifact_key, ''), created_at, timeout_seconds,
		        COALESCE(schedule_cron, ''), schedule_enabled, next_run_at,
		        COALESCE(webhook_url, ''), failure_threshold, consecutive_failures, alert_active, alerts_enabled
		 FROM tests WHERE uuid = $1`,
		uuid,
	).Scan(
		&test.UUID,
		&test.Name,
		&test.Runtime,
		&test.OriginalFilename,
		&test.Severity,
		&test.Command,
		&test.ArtifactKey,
		&test.CreatedAt,
		&test.TimeoutSeconds,
		&test.ScheduleCron,
		&test.ScheduleEnabled,
		&test.NextRunAt,
		&test.WebhookURL,
		&test.FailureThreshold,
		&test.ConsecutiveFailures,
		&test.AlertActive,
		&test.AlertsEnabled,
	)
	if err != nil {
		return nil, err
	}
	return &test, nil
}

func (r *postgresTestRepository) ListWithLastRun(ctx context.Context) ([]model.TestListItem, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT t.uuid::text, COALESCE(t.name, ''), t.runtime, t.original_filename, t.severity,
		        COALESCE(t.command, ''), COALESCE(t.artifact_key, ''), t.created_at, t.timeout_seconds,
		        COALESCE(t.schedule_cron, ''), t.schedule_enabled, t.next_run_at,
		        COALESCE(t.webhook_url, ''), t.failure_threshold, t.consecutive_failures, t.alert_active, t.alerts_enabled,
		        lr.status, lr.started_at, lr.duration_ms,
		        EXISTS (
		          SELECT 1 FROM jobs j
		          WHERE j.test_id = t.uuid AND j.status IN ('queued', 'running')
		        )
		 FROM tests t
		 LEFT JOIN LATERAL (
		   SELECT status, started_at, duration_ms
		   FROM tests_runs r
		   WHERE r.test_id = t.uuid
		   ORDER BY started_at DESC
		   LIMIT 1
		 ) lr ON true
		 ORDER BY t.created_at DESC`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []model.TestListItem
	for rows.Next() {
		var test model.Test
		var lastStatus *string
		var lastStarted *time.Time
		var lastDuration *int64
		var activeJob bool
		if err := rows.Scan(
			&test.UUID,
			&test.Name,
			&test.Runtime,
			&test.OriginalFilename,
			&test.Severity,
			&test.Command,
			&test.ArtifactKey,
			&test.CreatedAt,
			&test.TimeoutSeconds,
			&test.ScheduleCron,
			&test.ScheduleEnabled,
			&test.NextRunAt,
			&test.WebhookURL,
			&test.FailureThreshold,
			&test.ConsecutiveFailures,
			&test.AlertActive,
			&test.AlertsEnabled,
			&lastStatus,
			&lastStarted,
			&lastDuration,
			&activeJob,
		); err != nil {
			return nil, err
		}
		item := model.TestListItem{Test: test, ActiveJob: activeJob}
		if lastStatus != nil && lastStarted != nil && lastDuration != nil {
			item.LastRun = &model.LastRunSummary{
				Status:     *lastStatus,
				StartedAt:  *lastStarted,
				DurationMs: *lastDuration,
			}
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *postgresTestRepository) UpdateConfig(ctx context.Context, testID string, cfg TestConfigUpdate) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET
			command = $1,
			severity = $2,
			timeout_seconds = $3,
			schedule_cron = $4,
			schedule_enabled = $5,
			next_run_at = $6,
			webhook_url = $7,
			failure_threshold = $8,
			alerts_enabled = $9
		 WHERE uuid = $10`,
		cfg.Command,
		cfg.Severity,
		cfg.TimeoutSeconds,
		nullIfEmpty(cfg.ScheduleCron),
		cfg.ScheduleEnabled,
		cfg.NextRunAt,
		nullIfEmpty(cfg.WebhookURL),
		normalizeFailureThreshold(cfg.FailureThreshold),
		cfg.AlertsEnabled,
		testID,
	)
	return err
}

func (r *postgresTestRepository) ListDueScheduled(ctx context.Context, now time.Time) ([]*model.Test, error) {
	rows, err := r.pool.Query(ctx,
		`SELECT uuid::text, COALESCE(name, ''), runtime, original_filename, severity,
		        COALESCE(command, ''), COALESCE(artifact_key, ''), created_at, timeout_seconds,
		        COALESCE(schedule_cron, ''), schedule_enabled, next_run_at,
		        COALESCE(webhook_url, ''), failure_threshold, consecutive_failures, alert_active
		 FROM tests
		 WHERE schedule_enabled = true
		   AND schedule_cron IS NOT NULL
		   AND next_run_at IS NOT NULL
		   AND next_run_at <= $1
		 ORDER BY next_run_at ASC
		 LIMIT 100`,
		now,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []*model.Test
	for rows.Next() {
		var test model.Test
		if err := rows.Scan(
			&test.UUID,
			&test.Name,
			&test.Runtime,
			&test.OriginalFilename,
			&test.Severity,
			&test.Command,
			&test.ArtifactKey,
			&test.CreatedAt,
			&test.TimeoutSeconds,
			&test.ScheduleCron,
			&test.ScheduleEnabled,
			&test.NextRunAt,
			&test.WebhookURL,
			&test.FailureThreshold,
			&test.ConsecutiveFailures,
			&test.AlertActive,
		); err != nil {
			return nil, err
		}
		out = append(out, &test)
	}
	return out, rows.Err()
}

func (r *postgresTestRepository) RecordRunOutcome(ctx context.Context, testID string, passed bool) (*model.RunOutcomeAlert, error) {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx)

	var name, severity, webhook string
	var threshold, consecutive int
	var alertActive, alertsEnabled bool
	err = tx.QueryRow(ctx,
		`SELECT COALESCE(name, ''), severity, COALESCE(webhook_url, ''), failure_threshold,
		        consecutive_failures, alert_active, alerts_enabled
		 FROM tests WHERE uuid = $1 FOR UPDATE`,
		testID,
	).Scan(&name, &severity, &webhook, &threshold, &consecutive, &alertActive, &alertsEnabled)
	if err != nil {
		return nil, err
	}

	out := &model.RunOutcomeAlert{
		WebhookURL: webhook,
		TestName:   name,
		Severity:   severity,
		Threshold:  threshold,
	}

	threshold = normalizeFailureThreshold(threshold)

	if passed {
		if alertActive && webhook != "" && alertsEnabled {
			out.SendRecovery = true
		}
		consecutive = 0
		alertActive = false
	} else {
		consecutive++
		out.Consecutive = consecutive
		alertActive = consecutive >= threshold
		if webhook != "" && alertsEnabled && alert.FailureAlertDue(consecutive, threshold) {
			out.SendFailure = true
		}
	}

	_, err = tx.Exec(ctx,
		`UPDATE tests SET consecutive_failures = $1, alert_active = $2 WHERE uuid = $3`,
		consecutive, alertActive, testID,
	)
	if err != nil {
		return nil, err
	}
	if err := tx.Commit(ctx); err != nil {
		return nil, err
	}

	if !out.SendFailure && !out.SendRecovery {
		return nil, nil
	}
	return out, nil
}

func (r *postgresTestRepository) Delete(ctx context.Context, testID string) error {
	tx, err := r.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM tests_runs WHERE test_id = $1`, testID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM jobs WHERE test_id = $1`, testID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `DELETE FROM tests WHERE uuid = $1`, testID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return pgx.ErrNoRows
	}
	return tx.Commit(ctx)
}

func (r *postgresTestRepository) UpdateNextRunAt(ctx context.Context, testID string, next time.Time) error {
	_, err := r.pool.Exec(ctx,
		`UPDATE tests SET next_run_at = $1 WHERE uuid = $2`,
		next.UTC(), testID,
	)
	return err
}
