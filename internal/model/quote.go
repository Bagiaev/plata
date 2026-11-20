package model

import (
	"time"

	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

type Status string

const (
	StatusPending   Status = "pending"
	StatusCompleted Status = "completed"
	StatusFailed    Status = "failed"
)

type Pair struct {
	Base  string
	Quote string
}

type Update struct {
	ID        uuid.UUID
	Pair      Pair
	Status    Status
	Rate      *decimal.Decimal
	UpdatedAt *time.Time
	CreatedAt time.Time
}

type Quote struct {
	Pair      Pair
	Rate      decimal.Decimal
	UpdatedAt time.Time
}

type Job struct {
	ID   uuid.UUID
	Pair Pair
}
