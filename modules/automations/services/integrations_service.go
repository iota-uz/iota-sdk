// Package services holds the business logic of the automations module.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/connectors"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

// OAuthState is a pending OAuth2 authorization: who started it, for which kind and connection name.
type OAuthState struct {
	Hash         []byte
	TenantID     uuid.UUID
	UserID       int64
	Kind         catalog.ConnectorKind
	Name         string
	DisplayName  string
	ConnectionID uuid.UUID // re-authorization of this connection; uuid.Nil: a new one
	ExpiresAt    time.Time
}

// OAuthStates keeps pending OAuth2 states; a state is taken at most once.
type OAuthStates interface {
	Put(ctx context.Context, s OAuthState) error
	// Take removes and returns the state of hash started by the user in the tenant; ErrStateInvalid
	// when there is none (taken, another user's or tenant's) or it expired.
	Take(ctx context.Context, tenantID uuid.UUID, userID int64, hash []byte, now time.Time) (OAuthState, error)
}

// ConnectionChecks keeps when each connection was last checked with its provider.
type ConnectionChecks interface {
	Touch(ctx context.Context, tenantID, connectionID uuid.UUID, at time.Time) error
	LastChecked(ctx context.Context, tenantID uuid.UUID) (map[uuid.UUID]time.Time, error)
}

// Actor is who calls the service: the tenant and the user of the session.
type Actor struct {
	TenantID uuid.UUID
	UserID   int64
}

var (
	// ErrStateInvalid: the OAuth2 state is unknown, taken, of another user or tenant, or expired.
	ErrStateInvalid = errors.New("automations: OAuth2 state is invalid or expired")
	// ErrOAuthUnavailable: OAuth2 is not configured in the application.
	ErrOAuthUnavailable = errors.New("automations: OAuth2 is not configured")
)

// InvalidInputError names the fields the input fails on, by code ("required", "invalid", "name").
// It never carries values.
type InvalidInputError struct {
	Fields map[string]string
}

func (e *InvalidInputError) Error() string {
	names := make([]string, 0, len(e.Fields))
	for k := range e.Fields {
		names = append(names, k)
	}
	sort.Strings(names)
	return "automations: invalid input: " + strings.Join(names, ", ")
}

// TestFailedError is a refusal of the provider: the connector's message, with every credential value
// of the request masked.
type TestFailedError struct {
	Message string
}

func (e *TestFailedError) Error() string { return "automations: connection test failed: " + e.Message }

// PlatformState is the platform credential of a connector as a tenant sees it.
type PlatformState struct {
	Policy           connectors.PlatformPolicy
	Available        bool
	EnabledForTenant bool
}

// ConnectionEntry is a connection with what the page shows next to it.
type ConnectionEntry struct {
	Connection  connectors.Connection
	UsedBy      int
	LastChecked *time.Time
	// Fields are the non-secret credential fields (only with WithFields).
	Fields map[string]string
}

// ConnectorEntry is a registered connector with the tenant's connections.
type ConnectorEntry struct {
	Connector   connectors.Connector
	Platform    *PlatformState // nil: policy none
	Connections []ConnectionEntry
}

// ListOptions configures List.
type ListOptions struct {
	// WithFields adds the non-secret credential fields of each connection (the edit form of an
	// administrator).
	WithFields bool
}

// CreateInput creates a connection.
type CreateInput struct {
	Kind        catalog.ConnectorKind
	Name        string
	DisplayName string
	Credentials map[string]any
}

// UpdateInput changes a connection; Credentials nil keeps them, an empty secret field keeps its value.
type UpdateInput struct {
	ID          uuid.UUID
	Version     int
	DisplayName *string
	Credentials map[string]any
}

// IntegrationsService administers the connections of tenants: it checks credentials with the connector
// before anything is saved and never returns a credential value.
type IntegrationsService struct {
	dir      connectors.Directory
	store    connectors.ConnectionStore
	usage    connectors.UsageIndex
	platform connectors.PlatformCredentials
	grants   connectors.PlatformGrants
	oauth    connectors.OAuth2
	states   OAuthStates
	checks   ConnectionChecks
	stateTTL time.Duration
	now      func() time.Time
}

