ALTER TABLE tests RENAME COLUMN artifact_key TO binary_url;
ALTER TABLE tests DROP COLUMN command;
ALTER TABLE tests DROP COLUMN name;
