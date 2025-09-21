CREATE TABLE IF NOT EXISTS workers (
    id                SERIAL PRIMARY KEY,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    name              TEXT UNIQUE NOT NULL,
    type              TEXT NOT NULL,
    status            TEXT DEFAULT 'online',
    last_seen         TIMESTAMPTZ DEFAULT current_timestamp,
    orders_processed  INTEGER DEFAULT 0
    );

CREATE INDEX IF NOT EXISTS idx_workers_status ON workers(status);
CREATE INDEX IF NOT EXISTS idx_workers_last_seen ON workers(last_seen);