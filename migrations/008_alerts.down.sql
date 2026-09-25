ALTER TABLE tests DROP COLUMN IF EXISTS alert_active;
ALTER TABLE tests DROP COLUMN IF EXISTS consecutive_failures;
ALTER TABLE tests DROP COLUMN IF EXISTS failure_threshold;
ALTER TABLE tests DROP COLUMN IF EXISTS webhook_url;
