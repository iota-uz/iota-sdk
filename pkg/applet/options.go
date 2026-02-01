package applet

import (
	"github.com/sirupsen/logrus"
)

// BuilderOption is a functional option for configuring ContextBuilder
type BuilderOption func(*ContextBuilder)

// WithLogger sets the logger for the ContextBuilder
func WithLogger(logger *logrus.Logger) BuilderOption {
	return func(b *ContextBuilder) {
		b.logger = logger
	}
}

// TenantNameResolver is a function that resolves a tenant ID to a tenant name.
// This allows applets to inject custom tenant name resolution logic.
//
// Example:
//
//	func(ctx context.Context, tenantID uuid.UUID) (string, error) {
//	    tenant, err := tenantService.GetByID(ctx, tenantID)
//	    if err != nil {
//	        return "", err
//	    }
//	    return tenant.Name(), nil
//	}
type TenantNameResolver interface {
	ResolveTenantName(tenantID string) (string, error)
}

// WithTenantNameResolver sets a custom tenant name resolver for the ContextBuilder.
// If not set, defaults to "Default Tenant" for all tenants.
func WithTenantNameResolver(resolver TenantNameResolver) BuilderOption {
	return func(b *ContextBuilder) {
		b.tenantNameResolver = resolver
	}
}
