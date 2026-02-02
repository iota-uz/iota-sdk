package applet

// BuilderOption is a functional option for configuring ContextBuilder
type BuilderOption func(*ContextBuilder)

// TenantNameResolver is a function that resolves a tenant ID to a tenant name.
// This allows applets to inject custom tenant name resolution logic.
//
// Example:
//
//	type MyTenantResolver struct {
//	    service TenantService
//	}
//
//	func (r *MyTenantResolver) ResolveTenantName(tenantID string) (string, error) {
//	    tenant, err := r.service.GetByID(ctx, tenantID)
//	    if err != nil {
//	        return "", err
//	    }
//	    return tenant.Name(), nil
//	}
type TenantNameResolver interface {
	ResolveTenantName(tenantID string) (string, error)
}

// WithTenantNameResolver sets a custom tenant name resolver for the ContextBuilder.
// If not set, uses 3-layer fallback: resolver → database → "Tenant {short-uuid}"
func WithTenantNameResolver(resolver TenantNameResolver) BuilderOption {
	return func(b *ContextBuilder) {
		b.tenantNameResolver = resolver
	}
}

// WithErrorEnricher sets a custom error context enricher for the ContextBuilder.
// If not set, returns minimal error context with empty support email and no debug mode.
func WithErrorEnricher(enricher ErrorContextEnricher) BuilderOption {
	return func(b *ContextBuilder) {
		b.errorEnricher = enricher
	}
}

// WithSessionStore sets a custom session store for reading actual session expiry.
// If not set, uses configured SessionConfig.ExpiryDuration as default.
//
// This allows the applet to display accurate session expiry times instead of
// relying on the configured default duration.
//
// Example:
//
//	sessionStore := &MySessionStore{store: cookieStore}
//	builder := NewContextBuilder(config, bundle, sessionConfig, logger, metrics,
//	    WithSessionStore(sessionStore),
//	)
func WithSessionStore(store SessionStore) BuilderOption {
	return func(b *ContextBuilder) {
		b.sessionStore = store
	}
}
