CREATE TABLE test_env_vars (
    test_id    UUID NOT NULL REFERENCES tests(uuid) ON DELETE CASCADE,
    name       VARCHAR(64) NOT NULL,
    value      TEXT NOT NULL,
    is_secret  BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT NOW(),
    PRIMARY KEY (test_id, name)
);
