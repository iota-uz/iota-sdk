package connectors

import (
	"context"
	"errors"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
)

// Provisioner is an optional interface of a Connector that sets up the provider's side of a connection:
// a webhook the provider calls. The integrations service calls it; a connector never assumes who does.
type Provisioner interface {
	// Provision runs when a connection is saved: created, its credentials replaced, or enabled again.
	// An error refuses the save (what the save wrote is rolled back). It must be safe to repeat, and
	// neither it nor its error carries credential values.
	Provision(ctx context.Context, c Connection, creds Credentials) error
	// Deprovision undoes Provision for credentials a connection no longer uses: it was deleted or
	// disabled, or its credentials were replaced (these are the old ones). The caller ignores its error:
	// a provider that cannot be reached never keeps a connection from being removed.
	Deprovision(ctx context.Context, c Connection, creds Credentials) error
}

// Warning is something a person confirms before a connection is saved. Code is a stable identifier the
// screen shows a phrase for; the connector's own text never reaches a client.
type Warning struct {
	Code string `json:"code"`
}

// Advisor is an optional interface of a Connector: it looks at credentials the provider accepts and
// reports what connecting them would change elsewhere (a bot that another service already uses). A
// connection with a warning is saved only when the person confirmed its code.
type Advisor interface {
	Advise(ctx context.Context, c Credentials) ([]Warning, error)
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
