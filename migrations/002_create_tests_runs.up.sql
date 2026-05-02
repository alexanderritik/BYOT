CREATE TABLE tests_runs (
    uuid UUID PRIMARY KEY,
    test_id UUID REFERENCES tests(uuid),
    status        VARCHAR(50) NOT NULL,
    duration_ms   INT,
    started_at    TIMESTAMP NOT NULL,
    finished_at   TIMESTAMP NOT NULL,
    log_url       TEXT NOT NULL,
    log_size_bytes INT
);