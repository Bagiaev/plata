-- +goose Up
UPDATE quote_updates
SET status = 'pending', locked_at = NULL
WHERE status = 'processing';

ALTER TABLE quote_updates
    DROP CONSTRAINT quote_updates_status_check,
    DROP COLUMN started_at,
    DROP COLUMN attempts,
    DROP COLUMN next_attempt_at,
    DROP COLUMN locked_at,
    ADD CONSTRAINT quote_updates_status_check
        CHECK (status IN ('pending', 'completed', 'failed'));

CREATE INDEX quote_updates_pending_idx
    ON quote_updates (created_at, id) WHERE status = 'pending';
