-- Wipes application data (keeps schema_migrations). Safe for local dev.
TRUNCATE TABLE jobs, tests_runs, tests RESTART IDENTITY CASCADE;