// IntegrationsDeps are the dependencies of the service; Usage, Platform, Grants and OAuth may be nil.
type IntegrationsDeps struct {
	Directory connectors.Directory
	Store     connectors.ConnectionStore
	Usage     connectors.UsageIndex
	Platform  connectors.PlatformCredentials
	Grants    connectors.PlatformGrants
	OAuth     connectors.OAuth2
	States    OAuthStates
	Checks    ConnectionChecks
	// StateTTL is how long an OAuth2 state is accepted (default 10 minutes).
	StateTTL time.Duration
	Now      func() time.Time
}

// NewIntegrationsService builds the service.
func NewIntegrationsService(d IntegrationsDeps) *IntegrationsService {
	s := &IntegrationsService{
		dir: d.Directory, store: d.Store, usage: d.Usage, platform: d.Platform, grants: d.Grants,
		oauth: d.OAuth, states: d.States, checks: d.Checks, stateTTL: d.StateTTL, now: d.Now,
	}
	if s.stateTTL <= 0 {
		s.stateTTL = 10 * time.Minute
	}
	if s.now == nil {
		s.now = time.Now
	}
	return s
}

// Directory is the connectors the application registered.
func (s *IntegrationsService) Directory() connectors.Directory { return s.dir }

// List is every registered connector with the tenant's connections. Without an encryption key the
// connections are empty: the page still shows the connectors.
func (s *IntegrationsService) List(ctx context.Context, a Actor, opts ListOptions) ([]ConnectorEntry, error) {
	const op = serrors.Op("automations.IntegrationsService.List")
	conns, err := s.store.List(ctx, a.TenantID, "")
	if err != nil && !errors.Is(err, connectors.ErrNotConfigured) {
		return nil, serrors.Wrap(op, err)
	}
	checked := map[uuid.UUID]time.Time{}
	if s.checks != nil && len(conns) > 0 {
		if checked, err = s.checks.LastChecked(ctx, a.TenantID); err != nil {
			return nil, serrors.Wrap(op, err)
		}
	}
	byKind := map[catalog.ConnectorKind][]ConnectionEntry{}
	for _, c := range conns {
		e := ConnectionEntry{Connection: c}
		if e.UsedBy, err = s.usedBy(ctx, a.TenantID, c.ID); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		if at, ok := checked[c.ID]; ok {
			at := at
			e.LastChecked = &at
		}
		if opts.WithFields {
			if conn, ok := s.dir.Get(c.Kind); ok {
				_, creds, err := s.store.Get(ctx, a.TenantID, c.ID)
				if err == nil {
					e.Fields = connectors.RedactedCredentials(conn.Descriptor(), creds)
				} else if !errors.Is(err, connectors.ErrNotConfigured) && !errors.Is(err, connectors.ErrUndecryptable) {
					return nil, serrors.Wrap(op, err)
				}
			}
		}
		byKind[c.Kind] = append(byKind[c.Kind], e)
	}
	list := s.dir.List()
	out := make([]ConnectorEntry, 0, len(list))
	for _, conn := range list {
		entry := ConnectorEntry{Connector: conn, Connections: byKind[conn.Kind()]}
		if entry.Connections == nil {
			entry.Connections = []ConnectionEntry{}
		}
		if entry.Platform, err = s.platformState(ctx, a.TenantID, conn); err != nil {
			return nil, serrors.Wrap(op, err)
		}
		out = append(out, entry)
	}
	return out, nil
}

func (s *IntegrationsService) usedBy(ctx context.Context, tenantID, id uuid.UUID) (int, error) {
	uses, err := s.Usage(ctx, tenantID, id)
	if err != nil {
		return 0, err
	}
	seen := map[uuid.UUID]bool{}
	for _, u := range uses {
		seen[u.AutomationID] = true
	}
	return len(seen), nil
}

// Usage is where a connection is used; empty until the engine provides a UsageIndex.
func (s *IntegrationsService) Usage(ctx context.Context, tenantID, id uuid.UUID) ([]connectors.Use, error) {
	if s.usage == nil {
		return nil, nil
	}
	uses, err := s.usage.Uses(ctx, tenantID, id)
	if err != nil {
		return nil, serrors.Wrap("automations.IntegrationsService.Usage", err)
	}
	return uses, nil
}

// ConnectionUsage is the uses of a connection of the tenant; connectors.ErrNotFound for another
// tenant's or a deleted one.
func (s *IntegrationsService) ConnectionUsage(ctx context.Context, a Actor, id uuid.UUID) ([]connectors.Use, error) {
	if _, err := s.find(ctx, a.TenantID, id); err != nil {
		return nil, err
	}
	return s.Usage(ctx, a.TenantID, id)
}

