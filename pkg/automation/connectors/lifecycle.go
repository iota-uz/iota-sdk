package connectors

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"

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
	Provision(ctx context.Context, c Connection, creds Credentials) (inbound map[string]json.RawMessage, err error)
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
	StartPlatform(ctx context.Context) error
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
