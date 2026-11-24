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
	"github.com/jackc/pgx/v5"

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
	// ErrOAuthRefused: the provider issued a token the connector does not accept.
	ErrOAuthRefused = errors.New("automations: the connector refused the token of the sign-in")
	// ErrOAuthExchange: the provider did not confirm the sign-in (the code exchange failed).
	ErrOAuthExchange = errors.New("automations: the provider did not confirm the sign-in")
	// ErrOAuthProvider: an OAuth2 call (a token, a refresh, the start of a sign-in) failed in the
	// provider or the OAuth2 implementation; its own error is dropped (it may carry provider text).
	ErrOAuthProvider = errors.New("automations: the OAuth2 provider call failed")
)

// RequestState is what a reserved idempotency key says about its request.
type RequestState int

const (
	// RequestNew: the key is this request's now; run it.
	RequestNew RequestState = iota
	// RequestInProgress: a request with the key is running (a retry overlapped it).
	RequestInProgress
	// RequestDone: a request with the key changed ConnectionID; answer with it.
	RequestDone
	// RequestMismatch: the key was used for another payload.
	RequestMismatch
)

// Reservation is the state of an idempotency key. Owner identifies a RequestNew reservation: only
// its holder completes or releases it.
type Reservation struct {
	State        RequestState
	ConnectionID uuid.UUID
	Owner        uuid.UUID
}

// Requests keeps the idempotency keys of the page. A key is reserved before the provider is asked and
// bound to a fingerprint of the payload; Complete runs in the transaction of the save, so the key
// records the connection exactly when the change commits; Release frees a key whose request failed.
type Requests interface {
	// Reserve takes key for a request with fingerprint; same tells whether a stored fingerprint is of
	// the same payload (it may have been made under a key that has since rotated).
	Reserve(ctx context.Context, tenantID uuid.UUID, key, fingerprint string, same func(stored string) bool) (Reservation, error)
	Complete(ctx context.Context, tx pgx.Tx, tenantID uuid.UUID, key string, owner, connectionID uuid.UUID) error
	Release(ctx context.Context, tenantID uuid.UUID, key string, owner uuid.UUID) error
}

// ErrRequestInProgress is a request with the same idempotency key that is still running.
var ErrRequestInProgress = errors.New("automations: a request with this key is still running")

// maxKey bounds an idempotency key.
const maxKey = 100

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

// TestFailedError is a refusal of the provider, by category. The provider's own text is not kept:
// it may carry the credentials in a form no mask anticipates.
type TestFailedError struct {
	Reason Reason
}

func (e *TestFailedError) Error() string {
	return "automations: connection test failed: " + string(e.Reason)
}

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
	// SignIn: the connection signs in through the browser with its own authorization server
	// (connectors.ConnectionAuthorizer); one that needs re-authorization is offered a sign-in.
	SignIn bool
}

// ConnectorEntry is a registered connector with the tenant's connections.
type ConnectorEntry struct {
	Connector   connectors.Connector
	Platform    *PlatformState // nil: policy none
	Connections []ConnectionEntry
	// OAuthPresets are the sign-in presets of the connector the application can use: the provider's OAuth2
	// client is configured (a preset without one is not offered).
	OAuthPresets []string
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
	// IdempotencyKey identifies the intent: a retry with the same key returns the connection the first
	// request created.
	IdempotencyKey string
}

// UpdateInput changes a connection; Credentials nil keeps them, an empty secret field keeps its value.
type UpdateInput struct {
	ID          uuid.UUID
	Version     int
	DisplayName *string
	Credentials map[string]any
	// IdempotencyKey: a retry with the same key returns the connection as the first request left it.
	IdempotencyKey string
}

// IntegrationsService administers the connections of tenants: it checks credentials with the connector
// before anything is saved and never returns a credential value.
type IntegrationsService struct {
	dir      connectors.Directory
	store    connectors.Store
	usage    connectors.UsageIndex
	platform connectors.PlatformCredentials
	grants   connectors.PlatformGrants
	oauth    connectors.OAuth2
	states   OAuthStates
	checks   ConnectionChecks
	requests Requests
	stateTTL time.Duration
	now      func() time.Time
}

// IntegrationsDeps are the dependencies of the service; Usage, Platform, Grants and OAuth may be nil.
type IntegrationsDeps struct {
	Directory connectors.Directory
	Store     connectors.Store
	Usage     connectors.UsageIndex
	Platform  connectors.PlatformCredentials
	Grants    connectors.PlatformGrants
	OAuth     connectors.OAuth2
	States    OAuthStates
	Checks    ConnectionChecks
	Requests  Requests
	// StateTTL is how long an OAuth2 state is accepted (default 10 minutes).
	StateTTL time.Duration
	Now      func() time.Time
}

