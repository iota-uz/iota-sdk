package composables

import (
	"context"
	"errors"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/pkg/constants"
)

var (
	ErrNoTenantIDFound = errors.New("no tenant id found in context")
)

type Tenant struct {
	ID     uuid.UUID
	Name   string
	Domain string
}

func UseTenantID(ctx context.Context) (uuid.UUID, error) {
	t, ok := ctx.Value(constants.TenantIDKey).(uuid.UUID)
	if !ok {
		return uuid.Nil, ErrNoTenantIDFound
	}
	return t, nil
}

func WithTenantID(ctx context.Context, tenantID uuid.UUID) context.Context {
	return context.WithValue(ctx, constants.TenantIDKey, tenantID)
}
