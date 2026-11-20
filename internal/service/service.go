package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/google/uuid"

	"plata/internal/apperror"
	"plata/internal/model"
)

type Repository interface {
	Create(ctx context.Context, pair model.Pair, key *string) (model.Update, error)
	Get(ctx context.Context, id uuid.UUID) (model.Update, error)
	Latest(ctx context.Context, pair model.Pair) (model.Quote, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, base, quote string, key *string) (model.Update, error) {
	pair, err := currencyPair(base, quote)
	if err != nil {
		return model.Update{}, err
	}
	if key != nil && !validKey(*key) {
		return model.Update{}, apperror.ErrInvalidKey
	}

	update, err := s.repo.Create(ctx, pair, key)
	if err != nil {
		return model.Update{}, fmt.Errorf("create update: %w", err)
	}

	if update.Pair != pair {
		return model.Update{}, apperror.ErrKeyConflict
	}

	return update, nil
}

func (s *Service) Get(ctx context.Context, rawID string) (model.Update, error) {
	id, err := uuid.Parse(rawID)
	if err != nil || len(rawID) != 36 {
		return model.Update{}, apperror.ErrInvalidID
	}

	update, err := s.repo.Get(ctx, id)
	if err != nil {
		return model.Update{}, fmt.Errorf("get update: %w", err)
	}

	return update, nil
}

func (s *Service) Latest(ctx context.Context, base, quote string) (model.Quote, error) {
	pair, err := currencyPair(base, quote)
	if err != nil {
		return model.Quote{}, err
	}

	result, err := s.repo.Latest(ctx, pair)
	if err != nil {
		return model.Quote{}, fmt.Errorf("get latest quote: %w", err)
	}

	return result, nil
}

func currencyPair(base, quote string) (model.Pair, error) {
	base = strings.ToUpper(strings.TrimSpace(base))
	quote = strings.ToUpper(strings.TrimSpace(quote))

	if base == quote || !supportedCurrency(base) || !supportedCurrency(quote) {
		return model.Pair{}, apperror.ErrInvalidCurrency
	}

	return model.Pair{Base: base, Quote: quote}, nil
}

func supportedCurrency(code string) bool {
	return code == "USD" || code == "EUR" || code == "MXN"
}

func validKey(key string) bool {
	if len(key) == 0 || len(key) > 255 {
		return false
	}

	for _, char := range key {
		if char < '!' || char > '~' {
			return false
		}
	}

	return true
}
