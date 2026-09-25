ALTER TABLE jobs DROP COLUMN IF EXISTS trigger;

DROP INDEX IF EXISTS idx_tests_next_run_at;
ALTER TABLE tests DROP COLUMN IF EXISTS next_run_at;
ALTER TABLE tests DROP COLUMN IF EXISTS schedule_timezone;
ALTER TABLE tests DROP COLUMN IF EXISTS schedule_enabled;
ALTER TABLE tests DROP COLUMN IF EXISTS schedule_cron;