// NewIntegrationsService builds the service.
func NewIntegrationsService(d IntegrationsDeps) *IntegrationsService {
	s := &IntegrationsService{
		dir: d.Directory, store: d.Store, usage: d.Usage, platform: d.Platform, grants: d.Grants,
		oauth: d.OAuth, states: d.States, checks: d.Checks, requests: d.Requests, stateTTL: d.StateTTL, now: d.Now,
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
		if e, err = s.reveal(ctx, a.TenantID, e, opts.WithFields); err != nil {
			return nil, serrors.Wrap(op, err)
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
		entry.OAuthPresets = s.availablePresets(conn)
		out = append(out, entry)
	}
	return out, nil
}

// availablePresets are the sign-in presets of a connector whose OAuth2 client credential the application has.
func (s *IntegrationsService) availablePresets(conn connectors.Connector) []string {
	d := conn.Descriptor()
	if len(d.OAuth2Presets) == 0 || s.oauth == nil || s.platform == nil || s.states == nil {
		return nil
	}
	if _, ok := s.oauth.(connectors.PresetOAuth2); !ok {
		return nil
	}
	var out []string
	for name, p := range d.OAuth2Presets {
		if client, _, ok := s.platform.Platform(catalog.ConnectorKind(p.ClientCredential)); ok && client[connectors.OAuth2ClientID] != "" {
			out = append(out, name)
		}
	}
	sort.Strings(out)
	return out
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
		return nil, nil //nolint:nilnil // documented contract: nil is «no platform credential» (policy none), not a failure
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
	for k := range connectors.InternalFields(d) {
		delete(creds, k) // the connector writes these itself
		delete(credFields, k)
	}
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
	if _, err := s.store.List(ctx, a.TenantID, in.Kind); err != nil {
		// No keyring: nothing could be saved, so the credentials are not sent to the provider either.
		return ConnectionEntry{}, mapStoreError(op, err)
	}
	req, done, err := s.reserve(ctx, a.TenantID, in.IdempotencyKey, uuid.Nil, "create", string(in.Kind), name, display, creds)
	if err != nil || done != nil {
		return deref(done), err
	}
	saved, err := func() (connectors.Connection, error) {
		if az, ok := conn.(connectors.ConnectionAuthorizer); ok && az.NeedsAuthorization(creds) {
			// Saved before its sign-in: the page sends the browser to the connection's own authorization
			// server next (OAuthStart with the connection), and the callback checks it.
			return s.store.SaveWith(ctx, connectors.Connection{
				TenantID: a.TenantID, Kind: in.Kind, Name: name, DisplayName: display,
				Status: connectors.StatusNeedsReauth, StatusReason: string(ReasonReauth), CreatedBy: a.UserID,
			}, creds, req.complete)
		}
		identity, err := s.test(ctx, conn, creds)
		if err != nil {
			return connectors.Connection{}, err
		}
		return s.store.SaveWith(ctx, connectors.Connection{
			TenantID: a.TenantID, Kind: in.Kind, Name: name, DisplayName: display,
			Identity: identity, Status: connectors.StatusConnected, CreatedBy: a.UserID,
		}, creds, req.complete)
	}()
	if err != nil {
		req.release(ctx)
		var failed *TestFailedError
		if errors.As(err, &failed) {
			return ConnectionEntry{}, err
		}
		return ConnectionEntry{}, mapStoreError(op, err)
	}
	s.touch(ctx, a.TenantID, saved.ID)
	return s.entry(ctx, a.TenantID, saved, true)
}

// request is a reserved idempotency key; the zero value (no key) completes and releases nothing.
type request struct {
	s        *IntegrationsService
	tenantID uuid.UUID
	key      string
	owner    uuid.UUID
}

func (r request) complete(ctx context.Context, tx pgx.Tx, saved connectors.Connection) error {
	if r.key == "" {
		return nil
	}
	return r.s.requests.Complete(ctx, tx, r.tenantID, r.key, r.owner, saved.ID)
}

// releaseTimeout bounds the cleanup of a failed request's key.
const releaseTimeout = 5 * time.Second

func (r request) release(ctx context.Context) {
	if r.key == "" {
		return
	}
	// Even when the request's context is gone the key must not wait out its expiry; the cleanup has its
	// own deadline so that a saturated pool cannot hold the original error back (the key then expires).
	cctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), releaseTimeout)
	defer cancel()
	_ = r.s.requests.Release(cctx, r.tenantID, r.key, r.owner)
}

func deref(e *ConnectionEntry) ConnectionEntry {
	if e == nil {
		return ConnectionEntry{}
	}
	return *e
}

