package connectors

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
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
	if _, ok := c.(ConnectionAuthorizer); ok && len(desc.OAuth2Presets) > 0 {
		// Two sign-ins of different kinds: the connection's own authorization server (it is saved first, then
		// signed in) and the platform's providers (the sign-in makes the connections). A state of one would be
		// finished as the other, so a connector has one.
		return serrors.Wrap(op, fmt.Errorf("%w: connector %s declares OAuth2 presets and is a ConnectionAuthorizer", ErrInvalid, desc.Kind))
	}
	if ca, ok := c.(CodeAccessor); ok {
		access := ca.CodeAccess()
		check := access.Validate
		if _, resolved := c.(CodeBaseURLResolver); resolved {
			check = access.ValidateResolved
		}
		if err := check(); err != nil {
			return serrors.Wrap(op, fmt.Errorf("connector %s: %w", desc.Kind, err))
		}
		if _, operates := c.(CodeOperator); len(access.Operations) > 0 && !operates {
			return serrors.Wrap(op, fmt.Errorf("%w: connector %s declares code operations and is not a CodeOperator", ErrInvalid, desc.Kind))
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
	// AuthParams add to the authorization URL: they never replace what the flow itself sets.
	reserved := func(params map[string]string) error {
		for k := range params {
			switch strings.ToLower(k) {
			case "state", "client_id", "redirect_uri", "response_type", "scope", "code_challenge", "code_challenge_method":
				return fmt.Errorf("%w: connector %s: AuthParams may not set %q", ErrInvalid, kind, k)
			}
		}
		return nil
	}
	if d.OAuth2 != nil {
		if err := reserved(d.OAuth2.AuthParams); err != nil {
			return err
		}
	}
	for name, cfg := range d.OAuth2Presets {
		if err := reserved(cfg.AuthParams); err != nil {
			return err
		}
		if d.Auth == AuthOAuth2 || name == "" || cfg.AuthURL == "" || cfg.TokenURL == "" {
			return fmt.Errorf("%w: connector %s: OAuth2 preset %q needs a name, AuthURL and TokenURL, and a connector that is not auth oauth2", ErrInvalid, kind, name)
		}
		if cc := cfg.ClientCredential; cc == "" || cc == string(kind) {
			return fmt.Errorf("%w: connector %s: OAuth2 preset %q needs a client credential named apart from the kind", ErrInvalid, kind, name)
		}
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
		if err := checkAnnotations(kind, d.Credentials); err != nil {
			return err
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

// AuthorizationOnlyFields are the credential fields whose schema property has "x-authorization-only": true:
// the connector uses them only to authorize its own requests (an API token). They are never served as a
// secret by name (SecretResolver, a node's CONNECTION__FIELD), so code reaches the API only through the
// connection, never with the credential itself.
func AuthorizationOnlyFields(d Descriptor) map[string]bool {
	out := map[string]bool{}
	props, ok := schemaProperties(d.Credentials)
	if !ok {
		return out // no readable properties: SecretFields has none either, so nothing is served by name
	}
	for name, p := range props {
		raw, set := p["x-authorization-only"]
		if !set {
			continue
		}
		// Fail closed: only an explicit false lets the field be served; any other value (true, "false", a
		// number) withholds it. Directory.Register refuses such a schema anyway.
		var only bool
		if json.Unmarshal(raw, &only) != nil || only {
			out[name] = true
		}
	}
	return out
}

// schemaProperties are the raw keywords of each property of a credentials schema; a property that is not
// an object is kept with no keywords.
func schemaProperties(schema json.RawMessage) (map[string]map[string]json.RawMessage, bool) {
	var doc struct {
		Properties map[string]json.RawMessage `json:"properties"`
	}
	if len(schema) == 0 || json.Unmarshal(schema, &doc) != nil {
		return nil, false
	}
	out := make(map[string]map[string]json.RawMessage, len(doc.Properties))
	for name, raw := range doc.Properties {
		var kw map[string]json.RawMessage
		_ = json.Unmarshal(raw, &kw)
		out[name] = kw
	}
	return out, true
}

// checkAnnotations refuses a credentials schema whose x-authorization-only is not a boolean.
func checkAnnotations(kind catalog.ConnectorKind, schema json.RawMessage) error {
	props, _ := schemaProperties(schema)
	for name, p := range props {
		if raw, set := p["x-authorization-only"]; set {
			var only bool
			if json.Unmarshal(raw, &only) != nil {
				return fmt.Errorf("%w: connector %s: credentials property %s: x-authorization-only is not a boolean", ErrInvalid, kind, name)
			}
		}
	}
	return nil
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
// declares without x-secret or x-internal. A field the schema does not declare is withheld, so a
// connector without a schema shows nothing.
func RedactedCredentials(d Descriptor, c Credentials) map[string]string {
	var doc struct {
		Properties map[string]struct {
			Secret   bool `json:"x-secret"`
			Internal bool `json:"x-internal"`
		} `json:"properties"`
	}
	out := map[string]string{}
	if len(d.Credentials) == 0 || json.Unmarshal(d.Credentials, &doc) != nil {
		return out
	}
	for k, v := range c {
		if p, declared := doc.Properties[k]; declared && !p.Secret && !p.Internal {
			out[k] = v
		}
	}
	return out
}
