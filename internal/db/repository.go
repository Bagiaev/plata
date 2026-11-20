package db

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"

	"plata/internal/apperror"
	"plata/internal/model"
)

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

const updateColumns = `id, base_currency, quote_currency, status, rate::text, rate_updated_at, created_at`

func (r *Repository) Create(ctx context.Context, pair model.Pair, key *string) (model.Update, error) {
	update, err := scanUpdate(r.pool.QueryRow(ctx, `
		INSERT INTO quote_updates (id, base_currency, quote_currency, idempotency_key)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (idempotency_key) WHERE idempotency_key IS NOT NULL DO NOTHING
		RETURNING `+updateColumns, uuid.New(), pair.Base, pair.Quote, key))

	if errors.Is(err, pgx.ErrNoRows) && key != nil {
		update, err = scanUpdate(r.pool.QueryRow(ctx, `
			SELECT `+updateColumns+` FROM quote_updates WHERE idempotency_key = $1`, *key))
	}
	if err != nil {
		return model.Update{}, fmt.Errorf("insert or read update: %w", err)
	}

	return update, nil
}

func (r *Repository) Get(ctx context.Context, id uuid.UUID) (model.Update, error) {
	update, err := scanUpdate(r.pool.QueryRow(ctx, `
		SELECT `+updateColumns+` FROM quote_updates WHERE id = $1`, id))

	if errors.Is(err, pgx.ErrNoRows) {
		return model.Update{}, apperror.ErrUpdateNotFound
	}
	if err != nil {
		return model.Update{}, fmt.Errorf("read update: %w", err)
	}

	return update, nil
}

func (r *Repository) Latest(ctx context.Context, pair model.Pair) (model.Quote, error) {
	var rate string
	var updatedAt time.Time

	err := r.pool.QueryRow(ctx, `
		SELECT rate::text, rate_updated_at FROM quote_updates
		WHERE status = 'completed' AND base_currency = $1 AND quote_currency = $2
		ORDER BY rate_updated_at DESC, completed_at DESC, id DESC
		LIMIT 1`, pair.Base, pair.Quote).Scan(&rate, &updatedAt)

	if errors.Is(err, pgx.ErrNoRows) {
		return model.Quote{}, apperror.ErrQuoteNotFound
	}
	if err != nil {
		return model.Quote{}, fmt.Errorf("read latest quote: %w", err)
	}

	value, err := decimal.NewFromString(rate)
	if err != nil {
		return model.Quote{}, fmt.Errorf("parse stored rate: %w", err)
	}

	return model.Quote{Pair: pair, Rate: value, UpdatedAt: updatedAt.UTC()}, nil
}

func scanUpdate(row pgx.Row) (model.Update, error) {
	var update model.Update
	var rate *string

	err := row.Scan(&update.ID, &update.Pair.Base, &update.Pair.Quote, &update.Status,
		&rate, &update.UpdatedAt, &update.CreatedAt)
	if err != nil {
		return model.Update{}, err
	}

	if rate != nil {
		value, err := decimal.NewFromString(*rate)
		if err != nil {
			return model.Update{}, fmt.Errorf("parse stored rate: %w", err)
		}
		update.Rate = &value
	}

	if update.UpdatedAt != nil {
		utc := update.UpdatedAt.UTC()
		update.UpdatedAt = &utc
	}

	update.CreatedAt = update.CreatedAt.UTC()

	return update, nil
}