func (s *IntegrationsService) platformState(ctx context.Context, tenantID uuid.UUID, conn connectors.Connector) (*PlatformState, error) {
	policy := conn.Descriptor().Platform
	if policy != connectors.PlatformAuto && policy != connectors.PlatformOptIn {
		return nil, nil
	}
	st := &PlatformState{Policy: policy}
	if s.platform != nil {
		_, _, st.Available = s.platform.Platform(conn.Kind())
	}
	switch {
	case !st.Available:
	case policy == connectors.PlatformAuto:
		st.EnabledForTenant = true
	case s.grants != nil:
		enabled, err := s.grants.Enabled(ctx, tenantID, conn.Kind())
		if err != nil {
			return nil, err
		}
		st.EnabledForTenant = enabled
	}
	return st, nil
}

// find is a connection of the tenant without its credentials.
func (s *IntegrationsService) find(ctx context.Context, tenantID, id uuid.UUID) (connectors.Connection, error) {
	conns, err := s.store.List(ctx, tenantID, "")
	if err != nil {
		if errors.Is(err, connectors.ErrNotConfigured) {
			return connectors.Connection{}, connectors.ErrNotFound
		}
		return connectors.Connection{}, err
	}
	for _, c := range conns {
		if c.ID == id {
			return c, nil
		}
	}
	return connectors.Connection{}, connectors.ErrNotFound
}

// Create tests the credentials with the connector and saves the connection only when the test passed.
func (s *IntegrationsService) Create(ctx context.Context, a Actor, in CreateInput) (ConnectionEntry, error) {
	const op = serrors.Op("automations.IntegrationsService.Create")
	conn, ok := s.dir.Get(in.Kind)
	if !ok {
		return ConnectionEntry{}, connectors.ErrNotFound
	}
	d := conn.Descriptor()
	fields := map[string]string{}
	name := strings.TrimSpace(in.Name)
	if !connectors.IsName(name) {
		fields["name"] = "name"
	}
	display := strings.TrimSpace(in.DisplayName)
	if len([]rune(display)) > 200 {
		fields["display_name"] = "invalid"
	}
	if d.Auth == connectors.AuthOAuth2 {
		return ConnectionEntry{}, &InvalidInputError{Fields: map[string]string{"credentials": "oauth2"}}
	}
	creds, credFields := toCredentials(in.Credentials)
	for k, v := range credFields {
		fields[k] = v
	}
	for k, v := range checkCredentials(d, creds) {
		if _, set := fields[k]; !set {
			fields[k] = v
		}
	}
	if len(fields) > 0 {
		return ConnectionEntry{}, &InvalidInputError{Fields: fields}
	}
	identity, err := s.test(ctx, conn, creds)
	if err != nil {
		return ConnectionEntry{}, err
	}
	saved, err := s.store.Save(ctx, connectors.Connection{
		TenantID: a.TenantID, Kind: in.Kind, Name: name, DisplayName: display,
		Identity: identity, Status: connectors.StatusConnected, CreatedBy: a.UserID,
	}, creds)
	if err != nil {
		return ConnectionEntry{}, mapStoreError(op, err)
	}
	s.touch(ctx, a.TenantID, saved.ID)
	return s.entry(ctx, a.TenantID, saved, true)
}

