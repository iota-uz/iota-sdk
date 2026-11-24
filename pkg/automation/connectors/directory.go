package connectors

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/automation/catalog"
	"github.com/iota-uz/iota-sdk/pkg/automation/graph"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type directory struct {
	mu   sync.RWMutex
	reg  catalog.Registry
	byID map[catalog.ConnectorKind]Connector
}

// NewDirectory is an empty directory that registers each connector's contribution in reg (nil: the
// directory only lists connectors).
func NewDirectory(reg catalog.Registry) Directory {
	return &directory{reg: reg, byID: map[catalog.ConnectorKind]Connector{}}
}

func (d *directory) Register(c Connector) error {
	const op = serrors.Op("automation.connectors.Directory.Register")
	if c == nil {
		return serrors.Wrap(op, fmt.Errorf("%w: a nil connector", ErrInvalid))
	}
	desc := c.Descriptor()
	if err := checkDescriptor(c.Kind(), desc); err != nil {
		return serrors.Wrap(op, err)
	}
	if ca, ok := c.(CodeAccessor); ok {
		if err := ca.CodeAccess().Validate(); err != nil {
			return serrors.Wrap(op, fmt.Errorf("connector %s: %w", desc.Kind, err))
		}
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if _, dup := d.byID[desc.Kind]; dup {
		return serrors.Wrap(op, fmt.Errorf("%w: connector %s is registered twice", ErrInvalid, desc.Kind))
	}
	if d.reg != nil {
		if err := d.reg.RegisterConnector(c); err != nil {
			return serrors.Wrap(op, err)
		}
	}
	d.byID[desc.Kind] = c
	return nil
}

func (d *directory) List() []Connector {
	d.mu.RLock()
	defer d.mu.RUnlock()
	out := make([]Connector, 0, len(d.byID))
	for _, c := range d.byID {
		out = append(out, c)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Kind() < out[j].Kind() })
	return out
}

func (d *directory) Get(kind catalog.ConnectorKind) (Connector, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()
	c, ok := d.byID[kind]
	return c, ok
}

func checkDescriptor(kind catalog.ConnectorKind, d Descriptor) error {
	switch {
	case kind == "":
		return fmt.Errorf("%w: a connector without a kind", ErrInvalid)
	case d.Kind != kind:
		return fmt.Errorf("%w: connector %s: its descriptor's kind is %q", ErrInvalid, kind, d.Kind)
	}
	switch d.Auth {
	case AuthAPIKey, AuthBasic, AuthCustom:
		if d.OAuth2 != nil {
			return fmt.Errorf("%w: connector %s: OAuth2 is only for auth oauth2", ErrInvalid, kind)
		}
	case AuthOAuth2:
		if d.OAuth2 == nil || d.OAuth2.AuthURL == "" || d.OAuth2.TokenURL == "" {
			return fmt.Errorf("%w: connector %s: auth oauth2 needs OAuth2 with AuthURL and TokenURL", ErrInvalid, kind)
		}
		// The client credential must not be the one offered to tenants: nodes would get client_secret.
		if cc := d.OAuth2.ClientCredential; (d.Platform == PlatformAuto || d.Platform == PlatformOptIn) && (cc == "" || cc == string(kind)) {
			return fmt.Errorf("%w: connector %s: a platform policy needs an OAuth2 client credential named apart from the kind", ErrInvalid, kind)
		}
	default:
		return fmt.Errorf("%w: connector %s: auth %q is not api_key, basic, oauth2 or custom", ErrInvalid, kind, d.Auth)
	}
	switch d.Platform {
	case "", PlatformNone, PlatformAuto, PlatformOptIn:
	default:
		return fmt.Errorf("%w: connector %s: platform policy %q is not none, auto or opt_in", ErrInvalid, kind, d.Platform)
	}
	if len(d.Credentials) > 0 {
		if _, err := graph.SchemaParams(d.Credentials); err != nil {
			return fmt.Errorf("%w: connector %s: credentials schema: %v", ErrInvalid, kind, err)
		}
	}
	return nil
}

// SecretFields are the credential fields whose schema property has "x-secret": true.
func SecretFields(d Descriptor) map[string]bool {
	var doc struct {
		Properties map[string]struct {
			Secret bool `json:"x-secret"`
		} `json:"properties"`
	}
	out := map[string]bool{}
	if len(d.Credentials) == 0 || json.Unmarshal(d.Credentials, &doc) != nil {
		return out
	}
	for name, p := range doc.Properties {
		if p.Secret {
			out[name] = true
		}
	}
	return out
}

// ValidateCredentials checks credentials against the connector's credentials schema. Issues name the
// fields, never their values.
func ValidateCredentials(d Descriptor, c Credentials) error {
	if len(d.Credentials) == 0 {
		return nil
	}
	p, err := graph.SchemaParams(d.Credentials)
	if err != nil {
		return fmt.Errorf("%w: credentials schema: %v", ErrInvalid, err)
	}
	// Credentials are strings (in the form, on the wire and stored); a property the schema declares as
	// integer, number or boolean is validated as that scalar read from its text, so its exact digits are
	// kept. Text that is not such a scalar stays a string and fails the type.
	types := propertyTypes(d.Credentials)
	raw := make(map[string]any, len(c))
	for k, v := range c {
		raw[k] = scalar(types[k], v)
	}
	if _, issues := p.Parse(raw); len(issues) > 0 {
		fields := make([]string, 0, len(issues))
		for _, i := range issues {
			if len(i.Path) > 0 {
				fields = append(fields, fmt.Sprint(i.Path[0]))
			} else {
				fields = append(fields, "credentials")
			}
		}
		return fmt.Errorf("%w: credentials: check %v", ErrInvalid, fields)
	}
	return nil
}

func propertyTypes(schema json.RawMessage) map[string]string {
	var doc struct {
		Properties map[string]struct {
			Type json.RawMessage `json:"type"`
		} `json:"properties"`
	}
	if json.Unmarshal(schema, &doc) != nil {
		return nil
	}
	out := make(map[string]string, len(doc.Properties))
	for k, p := range doc.Properties {
		var t string
		if json.Unmarshal(p.Type, &t) == nil {
			out[k] = t
		}
	}
	return out
}

var (
	integerText = regexp.MustCompile(`^-?(0|[1-9][0-9]*)$`)
	numberText  = regexp.MustCompile(`^-?(0|[1-9][0-9]*)(\.[0-9]+)?([eE][+-]?[0-9]+)?$`)
)

func scalar(typ, v string) any {
	switch {
	case typ == "integer" && integerText.MatchString(v), typ == "number" && numberText.MatchString(v):
		return json.Number(v)
	case typ == "boolean" && (v == "true" || v == "false"):
		return v == "true"
	}
	return v
}

// RedactedCredentials is what a client may see of credentials: the fields the credentials schema
// declares without x-secret. A field the schema does not declare is withheld, so a connector without a
// schema shows nothing.
func RedactedCredentials(d Descriptor, c Credentials) map[string]string {
	var doc struct {
		Properties map[string]struct {
			Secret bool `json:"x-secret"`
		} `json:"properties"`
	}
	out := map[string]string{}
	if len(d.Credentials) == 0 || json.Unmarshal(d.Credentials, &doc) != nil {
		return out
	}
	for k, v := range c {
		if p, declared := doc.Properties[k]; declared && !p.Secret {
			out[k] = v
		}
	}
	return out
}
