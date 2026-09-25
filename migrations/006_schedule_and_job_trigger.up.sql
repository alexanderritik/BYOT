ALTER TABLE tests ADD COLUMN schedule_cron TEXT;
ALTER TABLE tests ADD COLUMN schedule_enabled BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE tests ADD COLUMN schedule_timezone TEXT;
ALTER TABLE tests ADD COLUMN next_run_at TIMESTAMPTZ;

CREATE INDEX idx_tests_next_run_at ON tests (next_run_at)
    WHERE schedule_enabled = true AND schedule_cron IS NOT NULL;

ALTER TABLE jobs ADD COLUMN trigger VARCHAR(20) NOT NULL DEFAULT 'manual';