// Update changes the display name and the credentials of a connection. New credentials are merged
// over the stored ones, checked and tested before they are saved.
func (s *IntegrationsService) Update(ctx context.Context, a Actor, in UpdateInput) (ConnectionEntry, error) {
	const op = serrors.Op("automations.IntegrationsService.Update")
	cur, stored, err := s.store.Get(ctx, a.TenantID, in.ID)
	if err != nil {
		return ConnectionEntry{}, mapStoreError(op, err)
	}
	if in.Version != cur.Version {
		return ConnectionEntry{}, connectors.ErrVersionConflict
	}
	conn, ok := s.dir.Get(cur.Kind)
	if !ok {
		return ConnectionEntry{}, connectors.ErrNotFound
	}
	d := conn.Descriptor()
	next := cur
	if in.DisplayName != nil {
		next.DisplayName = strings.TrimSpace(*in.DisplayName)
		if len([]rune(next.DisplayName)) > 200 {
			return ConnectionEntry{}, &InvalidInputError{Fields: map[string]string{"display_name": "invalid"}}
		}
	}
	var creds connectors.Credentials
	if in.Credentials != nil {
		if d.Auth == connectors.AuthOAuth2 {
			return ConnectionEntry{}, &InvalidInputError{Fields: map[string]string{"credentials": "oauth2"}}
		}
		given, fields := toCredentials(in.Credentials)
		if len(fields) > 0 {
			return ConnectionEntry{}, &InvalidInputError{Fields: fields}
		}
		creds = mergeCredentials(d, stored, given)
		if fields := checkCredentials(d, creds); len(fields) > 0 {
			return ConnectionEntry{}, &InvalidInputError{Fields: fields}
		}
		identity, err := s.test(ctx, conn, creds, stored)
		if err != nil {
			return ConnectionEntry{}, err
		}
		next.Identity = identity
		if next.Status != connectors.StatusDisabled {
			next.Status, next.StatusReason = connectors.StatusConnected, ""
		}
	}
	saved, err := s.store.Save(ctx, next, creds)
	if err != nil {
		return ConnectionEntry{}, mapStoreError(op, err)
	}
	if creds != nil {
		s.touch(ctx, a.TenantID, saved.ID)
	}
	return s.entry(ctx, a.TenantID, saved, true)
}

// TestResult is the outcome of checking a saved connection.
type TestResult struct {
	OK       bool
	Identity string
	Message  string
	// NeedsReauth: the provider refused the OAuth2 refresh; the user must sign in again.
	NeedsReauth bool
}

// Test checks a saved connection with its provider and records the status: connected or error (a
// disabled connection stays disabled).
func (s *IntegrationsService) Test(ctx context.Context, a Actor, id uuid.UUID) (TestResult, error) {
	const op = serrors.Op("automations.IntegrationsService.Test")
	cur, conn, creds, reauth, err := s.load(ctx, a.TenantID, id)
	if err != nil {
		return TestResult{}, mapStoreError(op, err)
	}
	if reauth {
		if err := s.store.SetStatus(ctx, a.TenantID, id, connectors.StatusNeedsReauth, "re-authorization required"); err != nil {
			return TestResult{}, serrors.Wrap(op, err)
		}
		s.touch(ctx, a.TenantID, id)
		return TestResult{NeedsReauth: true}, nil
	}
	identity, testErr := s.test(ctx, conn, creds)
	next := cur
	var failed *TestFailedError
	switch {
	case testErr == nil:
		next.Identity = identity
		if next.Status != connectors.StatusDisabled {
			next.Status, next.StatusReason = connectors.StatusConnected, ""
		}
	case errors.As(testErr, &failed):
		if next.Status != connectors.StatusDisabled {
			next.Status, next.StatusReason = connectors.StatusError, failed.Message
		}
	default:
		return TestResult{}, serrors.Wrap(op, testErr)
	}
	if _, err := s.store.Save(ctx, next, nil); err != nil {
		return TestResult{}, mapStoreError(op, err)
	}
	s.touch(ctx, a.TenantID, id)
	if failed != nil {
		return TestResult{OK: false, Message: failed.Message}, nil
	}
	return TestResult{OK: true, Identity: identity.Display}, nil
}

// load reads a connection with its connector and credentials; an OAuth2 connection gets a fresh token
// first, and reauth reports that the provider refused it.
func (s *IntegrationsService) load(ctx context.Context, tenantID, id uuid.UUID) (connectors.Connection, connectors.Connector, connectors.Credentials, bool, error) {
	cur, creds, err := s.store.Get(ctx, tenantID, id)
	if err != nil {
		return connectors.Connection{}, nil, nil, false, err
	}
	conn, ok := s.dir.Get(cur.Kind)
	if !ok {
		return connectors.Connection{}, nil, nil, false, connectors.ErrNotFound
	}
	if conn.Descriptor().Auth == connectors.AuthOAuth2 && s.oauth != nil && cur.Status == connectors.StatusConnected {
		if _, err := s.oauth.Token(ctx, tenantID, id); err != nil {
			if errors.Is(err, connectors.ErrNeedsReauth) {
				return cur, conn, nil, true, nil
			}
		} else if cur, creds, err = s.store.Get(ctx, tenantID, id); err != nil {
			return connectors.Connection{}, nil, nil, false, err
		}
	}
	return cur, conn, creds, false, nil
}

