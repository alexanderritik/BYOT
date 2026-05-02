CREATE TABLE tests (
    uuid UUID PRIMARY KEY,
    runtime TEXT NOT NULL,
    original_filename TEXT NOT NULL,
    severity      VARCHAR DEFAULT 'P2',
    binary_url    TEXT,
    created_at    TIMESTAMP DEFAULT NOW()
);