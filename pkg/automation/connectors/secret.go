package connectors

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type secretResolver struct {
	store    ConnectionStore
	dir      Directory
	fallback host.SecretResolver
}

// SecretResolver serves a package secret named "<connection name>.<field>" from the connection of
// that name in the invocation's tenant: only a field the connector's credentials schema marks
// "x-secret" and not "x-authorization-only" (a credential only the connector's own authorization uses),
// only of a connected connection. A name without a dot, or no connection of that name, goes
// to fallback (nil: not found). The host checks that the package declared the secret before asking.
//
// The directory tells which fields are secret; the issue's contract had SecretResolver(store, fallback),
// the directory is needed to honor x-secret.
func SecretResolver(store ConnectionStore, dir Directory, fallback host.SecretResolver) host.SecretResolver {
	return &secretResolver{store: store, dir: dir, fallback: fallback}
}

func (r *secretResolver) Secret(ctx context.Context, sc spi.Scope, name string) (string, error) {
	const op = serrors.Op("automation.connectors.SecretResolver")
	connName, field, ok := strings.Cut(name, ".")
	if !ok || !IsName(connName) || field == "" {
		return r.next(ctx, sc, name)
	}
	conns, err := r.store.List(ctx, sc.Subject.TenantID, "")
	if errors.Is(err, ErrNotConfigured) {
		return r.next(ctx, sc, name)
	}
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	var match []Connection
	for _, c := range conns {
		if c.Name == connName {
			match = append(match, c)
		}
	}
	switch len(match) {
	case 0:
		return r.next(ctx, sc, name)
	case 1:
	default:
		return "", serrors.Wrap(op, fmt.Errorf("%w: several connections are named %s", ErrInvalid, connName))
	}
	c := match[0]
	conn, ok := r.dir.Get(c.Kind)
	if !ok || !SecretFields(conn.Descriptor())[field] || AuthorizationOnlyFields(conn.Descriptor())[field] {
		return "", serrors.Wrap(op, fmt.Errorf("%w: %s is not a secret field of connector %s", ErrNotFound, field, c.Kind))
	}
	// Name, kind and status are checked on the row the credentials come from: List is an earlier
	// snapshot, and the connection may have been renamed or disabled since.
	cur, creds, err := r.store.Get(ctx, sc.Subject.TenantID, c.ID)
	if err != nil {
		return "", serrors.Wrap(op, err)
	}
	if cur.Name != connName || cur.Kind != c.Kind {
		return "", serrors.Wrap(op, fmt.Errorf("%w: connection %s", ErrNotFound, connName))
	}
	if cur.Status != StatusConnected {
		return "", serrors.Wrap(op, fmt.Errorf("%w: %s", ErrNotConnected, connName))
	}
	v, ok := creds[field]
	if !ok {
		return "", serrors.Wrap(op, fmt.Errorf("%w: connection %s has no %s", ErrNotFound, connName, field))
	}
	return v, nil
}

func (r *secretResolver) next(ctx context.Context, sc spi.Scope, name string) (string, error) {
	if r.fallback == nil {
		return "", serrors.Wrap("automation.connectors.SecretResolver", fmt.Errorf("%w: secret %s", ErrNotFound, name))
	}
	return r.fallback.Secret(ctx, sc, name)
}
