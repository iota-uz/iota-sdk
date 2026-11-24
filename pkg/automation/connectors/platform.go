package connectors

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// PlatformCredential is one credential from the application's configuration.
type PlatformCredential struct {
	Credentials Credentials
	Identity    Identity
}

type platformCredentials struct {
	dir   Directory
	mu    sync.RWMutex
	creds map[catalog.ConnectorKind]PlatformCredential
}

// PlatformWithdrawer is an optional interface of PlatformCredentials: a credential taken out of service
// while the application runs (NewPlatformCredentials implements it). A credential whose provider account a
// tenant's connection holds is withdrawn at startup, so no tenant sends through an account that is not the
// platform's.
type PlatformWithdrawer interface {
	Withdraw(kind catalog.ConnectorKind)
}

func (p *platformCredentials) Withdraw(kind catalog.ConnectorKind) {
	p.mu.Lock()
	defer p.mu.Unlock()
	delete(p.creds, kind)
}

// NewPlatformCredentials are credentials from configuration by name: a connector kind, offered to
// tenants by the connector's platform policy (dir), or the name of an OAuth2 client credential. Entries
// without credentials are left out.
func NewPlatformCredentials(dir Directory, creds map[catalog.ConnectorKind]PlatformCredential) PlatformCredentials {
	p := &platformCredentials{dir: dir, creds: map[catalog.ConnectorKind]PlatformCredential{}}
	for k, c := range creds {
		if len(c.Credentials) == 0 {
			continue
		}
		cp := make(Credentials, len(c.Credentials))
		for f, v := range c.Credentials {
			cp[f] = v
		}
		p.creds[k] = PlatformCredential{Credentials: cp, Identity: c.Identity}
	}
	return p
}

func (p *platformCredentials) Platform(kind catalog.ConnectorKind) (Credentials, Identity, bool) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	c, ok := p.creds[kind]
	if !ok {
		return nil, Identity{}, false
	}
	cp := make(Credentials, len(c.Credentials))
	for f, v := range c.Credentials {
		cp[f] = v
	}
	return cp, c.Identity, true
}

func (p *platformCredentials) Kinds() []catalog.ConnectorKind {
	p.mu.RLock()
	defer p.mu.RUnlock()
	out := make([]catalog.ConnectorKind, 0, len(p.creds))
	for k := range p.creds {
		if pol := p.Policy(k); pol == PlatformAuto || pol == PlatformOptIn {
			out = append(out, k)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i] < out[j] })
	return out
}

func (p *platformCredentials) Policy(kind catalog.ConnectorKind) PlatformPolicy {
	if p.dir == nil {
		return PlatformNone
	}
	c, ok := p.dir.Get(kind)
	if !ok {
		return PlatformNone
	}
	if pol := c.Descriptor().Platform; pol != "" {
		return pol
	}
	return PlatformNone
}

type grants struct{ db *pgxpool.Pool }

// NewPlatformGrants keeps tenant grants of opt_in platform credentials in automation.platform_grants.
func NewPlatformGrants(db *pgxpool.Pool) PlatformGrants { return &grants{db: db} }

func (g *grants) Enabled(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind) (bool, error) {
	var enabled bool
	err := g.db.QueryRow(ctx, `SELECT enabled FROM automation.platform_grants WHERE tenant_id = $1 AND kind = $2`, tenantID, string(kind)).Scan(&enabled)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, serrors.Wrap("automation.connectors.PlatformGrants.Enabled", err)
	}
	return enabled, nil
}

func (g *grants) SetEnabled(ctx context.Context, tenantID uuid.UUID, kind catalog.ConnectorKind, enabled bool, by int64) error {
	if kind == "" {
		return fmt.Errorf("%w: a grant without a kind", ErrInvalid)
	}
	_, err := g.db.Exec(ctx, `INSERT INTO automation.platform_grants (tenant_id, kind, enabled, enabled_by, enabled_at)
		VALUES ($1, $2, $3, $4, now())
		ON CONFLICT (tenant_id, kind) DO UPDATE SET enabled = EXCLUDED.enabled, enabled_by = EXCLUDED.enabled_by, enabled_at = EXCLUDED.enabled_at`,
		tenantID, string(kind), enabled, nullableUser(by))
	return serrors.Wrap("automation.connectors.PlatformGrants.SetEnabled", err)
}

func nullableUser(id int64) *int64 {
	if id == 0 {
		return nil
	}
	return &id
}

type usageRecorder struct{ db *pgxpool.Pool }

// NewUsageRecorder records spending in automation.connector_usage; platform and tenant usage are told
// apart by the connection ID ("platform:<kind>" or a connection UUID).
func NewUsageRecorder(db *pgxpool.Pool) catalog.UsageRecorder { return &usageRecorder{db: db} }

func (u *usageRecorder) Record(ctx context.Context, e catalog.UsageEntry) error {
	const op = serrors.Op("automation.connectors.UsageRecorder.Record")
	switch {
	case e.TenantID == uuid.Nil || e.Kind == "" || e.ConnectionID == "" || e.Unit == "":
		return serrors.Wrap(op, fmt.Errorf("%w: usage needs a tenant, a kind, a connection and a unit", ErrInvalid))
	case e.Quantity <= 0:
		return serrors.Wrap(op, fmt.Errorf("%w: usage quantity must be positive", ErrInvalid))
	}
	at := e.At
	if at.IsZero() {
		at = time.Now()
	}
	_, err := u.db.Exec(ctx, `INSERT INTO automation.connector_usage (tenant_id, kind, connection_id, unit, quantity, run_id, at)
		VALUES ($1, $2, $3, $4, $5, $6, $7)`, e.TenantID, string(e.Kind), e.ConnectionID, e.Unit, e.Quantity, e.RunID, at)
	return serrors.Wrap(op, err)
}
