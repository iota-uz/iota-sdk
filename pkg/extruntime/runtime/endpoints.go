package runtime

import (
	"context"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/endpoint"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/host"
)

// EndpointConfig tunes package HTTP endpoints.
type EndpointConfig struct {
	// PublicTenant resolves the tenant of a request without a session (a
	// webhook), e.g. by host name. Without it public endpoints answer 503
	// unless the application middleware put a tenant in the request context.
	PublicTenant func(r *http.Request) (uuid.UUID, bool)
	// TenantByHost resolves that tenant from the Host header against
	// tenants.domain (TenantByHost). It applies when PublicTenant is nil.
	TenantByHost bool
	MaxBodyBytes int64
	Timeout      time.Duration
}

// EndpointHandler serves /_ext/endpoints/<namespace>/<path> over the active
// package versions, as the session user or, for public endpoints, the trigger
// owner.
func (rt *Runtime) EndpointHandler() (*endpoint.Handler, error) {
	var subjects host.SubjectLoader
	if rt.Subjects != nil {
		subjects = rt.Subjects
	}
	publicTenant := rt.Config.Endpoints.PublicTenant
	if publicTenant == nil && rt.Config.Endpoints.TenantByHost {
		publicTenant = TenantByHost(rt.db)
	}
	return endpoint.NewHandler(endpoint.Config{
		Source:       rt.Manage.EndpointSource(),
		Runner:       rt.Host,
		Owners:       &endpoint.PgOwners{DB: rt.db},
		Subjects:     subjects,
		PublicTenant: publicTenant,
		Origins:      rt.origins,
		MaxBodyBytes: rt.Config.Endpoints.MaxBodyBytes,
		Timeout:      rt.Config.Endpoints.Timeout,
	})
}

const (
	tenantHostTTL      = 30 * time.Second
	tenantHostMaxCache = 1024
)

// TenantByHost resolves the tenant of a request without a session from its
// Host header: the active tenant whose tenants.domain equals the host name
// (port and case ignored). A host that names no tenant, or more than one,
// resolves to nothing, so a public endpoint answers 503 instead of acting in
// a guessed tenant. Answers, including misses, are cached for a short time.
func TenantByHost(db *pgxpool.Pool) func(*http.Request) (uuid.UUID, bool) {
	c := &hostTenants{db: db, entries: map[string]hostTenant{}}
	return func(r *http.Request) (uuid.UUID, bool) {
		host := r.Host
		if h, _, err := net.SplitHostPort(host); err == nil {
			host = h
		}
		host = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(host), "."))
		if host == "" {
			return uuid.Nil, false
		}
		id, err := c.lookup(r.Context(), host)
		if err != nil || id == uuid.Nil {
			return uuid.Nil, false
		}
		return id, true
	}
}

type hostTenant struct {
	id      uuid.UUID
	expires time.Time
}

type hostTenants struct {
	db      *pgxpool.Pool
	mu      sync.Mutex
	entries map[string]hostTenant
}

func (c *hostTenants) lookup(ctx context.Context, host string) (uuid.UUID, error) {
	now := time.Now()
	c.mu.Lock()
	e, ok := c.entries[host]
	c.mu.Unlock()
	if ok && now.Before(e.expires) {
		return e.id, nil
	}
	rows, err := c.db.Query(ctx, `SELECT id FROM tenants WHERE lower(domain) = $1 AND is_active LIMIT 2`, host)
	if err != nil {
		return uuid.Nil, err
	}
	defer rows.Close()
	var ids []uuid.UUID
	for rows.Next() {
		var id uuid.UUID
		if err := rows.Scan(&id); err != nil {
			return uuid.Nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return uuid.Nil, err
	}
	id := uuid.Nil
	if len(ids) == 1 {
		id = ids[0]
	}
	c.mu.Lock()
	if len(c.entries) >= tenantHostMaxCache {
		clear(c.entries)
	}
	c.entries[host] = hostTenant{id: id, expires: now.Add(tenantHostTTL)}
	c.mu.Unlock()
	return id, nil
}
