package dataset

import (
	"context"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
)

type Fact struct {
	Key           string  `json:"key"`
	Description   string  `json:"description,omitempty"`
	ExpectedValue string  `json:"expected_value"`
	ValueType     string  `json:"value_type,omitempty"`
	Tolerance     float64 `json:"tolerance,omitempty"`
	Normalization string  `json:"normalization,omitempty"`
}

type Dataset interface {
	ID() string
	Seed(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) error
	BuildOracle(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID) (map[string]Fact, error)
}

type Resolver interface {
	Get(datasetID string) (Dataset, error)
}