// SetDisabled disables a connection (it leaves the catalog of nodes of the tenant) or enables it again:
// enabling checks it with the provider, so it comes back connected or with the error.
func (s *IntegrationsService) SetDisabled(ctx context.Context, a Actor, id uuid.UUID, disabled bool) (ConnectionEntry, error) {
	const op = serrors.Op("automations.IntegrationsService.SetDisabled")
	if disabled {
		if _, err := s.find(ctx, a.TenantID, id); err != nil {
			return ConnectionEntry{}, mapStoreError(op, err)
		}
		if err := s.store.SetStatus(ctx, a.TenantID, id, connectors.StatusDisabled, ""); err != nil {
			return ConnectionEntry{}, mapStoreError(op, err)
		}
	} else {
		cur, conn, creds, _, err := s.load(ctx, a.TenantID, id)
		if err != nil {
			return ConnectionEntry{}, mapStoreError(op, err)
		}
		if cur.Status == connectors.StatusDisabled {
			next := cur
			identity, testErr := s.test(ctx, conn, creds)
			var failed *TestFailedError
			switch {
			case testErr == nil:
				next.Identity, next.Status, next.StatusReason = identity, connectors.StatusConnected, ""
			case errors.As(testErr, &failed):
				next.Status, next.StatusReason = connectors.StatusError, failed.Message
				if conn.Descriptor().Auth == connectors.AuthOAuth2 {
					next.Status = connectors.StatusNeedsReauth
				}
			default:
				return ConnectionEntry{}, serrors.Wrap(op, testErr)
			}
			if _, err := s.store.Save(ctx, next, nil); err != nil {
				return ConnectionEntry{}, mapStoreError(op, err)
			}
			s.touch(ctx, a.TenantID, id)
		}
	}
	cur, err := s.find(ctx, a.TenantID, id)
	if err != nil {
		return ConnectionEntry{}, mapStoreError(op, err)
	}
	return s.entry(ctx, a.TenantID, cur, true)
}

// Delete removes a connection nothing uses; a connection in use is a *connectors.InUseError.
func (s *IntegrationsService) Delete(ctx context.Context, a Actor, id uuid.UUID) error {
	if err := s.store.Delete(ctx, a.TenantID, id); err != nil {
		return mapStoreError("automations.IntegrationsService.Delete", err)
	}
	return nil
}

// SetPlatformEnabled grants or revokes an opt_in platform credential for the tenant.
func (s *IntegrationsService) SetPlatformEnabled(ctx context.Context, a Actor, kind catalog.ConnectorKind, enabled bool) error {
	conn, ok := s.dir.Get(kind)
	if !ok {
		return connectors.ErrNotFound
	}
	st, err := s.platformState(ctx, a.TenantID, conn)
	if err != nil {
		return serrors.Wrap("automations.IntegrationsService.SetPlatformEnabled", err)
	}
	if st == nil || st.Policy != connectors.PlatformOptIn || !st.Available || s.grants == nil {
		return &InvalidInputError{Fields: map[string]string{"kind": "platform"}}
	}
	return serrors.Wrap("automations.IntegrationsService.SetPlatformEnabled", s.grants.SetEnabled(ctx, a.TenantID, kind, enabled, a.UserID))
}

// OAuthStart begins an OAuth2 authorization for a new connection (name) or the re-authorization of
// connectionID; the state is kept for this user and tenant only.
func (s *IntegrationsService) OAuthStart(ctx context.Context, a Actor, kind catalog.ConnectorKind, name string, connectionID uuid.UUID, redirectURL string) (string, error) {
	const op = serrors.Op("automations.IntegrationsService.OAuthStart")
	conn, ok := s.dir.Get(kind)
	if !ok {
		return "", connectors.ErrNotFound
	}
	if conn.Descriptor().Auth != connectors.AuthOAuth2 {
		return "", &InvalidInputError{Fields: map[string]string{"kind": "oauth2"}}
	}
	if s.oauth == nil || s.states == nil {
		return "", ErrOAuthUnavailable
	}
	name = strings.TrimSpace(name)
	if name != "" && !connectors.IsName(name) {
		return "", &InvalidInputError{Fields: map[string]string{"name": "name"}}
	}
	if connectionID != uuid.Nil {
		cur, err := s.find(ctx, a.TenantID, connectionID)
		if err != nil {
			return "", mapStoreError(op, err)
		}
		if cur.Kind != kind {
			return "", connectors.ErrNotFound
		}
	}
	authURL, state, err := s.oauth.Begin(ctx, a.TenantID, kind, redirectURL)
	if err != nil {
		if errors.Is(err, connectors.ErrNotConfigured) {
			return "", ErrOAuthUnavailable
		}
		return "", serrors.Wrap(op, err)
	}
	if err := s.states.Put(ctx, OAuthState{
		Hash: StateHash(state), TenantID: a.TenantID, UserID: a.UserID, Kind: kind, Name: name,
		ConnectionID: connectionID, ExpiresAt: s.now().Add(s.stateTTL),
	}); err != nil {
		return "", serrors.Wrap(op, err)
	}
	return authURL, nil
}

