package mcp

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"testing"

	"code.dny.dev/ssrf"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors/mcp/mcptest"
)

// memStore is an in-memory Store with a lock per connection, as the row lock of the real one.
type memStore struct {
	mu    sync.Mutex
	locks map[uuid.UUID]*sync.Mutex
	conns map[uuid.UUID]connectors.Connection
	creds map[uuid.UUID]connectors.Credentials
	saves int
}

func newMemStore() *memStore {
	return &memStore{locks: map[uuid.UUID]*sync.Mutex{}, conns: map[uuid.UUID]connectors.Connection{}, creds: map[uuid.UUID]connectors.Credentials{}}
}

func (m *memStore) add(tenant uuid.UUID, creds connectors.Credentials) uuid.UUID {
	m.mu.Lock()
	defer m.mu.Unlock()
	id := uuid.New()
	m.conns[id] = connectors.Connection{ID: id, TenantID: tenant, Kind: Kind, Name: "mcp", Status: connectors.StatusConnected, Version: 1}
	m.creds[id] = cloneCreds(creds)
	m.locks[id] = &sync.Mutex{}
	return id
}

func cloneCreds(c connectors.Credentials) connectors.Credentials {
	out := connectors.Credentials{}
	for k, v := range c {
		out[k] = v
	}
	return out
}

func (m *memStore) Get(_ context.Context, tenant, id uuid.UUID) (connectors.Connection, connectors.Credentials, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	c, ok := m.conns[id]
	if !ok || c.TenantID != tenant {
		return connectors.Connection{}, nil, connectors.ErrNotFound
	}
	return c, cloneCreds(m.creds[id]), nil
}

func (m *memStore) status(id uuid.UUID) connectors.ConnectionStatus {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.conns[id].Status
}

func (m *memStore) setCreds(id uuid.UUID, f func(connectors.Credentials)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	f(m.creds[id])
	c := m.conns[id]
	c.Version++
	m.conns[id] = c
}

func (m *memStore) UpdateCredentials(ctx context.Context, tenant, id uuid.UUID, fn func(connectors.Connection, connectors.Credentials) (*connectors.CredentialChange, error)) (connectors.Connection, connectors.Credentials, error) {
	m.mu.Lock()
	lock := m.locks[id]
	m.mu.Unlock()
	if lock == nil {
		return connectors.Connection{}, nil, connectors.ErrNotFound
	}
	lock.Lock()
	defer lock.Unlock()
	c, creds, err := m.Get(ctx, tenant, id)
	if err != nil {
		return connectors.Connection{}, nil, err
	}
	change, err := fn(c, creds)
	if err != nil || change == nil {
		return c, creds, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if change.Credentials != nil {
		m.creds[id] = cloneCreds(change.Credentials)
	}
	if change.Status != "" {
		c.Status, c.StatusReason = change.Status, change.StatusReason
	}
	c.Version++
	m.conns[id] = c
	m.saves++
	return c, cloneCreds(m.creds[id]), nil
}

var testKey = []byte("mcp-test-key")

func (m *memStore) Seal(purpose string, plain []byte) (string, error) {
	mac := hmac.New(sha256.New, testKey)
	mac.Write([]byte(purpose))
	mac.Write(plain)
	return base64.RawURLEncoding.EncodeToString(plain) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (m *memStore) Unseal(purpose, sealed string) ([]byte, error) {
	body, sig, ok := strings.Cut(sealed, ".")
	if !ok {
		return nil, errors.New("bad")
	}
	plain, err := base64.RawURLEncoding.DecodeString(body)
	if err != nil {
		return nil, err
	}
	want, _ := m.Seal(purpose, plain)
	if !hmac.Equal([]byte(want), []byte(body+"."+sig)) {
		return nil, errors.New("bad")
	}
	return plain, nil
}

// withFake lets the connector reach the fake (its TLS certificate, its port, its loopback address) and
// resolves names by table; every other address meets the real guard.
func withFake(srv *mcptest.Server, names map[string]string) Option {
	return func(c *Connector) {
		_, p := srv.Host()
		port, _ := strconv.Atoi(p)
		c.net.ports = []uint16{443, uint16(port)}
		c.net.tls = srv.TLS()
		c.net.tls.ServerName = "example.com" // a name of the fake's certificate, whatever the URL's host
		allowed := srv.Addr
		g := ssrf.New(ssrf.WithPorts(c.net.ports...))
		c.net.guard = func(network, address string) error {
			if address == allowed {
				return nil
			}
			return g.Safe(network, address, nil)
		}
		c.net.resolve = func(ctx context.Context, host string) ([]netip.Addr, error) {
			if ip, ok := names[host]; ok {
				return []netip.Addr{netip.MustParseAddr(ip)}, nil
			}
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
	}
}

func testConnector(t *testing.T, srv *mcptest.Server, st *memStore, names map[string]string) *Connector {
	t.Helper()
	return New(WithStore(st), withFake(srv, names))
}

var tenant = uuid.MustParse("00000000-0000-0000-0000-0000000000aa")

func callInput(id uuid.UUID, creds connectors.Credentials, params map[string]any) catalog.CallInput {
	return catalog.CallInput{
		TenantID: tenant, RunID: uuid.New(), NodeID: "n1", Attempt: 1, IdempotencyKey: "k", Params: params,
		Connection: &catalog.ConnectionInfo{ID: id.String(), Kind: Kind, Name: "mcp"}, Credentials: creds,
	}
}

func callCode(err error) string {
	var ce *catalog.CallError
	if errors.As(err, &ce) {
		return ce.Code
	}
	return ""
}

// has reports whether the cache keeps a list of the connection, for any version or identity.
func (c *toolCache) has(k cacheKey) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	_, ok := c.entries[k]
	return ok
}