// reserve takes an idempotency key for a request with this payload. A key a finished request used
// answers with that connection as it is now (done); one a running request holds is
// ErrRequestInProgress; one used for another payload is invalid. Without a key nothing is reserved.
func (s *IntegrationsService) reserve(ctx context.Context, tenantID uuid.UUID, key string, target uuid.UUID, payload ...any) (request, *ConnectionEntry, error) {
	key = strings.TrimSpace(key)
	if key == "" || s.requests == nil {
		return request{}, nil, nil
	}
	if len(key) > maxKey {
		return request{}, nil, &InvalidInputError{Fields: map[string]string{"idempotency_key": "invalid"}}
	}
	data, err := json.Marshal(append([]any{target}, payload...))
	if err != nil {
		return request{}, nil, err
	}
	fp, err := s.store.Fingerprint(data)
	if err != nil {
		return request{}, nil, err
	}
	res, err := s.requests.Reserve(ctx, tenantID, key, fp, func(stored string) bool { return s.store.MatchesFingerprint(stored, data) })
	if err != nil {
		return request{}, nil, err
	}
	switch res.State {
	case RequestNew:
		return request{s: s, tenantID: tenantID, key: key, owner: res.Owner}, nil, nil
	case RequestInProgress:
		return request{}, nil, ErrRequestInProgress
	case RequestMismatch:
		return request{}, nil, &InvalidInputError{Fields: map[string]string{"idempotency_key": "invalid"}}
	case RequestDone:
	}
	c, err := s.find(ctx, tenantID, res.ConnectionID)
	if err != nil {
		return request{}, nil, err
	}
	e, err := s.entry(ctx, tenantID, c, true)
	if err != nil {
		return request{}, nil, err
	}
	return request{}, &e, nil
}

// Update changes the display name and the credentials of a connection. New credentials are merged
// over the stored ones, checked and tested before they are saved.
func (s *IntegrationsService) Update(ctx context.Context, a Actor, in UpdateInput) (ConnectionEntry, error) {
	req, done, err := s.reserve(ctx, a.TenantID, in.IdempotencyKey, in.ID, "update", in.Version, in.DisplayName, in.Credentials)
	if err != nil || done != nil {
		return deref(done), err
	}
	entry, err := s.update(ctx, a, in, req)
	if err != nil {
		req.release(ctx)
	}
	return entry, err
}

func (s *IntegrationsService) update(ctx context.Context, a Actor, in UpdateInput, req request) (ConnectionEntry, error) {
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
		if az, ok := conn.(connectors.ConnectionAuthorizer); ok && az.NeedsAuthorization(creds) {
			// New settings need a sign-in before they are checked.
			if next.Status != connectors.StatusDisabled {
				next.Status, next.StatusReason = connectors.StatusNeedsReauth, string(ReasonReauth)
			}
		} else {
			identity, err := s.test(ctx, conn, creds, stored)
			if err != nil {
				return ConnectionEntry{}, err
			}
			next.Identity = identity
			if next.Status != connectors.StatusDisabled {
				next.Status, next.StatusReason = connectors.StatusConnected, ""
			}
		}
	}
	if creds != nil {
		// The retained metadata may carry the credentials being replaced: once they are gone it could no
		// longer be recognised, so it is checked against both.
		scrubRetained(&next, secretsOf(d), stored, creds)
	}
	saved, err := s.store.SaveWith(ctx, next, creds, req.complete)
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
	Reason   Reason
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
		// OAuth2.Token already recorded needs_reauth in its locked transaction (a disabled connection's
		// check records nothing and stays disabled): writing it here could overwrite a disable or
		// re-authorization committed since.
		s.touch(ctx, a.TenantID, id)
		return TestResult{NeedsReauth: true, Reason: ReasonReauth}, nil
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
			next.Status, next.StatusReason = failedStatus(conn, creds, failed.Reason)
		}
	default:
		return TestResult{}, serrors.Wrap(op, testErr)
	}
	if _, err := s.store.Save(ctx, next, nil); err != nil {
		return TestResult{}, mapStoreError(op, err)
	}
	s.touch(ctx, a.TenantID, id)
	if failed != nil {
		return TestResult{OK: false, Reason: failed.Reason}, nil //nolint:nilerr // a refusal of the provider is the check's result, recorded in the status
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
	if az, ok := conn.(connectors.ConnectionAuthorizer); ok && az.SignsIn(creds) {
		return s.loadAuthorized(ctx, az, cur, conn, creds)
	}
	// A connector with a form and a sign-in (an email box) signs in with a preset its credentials name.
	d := conn.Descriptor()
	_, preset := d.OAuth2Presets[creds[connectors.OAuth2Preset]]
	if (d.Auth != connectors.AuthOAuth2 && !preset) || s.oauth == nil {
		return cur, conn, creds, false, nil
	}
	// A connected connection gets its token as a node would; a disabled one (being enabled, or checked)
	// gets it refreshed without a status change, so an expired access token with a valid refresh token
	// does not read as a refused sign-in.
	token := s.oauth.Token
	switch checker, ok := s.oauth.(connectors.OAuth2Checker); {
	case cur.Status == connectors.StatusConnected:
	case cur.Status == connectors.StatusDisabled && ok:
		token = checker.CheckToken
	default:
		token = nil
	}
	if token != nil {
		if _, err := token(ctx, tenantID, id); err != nil {
			switch {
			case errors.Is(err, connectors.ErrNeedsReauth):
				return cur, conn, nil, true, nil
			case errors.Is(err, connectors.ErrNotConnected):
				// The status changed meanwhile: check with the stored credentials.
			default:
				return connectors.Connection{}, nil, nil, false, oauthFailure(err)
			}
		} else if cur, creds, err = s.store.Get(ctx, tenantID, id); err != nil {
			return connectors.Connection{}, nil, nil, false, err
		}
	}
	return cur, conn, creds, false, nil
}

