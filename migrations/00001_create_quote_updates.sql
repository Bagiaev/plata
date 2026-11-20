-- +goose Up
CREATE TABLE quote_updates (
    id UUID PRIMARY KEY,
    base_currency TEXT NOT NULL CHECK (base_currency ~ '^[A-Z]{3}$'),
    quote_currency TEXT NOT NULL CHECK (quote_currency ~ '^[A-Z]{3}$'),
    status TEXT NOT NULL DEFAULT 'pending'
        CHECK (status IN ('pending', 'processing', 'completed', 'failed')),
    rate NUMERIC CHECK (rate > 0 AND rate < 'Infinity'::numeric),
    rate_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT statement_timestamp(),
    started_at TIMESTAMPTZ,
    completed_at TIMESTAMPTZ,
    attempts INTEGER NOT NULL DEFAULT 0 CHECK (attempts >= 0),
    next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT statement_timestamp(),
    locked_at TIMESTAMPTZ,
    idempotency_key TEXT CHECK (octet_length(idempotency_key) BETWEEN 1 AND 255),
    last_error TEXT,
    CHECK (base_currency <> quote_currency),
    CHECK ((status = 'completed') = (rate IS NOT NULL)),
    CHECK ((status = 'completed') = (rate_updated_at IS NOT NULL)),
    CHECK ((status = 'processing') = (locked_at IS NOT NULL)),
    CHECK ((status IN ('completed', 'failed')) = (completed_at IS NOT NULL))
);

CREATE UNIQUE INDEX quote_updates_idempotency_idx
    ON quote_updates (idempotency_key) WHERE idempotency_key IS NOT NULL;

CREATE INDEX quote_updates_pending_idx
    ON quote_updates (next_attempt_at, created_at, id) WHERE status = 'pending';

CREATE INDEX quote_updates_processing_idx
    ON quote_updates (locked_at) WHERE status = 'processing';

CREATE INDEX quote_updates_latest_idx
    ON quote_updates (base_currency, quote_currency, rate_updated_at DESC, completed_at DESC, id DESC)
    WHERE status = 'completed';

-- +goose Down
DROP TABLE quote_updates;