// OAuthComplete finishes an authorization the same user started in the same tenant: the state is
// accepted once. A new connection takes the name given at the start when it is free.
func (s *IntegrationsService) OAuthComplete(ctx context.Context, a Actor, state, code string) (connectors.Connection, error) {
	const op = serrors.Op("automations.IntegrationsService.OAuthComplete")
	if s.oauth == nil || s.states == nil {
		return connectors.Connection{}, ErrOAuthUnavailable
	}
	if state == "" || code == "" {
		return connectors.Connection{}, ErrStateInvalid
	}
	pending, err := s.states.Take(ctx, a.TenantID, a.UserID, StateHash(state), s.now())
	if err != nil {
		return connectors.Connection{}, err
	}
	saved, err := s.oauth.Complete(ctx, a.TenantID, state, code)
	if err != nil {
		if errors.Is(err, connectors.ErrInvalid) {
			return connectors.Connection{}, ErrStateInvalid
		}
		return connectors.Connection{}, serrors.Wrap(op, err)
	}
	if saved.Kind != pending.Kind {
		return connectors.Connection{}, ErrStateInvalid
	}
	s.touch(ctx, a.TenantID, saved.ID)
	if pending.ConnectionID == uuid.Nil && saved.Version == 1 && pending.Name != "" && pending.Name != saved.Name {
		renamed := saved
		renamed.Name = pending.Name
		if out, err := s.store.Save(ctx, renamed, nil); err == nil {
			saved = out
		} else if !errors.Is(err, connectors.ErrNameTaken) {
			return connectors.Connection{}, serrors.Wrap(op, err)
		}
	}
	return saved, nil
}

// OAuthAbandon drops the state of an authorization the provider refused (the user cancelled) and
// returns its kind, so the page reopens that connector's form.
func (s *IntegrationsService) OAuthAbandon(ctx context.Context, a Actor, state string) (catalog.ConnectorKind, error) {
	if s.states == nil || state == "" {
		return "", ErrStateInvalid
	}
	pending, err := s.states.Take(ctx, a.TenantID, a.UserID, StateHash(state), s.now())
	if err != nil {
		return "", err
	}
	return pending.Kind, nil
}

// StateHash is the stored form of an OAuth2 state.
func StateHash(state string) []byte {
	sum := sha256.Sum256([]byte(state))
	return sum[:]
}

func (s *IntegrationsService) entry(ctx context.Context, tenantID uuid.UUID, c connectors.Connection, withFields bool) (ConnectionEntry, error) {
	e := ConnectionEntry{Connection: c}
	var err error
	if e.UsedBy, err = s.usedBy(ctx, tenantID, c.ID); err != nil {
		return ConnectionEntry{}, err
	}
	if s.checks != nil {
		checked, err := s.checks.LastChecked(ctx, tenantID)
		if err != nil {
			return ConnectionEntry{}, err
		}
		if at, ok := checked[c.ID]; ok {
			e.LastChecked = &at
		}
	}
	if withFields {
		if conn, ok := s.dir.Get(c.Kind); ok {
			if _, creds, err := s.store.Get(ctx, tenantID, c.ID); err == nil {
				e.Fields = connectors.RedactedCredentials(conn.Descriptor(), creds)
			}
		}
	}
	return e, nil
}

func (s *IntegrationsService) touch(ctx context.Context, tenantID, id uuid.UUID) {
	if s.checks != nil {
		// The time of the last check is informational: failing to record it does not fail the action.
		_ = s.checks.Touch(ctx, tenantID, id, s.now())
	}
}

