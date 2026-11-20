package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"

	"plata/internal/model"
)

const pollInterval = 500 * time.Millisecond

type Repository interface {
	NextPending(ctx context.Context) (*model.Job, error)
	Complete(ctx context.Context, id uuid.UUID, rate decimal.Decimal, updatedAt time.Time) error
	Fail(ctx context.Context, id uuid.UUID, reason string) error
}

type QuoteProvider interface {
	GetRate(ctx context.Context, base, quote string) (decimal.Decimal, time.Time, error)
}

type Worker struct {
	repo     Repository
	provider QuoteProvider
	logger   *slog.Logger
}

func NewWorker(repo Repository, provider QuoteProvider, logger *slog.Logger) *Worker {
	return &Worker{repo: repo, provider: provider, logger: logger}
}

func (w *Worker) Run(ctx context.Context) {
	for ctx.Err() == nil {
		job, err := w.repo.NextPending(ctx)
		if err == nil && job != nil {
			err = w.process(ctx, *job)
			if err == nil {
				continue
			}
		}

		if err != nil && ctx.Err() == nil {
			w.logger.ErrorContext(ctx, "queue error", "error", err)
		}

		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}
	}
}

func (w *Worker) process(ctx context.Context, job model.Job) error {
	rate, updatedAt, err := w.provider.GetRate(ctx, job.Pair.Base, job.Pair.Quote)
	if ctx.Err() != nil {
		return ctx.Err()
	}

	if err != nil {
		return w.repo.Fail(ctx, job.ID, err.Error())
	}

	return w.repo.Complete(ctx, job.ID, rate, updatedAt)
}
