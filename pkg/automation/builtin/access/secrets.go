package access

import (
	"context"
	"errors"
	"fmt"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/secrets"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// CodeSecretForbidden: the run-as subject may not read the tenant's secrets.
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
		v, err := c.tenant.Resolve(ctx, tenantID, name)
		switch {
		case err == nil:
			// Whether the tenant manages the name is known only now; the value goes no further without the permission.
			if !runAs.Has(c.use) {
				return "", fmt.Errorf("%w: %s needs %s", ErrSecretForbidden, name, c.use)
			}
			return v, nil
		case errors.Is(err, secrets.ErrNotFound):
			// Not the tenant's: the application's chain next.
		default:
			return "", err
		}
	}
	if c.fallback == nil {
		return "", fmt.Errorf("%w: %s", ErrSecretMissing, name)
	}
	return c.fallback.Secret(ctx, tenantID, runAs, name)
}