// test runs Connector.Test. A refusal is a *TestFailedError whose message has every credential value
// masked (of creds and of extra, the stored credentials of an update).
func (s *IntegrationsService) test(ctx context.Context, conn connectors.Connector, creds connectors.Credentials, extra ...connectors.Credentials) (connectors.Identity, error) {
	identity, err := conn.Test(ctx, creds)
	if err == nil {
		identity.Display = mask(identity.Display, creds, extra...)
		identity.External = mask(identity.External, creds, extra...)
		return identity, nil
	}
	if ctx.Err() != nil {
		return connectors.Identity{}, ctx.Err()
	}
	return connectors.Identity{}, &TestFailedError{Message: mask(err.Error(), creds, extra...)}
}

// mask replaces every credential value of at least four characters in s.
func mask(s string, creds connectors.Credentials, extra ...connectors.Credentials) string {
	all := append([]connectors.Credentials{creds}, extra...)
	values := []string{}
	for _, c := range all {
		for _, v := range c {
			if len(v) >= 4 {
				values = append(values, v)
			}
		}
	}
	// Longer values first: one may contain another.
	sort.Slice(values, func(i, j int) bool { return len(values[i]) > len(values[j]) })
	for _, v := range values {
		s = strings.ReplaceAll(s, v, "•••")
	}
	return s
}

// toCredentials converts JSON values of a form to credential strings; a nested value is invalid.
func toCredentials(in map[string]any) (connectors.Credentials, map[string]string) {
	out := connectors.Credentials{}
	fields := map[string]string{}
	for k, v := range in {
		switch x := v.(type) {
		case nil:
		case string:
			out[k] = x
		case bool:
			out[k] = strconv.FormatBool(x)
		case float64:
			out[k] = strconv.FormatFloat(x, 'f', -1, 64)
		case json.Number:
			out[k] = x.String()
		default:
			fields[k] = "invalid"
		}
	}
	return out, fields
}

// mergeCredentials lays given over stored: an empty secret field keeps its stored value, an empty
// plain field is removed, a field the schema does not declare is dropped.
func mergeCredentials(d connectors.Descriptor, stored, given connectors.Credentials) connectors.Credentials {
	secret := connectors.SecretFields(d)
	declared := declaredFields(d)
	out := connectors.Credentials{}
	for k, v := range stored {
		out[k] = v
	}
	for k, v := range given {
		if declared != nil && !declared[k] {
			continue
		}
		switch {
		case v == "" && secret[k]:
		case v == "":
			delete(out, k)
		default:
			out[k] = v
		}
	}
	return out
}

func declaredFields(d connectors.Descriptor) map[string]bool {
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if len(d.Credentials) == 0 || json.Unmarshal(d.Credentials, &doc) != nil || doc.Properties == nil {
		return nil
	}
	out := make(map[string]bool, len(doc.Properties))
	for k := range doc.Properties {
		out[k] = true
	}
	return out
}

// checkCredentials names the fields the credentials fail the connector's schema on.
func checkCredentials(d connectors.Descriptor, c connectors.Credentials) map[string]string {
	fields := map[string]string{}
	if len(d.Credentials) == 0 {
		return fields
	}
	var doc struct {
		Required []string `json:"required"`
	}
	_ = json.Unmarshal(d.Credentials, &doc)
	for _, r := range doc.Required {
		if strings.TrimSpace(c[r]) == "" {
			fields[r] = "required"
		}
	}
	if len(fields) > 0 {
		return fields
	}
	if err := connectors.ValidateCredentials(d, c); err != nil {
		// The error names the fields as "check [a b]"; values are never part of it.
		msg := err.Error()
		if i := strings.LastIndex(msg, "["); i >= 0 {
			for _, f := range strings.Fields(strings.Trim(msg[i:], "[]")) {
				fields[f] = "invalid"
			}
		}
		if len(fields) == 0 {
			fields["credentials"] = "invalid"
		}
	}
	return fields
}

func mapStoreError(op serrors.Op, err error) error {
	switch {
	case errors.Is(err, connectors.ErrNotFound), errors.Is(err, connectors.ErrNameTaken),
		errors.Is(err, connectors.ErrVersionConflict), errors.Is(err, connectors.ErrConnectionInUse),
		errors.Is(err, connectors.ErrNotConfigured):
		return err
	}
	var invalid *InvalidInputError
	if errors.As(err, &invalid) {
		return err
	}
	return serrors.Wrap(op, err)
}
