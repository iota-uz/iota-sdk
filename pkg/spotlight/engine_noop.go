// Package spotlight provides this package.
package spotlight

import (
	"context"

	"github.com/google/uuid"
)

type NoopEngine struct{}

var _ IndexEngine = (*NoopEngine)(nil)

func NewNoopEngine() *NoopEngine {
	return &NoopEngine{}
}

func (e *NoopEngine) Upsert(context.Context, []SearchDocument) error {
	return nil
}

func (e *NoopEngine) Delete(context.Context, []DocumentRef) error {
	return nil
}

func (e *NoopEngine) DeleteTenant(context.Context, uuid.UUID) error {
	return nil
}

func (e *NoopEngine) Search(context.Context, SearchRequest) ([]SearchHit, error) {
	return nil, nil
}

func (e *NoopEngine) Health(context.Context) error {
	return nil
}
