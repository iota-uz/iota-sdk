package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// Provisioner is an optional interface of a Connector that sets up the provider's side of a connection:
// a webhook the provider calls. The integrations service calls it; a connector never assumes who does.
type Provisioner interface {
	// Provision runs when a connection is saved: created, its credentials replaced, or enabled again.
	// An error refuses the save (what the save wrote is rolled back). It must be safe to repeat, and
	// neither it nor its error carries credential values. The keys it returns are merged into the
	// connection's inbound state in the same transaction as the save (a webhook secret it generated): the
	// connector reads them back through InboundReader, never from the credentials.
	//
	// consent are the codes of the warnings the person confirmed. Provision looks at the provider's state
	// itself, at the moment it changes it, and refuses (a *ConfirmationRequiredError, a *BlockedError) what
	// needs a confirmation that is not among them or cannot be given: advice given earlier may be stale.
	//
	// held is the connection's inbound state as it is in the save's transaction (what a previous setup
	// handed back), read there so that Provision needs no second database connection while the save holds
	// its own.
	Provision(ctx context.Context, c Connection, creds Credentials, consent []string, held json.RawMessage) (inbound map[string]json.RawMessage, err error)
	// Deprovision undoes Provision for credentials a connection no longer uses: it was deleted or
	// disabled, or its credentials were replaced (these are the old ones). The caller ignores its error:
	// a provider that cannot be reached never keeps a connection from being removed.
	Deprovision(ctx context.Context, c Connection, creds Credentials) error
}

// Warning is something a person confirms before a connection is saved. Code is a stable identifier the
// screen shows a phrase for; the connector's own text never reaches a client.
type Warning struct {
	Code string `json:"code"`
	// Blocking: the credentials cannot be connected at all (the bot belongs to another connection); no
	// confirmation lifts it. Field is the credentials field the screen shows it at.
	Blocking bool   `json:"blocking,omitempty"`
	Field    string `json:"field,omitempty"`
}

// Advisor is an optional interface of a Connector: it looks at credentials the provider accepts and
// reports what connecting them would change elsewhere (a bot that another service already uses). A
// connection with a warning is saved only when the person confirmed its code.
type Advisor interface {
	// Advise is asked for a new connection (existing nil) and for credentials a connection is saved or
	// enabled with (existing is that connection).
	Advise(ctx context.Context, existing *Connection, c Credentials) ([]Warning, error)
}

// ConfirmationRequiredError is a save that has warnings the person did not confirm.
type ConfirmationRequiredError struct {
	Warnings []Warning
}

func (e *ConfirmationRequiredError) Error() string {
	codes := make([]string, 0, len(e.Warnings))
	for _, w := range e.Warnings {
		codes = append(codes, w.Code)
	}
	return "automation connections: confirmation required: " + strings.Join(codes, ", ")
}

// ErrConfirmationRequired matches a *ConfirmationRequiredError.
var ErrConfirmationRequired = errors.New("automation connections: confirmation required")

func (e *ConfirmationRequiredError) Is(target error) bool { return target == ErrConfirmationRequired }

// Unconfirmed is the warnings whose codes are not among confirmed.
func Unconfirmed(warnings []Warning, confirmed []string) []Warning {
	var out []Warning
	for _, w := range warnings {
		ok := false
		for _, c := range confirmed {
			ok = ok || c == w.Code
		}
		if !ok {
			out = append(out, w)
		}
	}
	return out
}

// PlatformProvider is an optional interface of a Connector that brings its own platform credential
// (a token from the application's environment) when the configuration names none for its kind.
type PlatformProvider interface {
	PlatformCredential() (PlatformCredential, bool)
}

// PlatformStarter is an optional interface of a Connector that sets up its platform credential when the
// application starts (a webhook of the platform bot). An error is logged; the application still starts.
type PlatformStarter interface {
	// StartPlatform sets the platform credential up. creds is the effective platform credential: the
	// configured one when the configuration names the kind, else the connector's own (PlatformProvider).
	StartPlatform(ctx context.Context, creds Credentials) error
}

