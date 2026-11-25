package mcp

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
)

type cacheKey struct {
	tenant, connection uuid.UUID
}

type cacheEntry struct {
	version  int
	identity string
	at       time.Time
	tools    []Tool
}

// toolCache keeps a connection's tools/list for ToolsCacheTTL, per connection version and per the
// identity it was listed as (the token sent): a change of the connection (new settings, a refresh) lists
// again, and a list is never answered to a token other than the one that read it (a server may list
// other tools, with other hints, to another user).
type toolCache struct {
	mu      sync.Mutex
	entries map[cacheKey]cacheEntry
}

func newToolCache() *toolCache { return &toolCache{entries: map[cacheKey]cacheEntry{}} }

func (c *toolCache) get(k cacheKey, version int, identity string, now time.Time) ([]Tool, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.entries[k]
	if !ok || e.version != version || e.identity != identity || now.Sub(e.at) > ToolsCacheTTL {
		return nil, false
	}
	return e.tools, true
}

func (c *toolCache) put(k cacheKey, version int, identity string, now time.Time, tools []Tool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.entries) > 10000 {
		for key, e := range c.entries {
			if now.Sub(e.at) > ToolsCacheTTL {
				delete(c.entries, key)
			}
		}
	}
	c.entries[k] = cacheEntry{version: version, identity: identity, at: now, tools: tools}
}

// listIdentity is who a tools/list is read as: the auth kind and the digest of the token sent.
func listIdentity(auth, token string) string {
	sum := sha256.Sum256([]byte(auth + "\x00" + token))
	return hex.EncodeToString(sum[:])
}

func (c *toolCache) drop(tenant, connection uuid.UUID) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.entries, cacheKey{tenant, connection})
}

// Tools lists the tools of a connected connection (tools/list), reused for ToolsCacheTTL. The caller
// checks the right to see them (Connections.Manage or Automations.Author).
func (c *Connector) Tools(ctx context.Context, tenantID, connectionID uuid.UUID) ([]Tool, error) {
	st, err := c.boundStore()
	if err != nil {
		return nil, err
	}
	conn, creds, err := st.Get(ctx, tenantID, connectionID)
	if err != nil {
		return nil, err
	}
	if conn.Kind != Kind {
		return nil, fmt.Errorf("%w: not an MCP connection", connectors.ErrNotFound)
	}
	if conn.Status != connectors.StatusConnected {
		return nil, connectors.ErrNotConnected
	}
	cf, err := readConfig(creds)
	if err != nil {
		return nil, err
	}
	key := cacheKey{tenantID, connectionID}
	if tools, ok := c.cache.get(key, conn.Version, listIdentity(cf.auth, creds[accessKey(cf.auth)]), c.now()); ok {
		return append([]Tool(nil), tools...), nil
	}
	token, refreshed, err := c.tokenFor(ctx, tenantID, connectionID, cf, false)
	if err != nil {
		return nil, err
	}
	_, tools, err := c.list(ctx, cf, token)
	if refused401(err) && cf.auth == AuthOAuth2 && !refreshed {
		if token, _, err = c.tokenFor(ctx, tenantID, connectionID, cf, true); err != nil {
			return nil, err
		}
		_, tools, err = c.list(ctx, cf, token)
	}
	if refused401(err) && cf.auth == AuthOAuth2 {
		c.markReauth(ctx, tenantID, connectionID, creds, token, "the server refused the access token after a refresh")
	}
	if err != nil {
		return nil, err
	}
	tools = newScrubber(creds, token).tools(tools)
	// The list is the server's the request read: a connection whose server or client changed meanwhile
	// is listed again, never answered (or cached) with another server's tools; nor one whose token is no
	// longer the one sent (a new sign-in, maybe of another user). A refresh made by this request (the
	// same settings, the token it sent) keeps it, under the new version.
	now, nowCreds, err := st.Get(ctx, tenantID, connectionID)
	if err != nil {
		return nil, err
	}
	if now.Version != conn.Version && (!sameSettings(nowCreds, creds) || nowCreds[accessKey(cf.auth)] != token) {
		return nil, &catalog.CallError{Code: catalog.CallUnavailable, Message: "the connection's settings changed while its tools were listed; list again", Retryable: true}
	}
	c.cache.put(key, now.Version, listIdentity(cf.auth, token), c.now(), tools)
	return append([]Tool(nil), tools...), nil
}
