package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// CodeSecretForbidden is the code of a node whose run-as subject may not read the tenant's secrets.
const CodeSecretForbidden = "secret_forbidden"

// ErrSecretForbidden reports that the run-as subject lacks the permission to read the tenant's secrets.
var ErrSecretForbidden = errors.New("automation builtin: secret is forbidden")

// ChainSecrets resolves a node's secret name in order:
//
//  1. the tenant's secrets (the built-in «Secret» connector, secrets.Resolver), when the run-as subject
//     holds use; without it the call fails (ErrSecretForbidden) rather than moving on;
//  2. the application's chain next (fallback, usually FromResolver: CONNECTION__FIELD connection secrets,
//     then host.SecretResolver);
//  3. neither knows the name: ErrSecretMissing.
//
// A failure of the tenant's store (secrets.ErrUnavailable) is returned as it is: it never falls back to the
// application's secrets, which would answer with another value than the tenant set.
func ChainSecrets(tenant secrets.Resolver, use string, fallback Secrets) Secrets {
	return chain{tenant: tenant, use: use, fallback: fallback}
}

type chain struct {
	tenant   secrets.Resolver
	use      string
	fallback Secrets
}

func (c chain) Secret(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error) {
	if c.tenant != nil && secrets.IsName(name) {
		// The records first, without the value: the run-as subject is authorized before the value is read
		// (or its use recorded). A name the tenant does not manage goes on to the application's chain.
		owned, err := c.tenant.Has(ctx, tenantID, name)
		if err != nil {
			return "", err
		}
		if owned {
			if !runAs.Has(c.use) {
				return "", fmt.Errorf("%w: %s needs %s", ErrSecretForbidden, name, c.use)
			}
			v, err := c.tenant.Resolve(ctx, tenantID, name)
			if errors.Is(err, secrets.ErrNotFound) {
				// Deleted between the two reads: as if it was never there.
				return c.next(ctx, tenantID, runAs, name)
			}
			return v, err
		}
	}
	return c.next(ctx, tenantID, runAs, name)
}

func (c chain) next(ctx context.Context, tenantID uuid.UUID, runAs spi.Subject, name string) (string, error) {
	if c.fallback == nil {
		return "", fmt.Errorf("%w: %s", ErrSecretMissing, name)
	}
	return c.fallback.Secret(ctx, tenantID, runAs, name)
}
