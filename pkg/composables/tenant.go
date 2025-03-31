package composables

import (
	"context"
	"errors"

	"github.com/iota-uz/iota-sdk/pkg/constants"
)

var (
	ErrNoTenantFound = errors.New("no tenant found in context")
)

type Tenant struct {
	ID     uint
	Name   string
	Domain string
}

func UseTenant(ctx context.Context) (*Tenant, error) {
	t, ok := ctx.Value(constants.TenantKey).(*Tenant)
	if !ok {
		return nil, ErrNoTenantFound
	}
	return t, nil
}

func MustUseTenant(ctx context.Context) *Tenant {
	t, err := UseTenant(ctx)
	if err != nil {
		panic(err)
	}
	return t
}

func WithTenant(ctx context.Context, tenant *Tenant) context.Context {
	return context.WithValue(ctx, constants.TenantKey, tenant)
}