// WithPlatformProviders adds the platform credentials the connectors of dir bring themselves, for a
// kind the configuration (creds) does not name.
func WithPlatformProviders(creds map[catalog.ConnectorKind]PlatformCredential, list ...Connector) map[catalog.ConnectorKind]PlatformCredential {
	out := make(map[catalog.ConnectorKind]PlatformCredential, len(creds)+len(list))
	for k, v := range creds {
		out[k] = v
	}
	for _, c := range list {
		pp, ok := c.(PlatformProvider)
		if !ok {
			continue
		}
		if _, set := out[c.Kind()]; set {
			continue
		}
		if pc, ok := pp.PlatformCredential(); ok && len(pc.Credentials) > 0 {
			out[c.Kind()] = pc
		}
	}
	return out
}

// Option is a choice offered for a node's param: the value goes into the params, the label is shown, the
// group (a key the screen names) sorts the choices.
type Option struct {
	Value string `json:"value"`
	Label string `json:"label"`
	Group string `json:"group,omitempty"`
}

// OptionSource is an optional interface of a Connector: the choices of a param field that names the
// source in its schema (x-options-source), for one of the tenant's connections: the chats a bot received
// messages from. The query narrows them.
type OptionSource interface {
	Options(ctx context.Context, tenantID, connectionID uuid.UUID, source, query string) ([]Option, error)
}

// MergeInbound sets the keys of add in an inbound state (a JSON object), keeping the others.
func MergeInbound(state json.RawMessage, add map[string]json.RawMessage) (json.RawMessage, error) {
	doc := map[string]json.RawMessage{}
	if len(state) > 0 {
		if err := json.Unmarshal(state, &doc); err != nil {
			doc = map[string]json.RawMessage{}
		}
	}
	for k, v := range add {
		doc[k] = v
	}
	return json.Marshal(doc)
}

// MergeInboundTx merges keys into the inbound state of a connection in the caller's transaction, which
// already holds its row (a save).
func MergeInboundTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID, add map[string]json.RawMessage) error {
	if len(add) == 0 {
		return nil
	}
	raw, err := json.Marshal(add)
	if err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE automation.connections SET inbound = COALESCE(inbound, '{}'::jsonb) || $3::jsonb WHERE tenant_id = $1 AND id = $2`,
		tenantID, id, raw)
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return fmt.Errorf("%w: the connection to record the provider's state on", ErrNotFound)
	}
	return nil
}

// ExistenceChecker is an optional interface of a ConnectionStore: whether a connection of any tenant
// exists. A connector that finds its bot's webhook pointing at a connection asks it, to tell a bot that
// belongs to a live connection from a webhook nobody owns any more.
type ExistenceChecker interface {
	ConnectionExists(ctx context.Context, id uuid.UUID) (bool, error)
}

// ExclusiveIdentity is an optional interface of a Connector whose provider account can deliver to one place
// only (a Telegram bot has one webhook): the account (Identity.External) belongs to one connection, or to
// the platform credential, never two. The integrations service reserves it in the transaction of the save,
// under a lock on the account, before the provider is changed.
type ExclusiveIdentity interface {
	// ExternalID is the provider's identifier of the account the credentials belong to, as Test reports it in
	// Identity.External; "" when the credentials name none.
	ExternalID(creds Credentials) string
}

// BlockedError is credentials that cannot be connected: the provider account belongs to another connection
// (or to the platform). Field is the credentials field the screen shows it at. No confirmation lifts it.
type BlockedError struct{ Field string }

func (e *BlockedError) Error() string {
	return "automation connections: the provider account belongs to another connection"
}

// ErrIdentityTaken is returned when the exclusive identity belongs to another connection or to the platform credential.
var ErrIdentityTaken = errors.New("automation connections: the provider account belongs to another connection")

func identityLockKey(kind catalog.ConnectorKind, external string) string {
	return string(kind) + "|" + external
}

// ReserveIdentity gives the exclusive identity to a connection in the caller's transaction (the save): it
// takes an advisory lock on the identity until the transaction ends, so concurrent saves for the same
// account run one after the other and the later one sees the earlier's reservation. The connection's
// reservation of another account (its credentials changed to another bot) is released. ErrIdentityTaken
// when another connection or the platform credential holds it.
func ReserveIdentity(ctx context.Context, tx pgx.Tx, kind catalog.ConnectorKind, external string, connectionID uuid.UUID) error {
	if external == "" {
		return nil
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identityLockKey(kind, external)); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE connection_id = $1 AND kind = $2 AND external <> $3`,
		connectionID, string(kind), external); err != nil {
		return err
	}
	var holder *uuid.UUID
	err := tx.QueryRow(ctx, `SELECT connection_id FROM automation.exclusive_identities WHERE kind = $1 AND external = $2`, string(kind), external).Scan(&holder)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		_, err = tx.Exec(ctx, `INSERT INTO automation.exclusive_identities (kind, external, connection_id) VALUES ($1, $2, $3)`, string(kind), external, connectionID)
		return err
	case err != nil:
		return err
	case holder != nil && *holder == connectionID:
		return nil
	}
	return ErrIdentityTaken
}

