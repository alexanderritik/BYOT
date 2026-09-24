ALTER TABLE tests ADD COLUMN name TEXT;
ALTER TABLE tests ADD COLUMN command TEXT;
ALTER TABLE tests RENAME COLUMN binary_url TO artifact_key;