// loadAuthorized is load of a connection that signs in with its own authorization server: a token that
// expires soon is refreshed by the connector (a disabled connection's check records nothing), and one
// that never signed in, or whose refresh was refused, needs a sign-in.
func (s *IntegrationsService) loadAuthorized(ctx context.Context, az connectors.ConnectionAuthorizer, cur connectors.Connection, conn connectors.Connector, creds connectors.Credentials) (connectors.Connection, connectors.Connector, connectors.Credentials, bool, error) {
	if az.NeedsAuthorization(creds) {
		return cur, conn, nil, true, nil
	}
	if cur.Status != connectors.StatusConnected && cur.Status != connectors.StatusDisabled {
		return cur, conn, creds, false, nil
	}
	fresh, err := az.FreshCredentials(ctx, cur.TenantID, cur.ID, cur.Status == connectors.StatusDisabled)
	switch {
	case errors.Is(err, connectors.ErrNeedsReauth):
		return cur, conn, nil, true, nil
	case errors.Is(err, connectors.ErrNotConnected):
		return cur, conn, creds, false, nil // the status changed meanwhile: check with the stored credentials
	case err != nil:
		return connectors.Connection{}, nil, nil, false, oauthFailure(err)
	}
	if cur, _, err = s.store.Get(ctx, cur.TenantID, cur.ID); err != nil {
		return connectors.Connection{}, nil, nil, false, err
	}
	return cur, conn, fresh, false, nil
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
		cur, conn, creds, reauth, err := s.load(ctx, a.TenantID, id)
		if err != nil {
			return ConnectionEntry{}, mapStoreError(op, err)
		}
		if cur.Status == connectors.StatusDisabled {
			next := cur
			var (
				identity connectors.Identity
				testErr  error
			)
			if !reauth {
				identity, testErr = s.test(ctx, conn, creds)
			}
			var failed *TestFailedError
			switch {
			case reauth:
				// The provider refused the refresh token (the check wrote nothing): a sign-in is needed.
				next.Status, next.StatusReason = connectors.StatusNeedsReauth, string(ReasonReauth)
			case testErr == nil:
				next.Identity, next.Status, next.StatusReason = identity, connectors.StatusConnected, ""
			case errors.As(testErr, &failed):
				next.Status, next.StatusReason = failedStatus(conn, creds, failed.Reason)
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
func (s *IntegrationsService) OAuthStart(ctx context.Context, a Actor, kind catalog.ConnectorKind, name, displayName string, connectionID uuid.UUID, redirectURL string) (string, error) {
	return s.OAuthStartPreset(ctx, a, kind, "", name, displayName, connectionID, redirectURL)
}

// OAuthStartPreset is OAuthStart for a connector that has a form and also signs in with a provider (an
// email box: preset gmail or m365). The sign-in makes every connection the connector's finisher names
// (one to send, one to receive) and saves them together.
func (s *IntegrationsService) OAuthStartPreset(ctx context.Context, a Actor, kind catalog.ConnectorKind, preset, name, displayName string, connectionID uuid.UUID, redirectURL string) (string, error) {
	const op = serrors.Op("automations.IntegrationsService.OAuthStart")
	conn, ok := s.dir.Get(kind)
	if !ok {
		return "", connectors.ErrNotFound
	}
	if az, ok := conn.(connectors.ConnectionAuthorizer); ok && preset == "" {
		return s.signInStart(ctx, a, az, kind, connectionID, redirectURL)
	}
	d := conn.Descriptor()
	if preset == "" {
		if d.Auth != connectors.AuthOAuth2 {
			return "", &InvalidInputError{Fields: map[string]string{"kind": "oauth2"}}
		}
	} else if _, known := d.OAuth2Presets[preset]; !known || d.Auth == connectors.AuthOAuth2 {
		return "", &InvalidInputError{Fields: map[string]string{"preset": "oauth2"}}
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
	var (
		authURL, state string
		err            error
	)
	if preset == "" {
		authURL, state, err = s.oauth.Begin(ctx, a.TenantID, kind, redirectURL)
	} else if p, ok := s.oauth.(connectors.PresetOAuth2); ok {
		authURL, state, err = p.BeginPreset(ctx, a.TenantID, kind, preset, redirectURL)
	} else {
		return "", ErrOAuthUnavailable
	}
	if err != nil {
		if errors.Is(err, connectors.ErrNotConfigured) {
			return "", ErrOAuthUnavailable
		}
		return "", serrors.Wrap(op, oauthFailure(err))
	}
	if err := s.states.Put(ctx, OAuthState{
		Hash: StateHash(state), TenantID: a.TenantID, UserID: a.UserID, Kind: kind, Name: name, DisplayName: truncate(strings.TrimSpace(displayName), 200),
		ConnectionID: connectionID, ExpiresAt: s.now().Add(s.stateTTL),
	}); err != nil {
		return "", serrors.Wrap(op, err)
	}
	return authURL, nil
}

// signInStart begins the sign-in of a saved connection with its own authorization server: the connector
// discovers it and registers a client when the connection has none; the state is kept for this user
// and tenant only, as a sign-in of the platform's OAuth2.
func (s *IntegrationsService) signInStart(ctx context.Context, a Actor, az connectors.ConnectionAuthorizer, kind catalog.ConnectorKind, connectionID uuid.UUID, redirectURL string) (string, error) {
	const op = serrors.Op("automations.IntegrationsService.OAuthStart")
	if s.states == nil {
		return "", ErrOAuthUnavailable
	}
	if connectionID == uuid.Nil {
		// The settings come first: the connection is saved, then signed in.
		return "", &InvalidInputError{Fields: map[string]string{"connection_id": "required"}}
	}
	cur, creds, err := s.store.Get(ctx, a.TenantID, connectionID)
	if err != nil {
		return "", mapStoreError(op, err)
	}
	if cur.Kind != kind {
		return "", connectors.ErrNotFound
	}
	if !az.SignsIn(creds) {
		return "", &InvalidInputError{Fields: map[string]string{"auth": "oauth2"}}
	}
	authURL, state, err := az.BeginAuthorization(ctx, connectors.AuthorizationStart{TenantID: a.TenantID, ConnectionID: connectionID, RedirectURL: redirectURL})
	switch {
	case errors.Is(err, connectors.ErrClientRequired):
		return "", &InvalidInputError{Fields: map[string]string{"oauth_client_id": "required"}}
	case err != nil:
		return "", serrors.Wrap(op, oauthFailure(err))
	}
	if err := s.states.Put(ctx, OAuthState{
		Hash: StateHash(state), TenantID: a.TenantID, UserID: a.UserID, Kind: kind, Name: cur.Name, DisplayName: cur.DisplayName,
		ConnectionID: connectionID, ExpiresAt: s.now().Add(s.stateTTL),
	}); err != nil {
		return "", serrors.Wrap(op, err)
	}
	return authURL, nil
}

// signInComplete finishes the sign-in of a connection with its own authorization server: the connector
// exchanges the code and stores the tokens, the connection is checked with them and saved connected (a
// disabled one stays disabled).
func (s *IntegrationsService) signInComplete(ctx context.Context, a Actor, az connectors.ConnectionAuthorizer, conn connectors.Connector, pending OAuthState, state, code string) (connectors.Connection, OAuthState, error) {
	const op = serrors.Op("automations.IntegrationsService.OAuthComplete")
	if pending.ConnectionID == uuid.Nil {
		return connectors.Connection{}, OAuthState{}, ErrStateInvalid
	}
	creds, err := az.CompleteAuthorization(ctx, connectors.AuthorizationReturn{TenantID: a.TenantID, ConnectionID: pending.ConnectionID, State: state, Code: code})
	switch {
	case errors.Is(err, connectors.ErrInvalid):
		return connectors.Connection{}, OAuthState{}, ErrStateInvalid
	case errors.Is(err, connectors.ErrNotFound):
		return connectors.Connection{}, pending, connectors.ErrNotFound
	case err != nil:
		return connectors.Connection{}, pending, serrors.Wrap(op, ErrOAuthExchange)
	}
	identity, err := s.test(ctx, conn, creds)
	if err != nil {
		return connectors.Connection{}, pending, ErrOAuthRefused
	}
	cur, old, err := s.store.Get(ctx, a.TenantID, pending.ConnectionID)
	if err != nil {
		return connectors.Connection{}, pending, mapStoreError(op, err)
	}
	// The status confirms the credentials just tested: a row changed since (another sign-in, new
	// settings) is not marked connected by this check. The save below is versioned against this read.
	if !sameCredentials(old, creds) {
		return connectors.Connection{}, pending, serrors.Wrap(op, ErrOAuthExchange)
	}
	secret := secretsOf(conn.Descriptor())
	cur.Identity = validIdentity(identity, secret, creds, old)
	scrubRetained(&cur, secret, old, creds)
	if cur.Status != connectors.StatusDisabled {
		cur.Status, cur.StatusReason = connectors.StatusConnected, ""
	}
	saved, err := s.store.Save(ctx, cur, nil)
	if err != nil {
		return connectors.Connection{}, pending, mapStoreError(op, err)
	}
	s.touch(ctx, a.TenantID, saved.ID)
	return saved, pending, nil
}

// sameCredentials reports whether two credential sets are the same.
func sameCredentials(a, b connectors.Credentials) bool {
	if len(a) != len(b) {
		return false
	}
	for k, v := range a {
		if bv, ok := b[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// OAuthComplete finishes an authorization the same user started in the same tenant: the state is
// accepted once. The code is exchanged and tested first; then exactly one row is saved — the connection
// being re-authorized (its version checked, a disabled one stays disabled), or a new connection under
// the requested name (connectors.ErrNameTaken leaves nothing behind). The pending state is returned
// with any error but ErrStateInvalid, so the page reopens the form it came from.
func (s *IntegrationsService) OAuthComplete(ctx context.Context, a Actor, state, code string) (connectors.Connection, OAuthState, error) {
	const op = serrors.Op("automations.IntegrationsService.OAuthComplete")
	if s.states == nil {
		return connectors.Connection{}, OAuthState{}, ErrOAuthUnavailable
	}
	if state == "" || code == "" {
		return connectors.Connection{}, OAuthState{}, ErrStateInvalid
	}
	pending, err := s.states.Take(ctx, a.TenantID, a.UserID, StateHash(state), s.now())
	if err != nil {
		return connectors.Connection{}, OAuthState{}, err
	}
	if conn, ok := s.dir.Get(pending.Kind); ok {
		if az, ok := conn.(connectors.ConnectionAuthorizer); ok {
			return s.signInComplete(ctx, a, az, conn, pending, state, code)
		}
	}
	if s.oauth == nil {
		return connectors.Connection{}, OAuthState{}, ErrOAuthUnavailable
	}
	grant, err := s.oauth.Exchange(ctx, a.TenantID, state, code)
	switch {
	case err == nil:
	case errors.Is(err, connectors.ErrInvalid):
		return connectors.Connection{}, OAuthState{}, ErrStateInvalid
	case errors.Is(err, connectors.ErrNotConfigured):
		return connectors.Connection{}, pending, ErrOAuthUnavailable
	case errors.Is(err, connectors.ErrOAuth2Refused):
		return connectors.Connection{}, pending, ErrOAuthRefused
	default:
		// An OAuth2 implementation other than the built-in one may carry provider text in its error: only
		// the classification leaves (the callback logs it).
		return connectors.Connection{}, pending, serrors.Wrap(op, ErrOAuthExchange)
	}
	if grant.Kind != pending.Kind {
		return connectors.Connection{}, OAuthState{}, ErrStateInvalid
	}
	// The connector's Test produced the identity from the new token: it is stored and listed, so it is
	// sanitized against the credentials before either write: every field of an OAuth2 connector's token
	// set is secret; a connector with a form besides its sign-in keeps its login and mailbox in the clear.
	connector, ok := s.dir.Get(grant.Kind)
	if !ok {
		return connectors.Connection{}, pending, connectors.ErrNotFound
	}
	secret := secretsOf(connector.Descriptor())
	grant.Identity = validIdentity(grant.Identity, secret, grant.Credentials)
	var primary connectors.GroupItem
	if pending.ConnectionID != uuid.Nil {
		cur, old, err := s.store.Get(ctx, a.TenantID, pending.ConnectionID)
		if err != nil {
			return connectors.Connection{}, pending, mapStoreError(op, err)
		}
		if cur.Kind != grant.Kind {
			return connectors.Connection{}, pending, connectors.ErrNotFound
		}
		// A sign-in of another account does not take over a connection that is part of a pair: the other
		// connection of the pair would keep the old mailbox's settings and take the new account's tokens.
		if len(grant.Companions) > 0 && cur.Identity.External != "" && cur.Identity.External != grant.Identity.External {
			return connectors.Connection{}, pending, ErrOAuthRefused
		}
		cur.Identity = validIdentity(grant.Identity, secret, grant.Credentials, old)
		scrubRetained(&cur, secret, old, grant.Credentials)
		if cur.Status != connectors.StatusDisabled {
			cur.Status, cur.StatusReason = connectors.StatusConnected, ""
		}
		primary = connectors.GroupItem{Connection: cur, Credentials: grant.Credentials}
	} else {
		name := pending.Name
		if name == "" {
			name = slug(string(grant.Kind))
		}
		display := pending.DisplayName
		if display == "" {
			display = grant.Identity.Display
		}
		primary = connectors.GroupItem{Connection: connectors.Connection{
			TenantID: a.TenantID, Kind: grant.Kind, Name: name, DisplayName: display,
			Identity: grant.Identity, Status: connectors.StatusConnected, CreatedBy: a.UserID,
		}, Credentials: grant.Credentials}
	}
	var saved connectors.Connection
	if len(grant.Companions) == 0 {
		if saved, err = s.store.Save(ctx, primary.Connection, primary.Credentials); err != nil {
			return connectors.Connection{}, pending, mapStoreError(op, err)
		}
	} else {
		// One sign-in, several connections (an email box sends and receives): saved together, so a failed
		// write leaves neither, and the later ones take their tokens from the first.
		group, ok := s.store.(connectors.GroupSaver)
		if !ok {
			return connectors.Connection{}, pending, serrors.Wrap(op, ErrOAuthExchange)
		}
		items := []connectors.GroupItem{primary}
		for _, comp := range grant.Companions {
			cc, ok := s.dir.Get(comp.Kind)
			if !ok {
				return connectors.Connection{}, pending, serrors.Wrap(op, ErrOAuthExchange)
			}
			csecret := secretsOf(cc.Descriptor())
			comp.Identity = validIdentity(comp.Identity, csecret, comp.Credentials)
			it, err := connectors.PlanCompanion(ctx, s.store, csecret, a.TenantID, comp, a.UserID, primary)
			if err != nil {
				return connectors.Connection{}, pending, mapStoreError(op, err)
			}
			items = append(items, it)
		}
		all, err := group.SaveGroup(ctx, items)
		if err != nil {
			return connectors.Connection{}, pending, mapStoreError(op, err)
		}
		saved = all[0]
	}
	s.touch(ctx, a.TenantID, saved.ID)
	return saved, pending, nil
}

// slug is a connection name from a connector kind.
func slug(kind string) string {
	var b strings.Builder
	for _, r := range strings.ToLower(kind) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	out := b.String()
	if out == "" || out[0] < 'a' || out[0] > 'z' {
		out = "c_" + out
	}
	if len(out) > 40 {
		out = out[:40]
	}
	return out
}

// OAuthAbandon drops the state of an authorization the provider refused (the user cancelled) and
// returns it, so the page reopens the form it came from.
func (s *IntegrationsService) OAuthAbandon(ctx context.Context, a Actor, state string) (OAuthState, error) {
	if s.states == nil || state == "" {
		return OAuthState{}, ErrStateInvalid
	}
	return s.states.Take(ctx, a.TenantID, a.UserID, StateHash(state), s.now())
}

// failedStatus is the status and stored reason of a connection its provider refused: an OAuth2 token
// the provider no longer accepts needs a new sign-in (the page offers it); other credentials are an
// error. The reason is a Reason, never the provider's text.
func failedStatus(conn connectors.Connector, creds connectors.Credentials, r Reason) (connectors.ConnectionStatus, string) {
	az, custom := conn.(connectors.ConnectionAuthorizer)
	oauth := conn.Descriptor().Auth == connectors.AuthOAuth2 || (custom && az.SignsIn(creds))
	if oauth && (r == ReasonAuth || r == ReasonUnknown) {
		return connectors.StatusNeedsReauth, string(ReasonReauth)
	}
	return connectors.StatusError, string(r)
}

// oauthFailure is what of an OAuth2 implementation's error may leave the service (the RPC logs what it
// does not recognise): the documented sentinels and a cancellation as such, anything else as
// ErrOAuthProvider without its text. A built-in or custom implementation may put a provider's response,
// and with it credentials, in its error.
func oauthFailure(err error) error {
	for _, keep := range []error{
		connectors.ErrNotFound, connectors.ErrNotConfigured, connectors.ErrUndecryptable, connectors.ErrVersionConflict,
		connectors.ErrNeedsReauth, connectors.ErrNotConnected, connectors.ErrInvalid, context.Canceled, context.DeadlineExceeded,
	} {
		if errors.Is(err, keep) {
			return keep
		}
	}
	return ErrOAuthProvider
}

// StateHash is the stored form of an OAuth2 state.
func StateHash(state string) []byte {
	sum := sha256.Sum256([]byte(state))
	return sum[:]
}

func (s *IntegrationsService) entry(ctx context.Context, tenantID uuid.UUID, c connectors.Connection, withFields bool) (ConnectionEntry, error) {
	e := ConnectionEntry{Connection: c}
	if conn, ok := s.dir.Get(c.Kind); ok {
		if az, ok := conn.(connectors.ConnectionAuthorizer); ok {
			if _, creds, err := s.store.Get(ctx, tenantID, c.ID); err == nil {
				e.SignIn = az.SignsIn(creds)
			}
		}
	}
	var err error
	if e.UsedBy, err = s.usedBy(ctx, tenantID, c.ID); err != nil {
		return ConnectionEntry{}, err
	}
	if s.checks != nil {
		// The time of the last check is informational: entry also answers a committed create or update,
		// which must not read as failed (and be retried into name_taken or a version conflict) because
		// this lookup failed.
		if checked, err := s.checks.LastChecked(ctx, tenantID); err == nil {
			if at, ok := checked[c.ID]; ok {
				e.LastChecked = &at
			}
		}
	}
	return s.reveal(ctx, tenantID, e, withFields)
}

// scrubRetained clears the display name a connection keeps across a credential change when it carries
// a value of the old or the new credentials (the store falls back to the name). The name itself is never
// changed: an administrator chose it (or it was derived from the kind, never from a provider) and
// package secrets and nodes refer to the connection by it.
func scrubRetained(c *connectors.Connection, secret secretSet, old, next connectors.Credentials) {
	if carriesSecret(c.DisplayName, secret, old, next) {
		c.DisplayName = ""
	}
}

// reveal is the output boundary of a connection: whatever wrote the row (this service, Store.Save,
// OAuth2.Complete of the shared connector API), what leaves the service is checked against the
// connection's current credentials — the identity by validIdentity, the display name by carriesSecret;
// what fails is emptied. The name is shown as stored: it is an identifier an administrator chose
// (references resolve by it), never provider text. A row whose credentials cannot be read shows no identity and
// no display name, since it cannot be checked.
func (s *IntegrationsService) reveal(ctx context.Context, tenantID uuid.UUID, e ConnectionEntry, withFields bool) (ConnectionEntry, error) {
	// Metadata and credentials come from the same read of the row: a list read earlier may predate a
	// credential rotation, and checking its metadata against newer credentials would let an old secret
	// through.
	cur, creds, err := s.store.Get(ctx, tenantID, e.Connection.ID)
	switch {
	case err == nil:
		e.Connection = cur
	case errors.Is(err, connectors.ErrNotConfigured), errors.Is(err, connectors.ErrUndecryptable), errors.Is(err, connectors.ErrNotFound):
		e.Connection.Identity, e.Connection.DisplayName, e.Fields = connectors.Identity{}, "", nil
		return e, nil
	default:
		return ConnectionEntry{}, err
	}
	c := e.Connection
	secret := func(string) bool { return true }
	conn, known := s.dir.Get(c.Kind)
	if known {
		secret = secretsOf(conn.Descriptor())
	}
	e.Connection.Identity = validIdentity(c.Identity, secret, creds)
	if carriesSecret(c.DisplayName, secret, creds) {
		e.Connection.DisplayName = ""
	}
	// An OAuth2 connection's credentials are a token set the exchange stores whatever the schema says:
	// none of it is a form field.
	if withFields && known && conn.Descriptor().Auth != connectors.AuthOAuth2 {
		e.Fields = connectors.RedactedCredentials(conn.Descriptor(), creds)
	}
	return e, nil
}

func (s *IntegrationsService) touch(ctx context.Context, tenantID, id uuid.UUID) {
	if s.checks != nil {
		// The time of the last check is informational: failing to record it does not fail the action.
		_ = s.checks.Touch(ctx, tenantID, id, s.now())
	}
}

// test runs Connector.Test. A refusal is a *TestFailedError with its category only; the identity is
// validated against creds and extra (the stored credentials of an update) before anything keeps it.
func (s *IntegrationsService) test(ctx context.Context, conn connectors.Connector, creds connectors.Credentials, extra ...connectors.Credentials) (connectors.Identity, error) {
	all := append([]connectors.Credentials{creds}, extra...)
	secret := secretsOf(conn.Descriptor())
	identity, err := conn.Test(ctx, creds)
	if err == nil {
		return validIdentity(identity, secret, all...), nil
	}
	if ctx.Err() != nil {
		return connectors.Identity{}, ctx.Err()
	}
	msg := err.Error()
	return connectors.Identity{}, &TestFailedError{Reason: classify(msg)}
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
	// The same predicate as the metadata checks: without x-secret fields every field is secret.
	secret := secretsOf(d)
	declared := declaredFields(d)
	internal := connectors.InternalFields(d)
	out := connectors.Credentials{}
	for k, v := range stored {
		out[k] = v
	}
	for k, v := range given {
		if (declared != nil && !declared[k]) || internal[k] {
			continue // a field the connector writes itself is never taken from a client
		}
		switch {
		case v == "" && secret(k):
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
		errors.Is(err, connectors.ErrNotConfigured), errors.Is(err, ErrRequestInProgress):
		return err
	}
	var invalid *InvalidInputError
	if errors.As(err, &invalid) {
		return err
	}
	return serrors.Wrap(op, err)
}