// identityHolder is who holds an exclusive identity.
type identityHolder struct {
	connection uuid.UUID // the holding connection; uuid.Nil for the platform credential
	platform   bool
	held       bool
}

func holderOf(ctx context.Context, q interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}, kind catalog.ConnectorKind, external string) (identityHolder, error) {
	var (
		holder   *uuid.UUID
		platform bool
	)
	err := q.QueryRow(ctx, `SELECT connection_id, platform FROM automation.exclusive_identities WHERE kind = $1 AND external = $2`, string(kind), external).Scan(&holder, &platform)
	if errors.Is(err, pgx.ErrNoRows) {
		return identityHolder{}, nil
	}
	if err != nil {
		return identityHolder{}, err
	}
	out := identityHolder{platform: platform, held: true}
	if holder != nil {
		out.connection = *holder
	}
	return out, nil
}

// ReservePlatformIdentity gives the exclusive identity to the platform credential, under the same lock as a
// connection's reservation. A connection that holds it already keeps it (ErrIdentityTaken): the platform's
// credential then cannot be set up for that account. The platform's reservation of another account is released.
func ReservePlatformIdentity(ctx context.Context, db *pgxpool.Pool, kind catalog.ConnectorKind, external string) error {
	if external == "" {
		return nil
	}
	return pgx.BeginFunc(ctx, db, func(tx pgx.Tx) error {
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, identityLockKey(kind, external)); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE platform AND kind = $1 AND external <> $2`, string(kind), external); err != nil {
			return err
		}
		holder, err := holderOf(ctx, tx, kind, external)
		switch {
		case err != nil:
			return err
		case holder.held && holder.platform:
			return nil
		case holder.held && holder.connection != uuid.Nil:
			return ErrIdentityTaken
		}
		_, err = tx.Exec(ctx, `INSERT INTO automation.exclusive_identities (kind, external, platform) VALUES ($1, $2, TRUE)`, string(kind), external)
		return err
	})
}

// InboundTx reads a connection's inbound state in the caller's transaction.
func InboundTx(ctx context.Context, tx pgx.Tx, tenantID, id uuid.UUID) (json.RawMessage, error) {
	var state []byte
	err := tx.QueryRow(ctx, `SELECT inbound FROM automation.connections WHERE tenant_id = $1 AND id = $2`, tenantID, id).Scan(&state)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(state), nil
}

// ReleasePlatformIdentities releases the platform credential's reservations of a kind: the credential is no
// longer configured, so the account it held is free to be connected as a tenant's own.
func ReleasePlatformIdentities(ctx context.Context, db *pgxpool.Pool, kind catalog.ConnectorKind) error {
	_, err := db.Exec(ctx, `DELETE FROM automation.exclusive_identities WHERE platform AND kind = $1`, string(kind))
	return err
}

// InboundOnce is an optional interface of a ConnectionStore: a record of provider updates a connector handled
// itself (no event was published for them), by the provider's id, on the table of the engine's own dedup
// (automation.inbound_dedup, pruned with it). A connector checks it before it acts on a repeated delivery
// and records the update once it has completed, so an update whose answer to the provider was lost is not
// acted on twice.
type InboundOnce interface {
	// InboundSeen asks across tenants: a repeated delivery is recognised even when what it carried no longer
	// names the tenant it was first handled for (a one-use link that was consumed).
	InboundSeen(ctx context.Context, connection, event, key string) (bool, error)
	InboundDone(ctx context.Context, tenantID uuid.UUID, connection, event, key string) error
}
