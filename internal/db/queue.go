package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/shopspring/decimal"

	"plata/internal/model"
)

// NextPending leaves the row pending while the single worker fetches the rate.
func (r *Repository) NextPending(ctx context.Context) (*model.Job, error) {
	var job model.Job
	err := r.pool.QueryRow(ctx, `
		SELECT id, base_currency, quote_currency
		FROM quote_updates
		WHERE status = 'pending'
		ORDER BY created_at, id
		LIMIT 1`).Scan(&job.ID, &job.Pair.Base, &job.Pair.Quote)

	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read pending update: %w", err)
	}

	return &job, nil
}

func (r *Repository) Complete(ctx context.Context, id uuid.UUID, rate decimal.Decimal, updatedAt time.Time) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE quote_updates
		SET status = 'completed', rate = $2::text::numeric, rate_updated_at = $3,
			completed_at = statement_timestamp(), last_error = NULL
		WHERE id = $1 AND status = 'pending'`, id, rate.String(), updatedAt)
	if err != nil {
		return fmt.Errorf("save quote: %w", err)
	}

	return nil
}

func (r *Repository) Fail(ctx context.Context, id uuid.UUID, reason string) error {
	_, err := r.pool.Exec(ctx, `
		UPDATE quote_updates
		SET status = 'failed', completed_at = statement_timestamp(), last_error = $2
		WHERE id = $1 AND status = 'pending'`, id, reason)
	if err != nil {
		return fmt.Errorf("save update failure: %w", err)
	}

	return nil
}
