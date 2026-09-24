CREATE TABLE jobs (
    uuid UUID PRIMARY KEY,
    test_id UUID NOT NULL REFERENCES tests(uuid),
    status VARCHAR(50) NOT NULL DEFAULT 'queued',
    queued_at TIMESTAMP NOT NULL DEFAULT NOW(),
    started_at TIMESTAMP,
    finished_at TIMESTAMP,
    worker_id TEXT,
    error_message TEXT
);

CREATE INDEX idx_jobs_status ON jobs(status);
CREATE INDEX idx_jobs_queued_at ON jobs(queued_at);
