// Package manifest is the only Go representation of extension resources.
// A manifest is produced by the build worker from the exported values of a
// package's contracts, server and client entry modules and stored with the
// immutable package version.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
)

const SchemaVersion = "iota.manifest/1"

type TypeKind string

const (
	KindString   TypeKind = "string"
	KindInt      TypeKind = "int"
	KindInt64    TypeKind = "int64"
	KindDecimal  TypeKind = "decimal"
	KindBool     TypeKind = "bool"
	KindDate     TypeKind = "date"
	KindDateTime TypeKind = "datetime"
	KindUUID     TypeKind = "uuid"
	KindEnum     TypeKind = "enum"
	KindJSON     TypeKind = "json"
	KindRef      TypeKind = "ref"
	KindObject   TypeKind = "object"
	KindArray    TypeKind = "array"
)

// Type describes a wire value. Optional means the object key may be absent;
// Nullable means the value may be JSON null.
type Type struct {
	Kind       TypeKind   `json:"kind"`
	Optional   bool       `json:"optional,omitempty"`
	Nullable   bool       `json:"nullable,omitempty"`
	Values     []string   `json:"values,omitempty"`
	Entity     string     `json:"entity,omitempty"`
	Properties []Property `json:"properties,omitempty"`
	Items      *Type      `json:"items,omitempty"`
	Precision  int        `json:"precision,omitempty"`
	Scale      int        `json:"scale,omitempty"`
	MaxLength  int        `json:"maxLength,omitempty"`
}

type Property struct {
	Name string `json:"name"`
	Type Type   `json:"type"`
}

type Label map[string]string

type ResourceKind string

const (
	ResEntity      ResourceKind = "entity"
	ResOverlay     ResourceKind = "overlay"
	ResQuery       ResourceKind = "query"
	ResCommand     ResourceKind = "command"
	ResOperation   ResourceKind = "operation"
	ResEvent       ResourceKind = "event"
	ResWorkflow    ResourceKind = "workflow"
	ResNotice      ResourceKind = "notice"
	ResRoute       ResourceKind = "route"
	ResPermission  ResourceKind = "permission"
	ResIssue       ResourceKind = "issue"
	ResDomainError ResourceKind = "domainError"
	ResColumn      ResourceKind = "column"
	ResPage        ResourceKind = "page"
	ResNavigation  ResourceKind = "navigation"
	ResSlot        ResourceKind = "slot"
	ResCell        ResourceKind = "cell"
	ResShell       ResourceKind = "shell"
	// ResContactResolver publishes how a record recipient resolves to channel
	// targets; ResDeliveryPolicy is a business consent/opt-out policy.
	ResContactResolver ResourceKind = "contactResolver"
	ResDeliveryPolicy  ResourceKind = "deliveryPolicy"
)

// Field is a stable, typed column of an entity or overlay. ID never changes;
// a rename is declared with RenamedFrom.
type Field struct {
	ID          string          `json:"id"`
	Type        Type            `json:"type"`
	Unique      bool            `json:"unique,omitempty"`
	Index       bool            `json:"index,omitempty"`
	Default     json.RawMessage `json:"default,omitempty"`
	RenamedFrom []string        `json:"renamedFrom,omitempty"`
	Label       Label           `json:"label,omitempty"`
}

// Condition is one comparison of a filter. Value may be the placeholder
// {"$":"user"} meaning the current subject's user Ref.
// Filter keys are field IDs or dotted ref paths ("holder.owner") compiled to
// explicit joins through ref fields.
type Condition struct {
	Eq     json.RawMessage   `json:"eq,omitempty"`
	Ne     json.RawMessage   `json:"ne,omitempty"`
	In     []json.RawMessage `json:"in,omitempty"`
	Lt     json.RawMessage   `json:"lt,omitempty"`
	Lte    json.RawMessage   `json:"lte,omitempty"`
	Gt     json.RawMessage   `json:"gt,omitempty"`
	Gte    json.RawMessage   `json:"gte,omitempty"`
	IsNull *bool             `json:"isNull,omitempty"`
	Like   *string           `json:"like,omitempty"`
}

// Filter is a conjunction of field conditions plus optional nested OR groups.
type Filter struct {
	Fields map[string]Condition `json:"fields,omitempty"`
	Or     []Filter             `json:"or,omitempty"`
}

// ReadRule grants row visibility. Rows nil with All=false denies; a rule
// applies when the subject holds Permission (empty = everyone authenticated).
type ReadRule struct {
	Permission string  `json:"permission,omitempty"`
	All        bool    `json:"all,omitempty"`
	Rows       *Filter `json:"rows,omitempty"`
}

// Access: no Read rules = readable by every subject of the tenant; with rules,
// visible rows are the union of rules whose Permission the subject holds (none
// applicable = no rows). Rows the subject cannot read cannot be updated or
// deleted either. Fields lists permissions that are all required to read the
// field (projection, filter, sort, export) and to write it: a subject cannot
// set, change or clear a field it cannot read (ErrFieldNotAccessible), so a
// write can neither overwrite nor probe a hidden value. An overlay row is
// visible only to a subject that can see its base row — the read permission of
// a native base, the read rules of an own base entity — in addition to the
// overlay's own rules.
type Access struct {
	Read   []ReadRule          `json:"read,omitempty"`
	Fields map[string][]string `json:"fields,omitempty"` // field id -> permissions required to read
}

type Nav struct {
	Label     Label  `json:"label"`
	Group     string `json:"group,omitempty"`
	Workspace string `json:"workspace,omitempty"`
	Parent    string `json:"parent,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Order     int    `json:"order,omitempty"`
}

type NavPatch struct {
	Ref    string `json:"ref"`
	Hide   bool   `json:"hide,omitempty"`
	Rename Label  `json:"rename,omitempty"`
	Parent string `json:"parent,omitempty"`
	Order  *int   `json:"order,omitempty"`
}

type NavGroup struct {
	ID        string `json:"id"`
	Label     Label  `json:"label"`
	Workspace string `json:"workspace,omitempty"`
	Icon      string `json:"icon,omitempty"`
	Order     int    `json:"order,omitempty"`
}

type Workspace struct {
	ID    string `json:"id"`
	Label Label  `json:"label"`
	Icon  string `json:"icon,omitempty"`
	Order int    `json:"order,omitempty"`
}

// PaletteEntry adds a command-palette/search entry pointing at a page or
// command resource.
type PaletteEntry struct {
	ID       string   `json:"id"`
	Label    Label    `json:"label"`
	Target   string   `json:"target"`
	Keywords []string `json:"keywords,omitempty"`
}

// Resource is a declared contract. Kind selects which fields are meaningful.
type Resource struct {
	ID   string       `json:"id"`
	Kind ResourceKind `json:"kind"`
	// ContractHash is computed by Go at publish time (ContractHashOf) from the
	// canonical public shape; it is never authored.
	ContractHash string `json:"contractHash,omitempty"`
	Label        Label  `json:"label,omitempty"`
	Description  string `json:"description,omitempty"`

	// entity / overlay
	Fields []Field `json:"fields,omitempty"`
	Base   string  `json:"base,omitempty"`
	Access *Access `json:"access,omitempty"`
	// KeyType of native entities ("int64" | "uuid" | "string"); package
	// entities are always uuid.
	KeyType string `json:"keyType,omitempty"`

	// query / command / operation / workflow / issue / domainError / event / notice / route
	Atomic  bool            `json:"atomic,omitempty"`
	Input   *Type           `json:"input,omitempty"`
	Output  *Type           `json:"output,omitempty"`
	Errors  []string        `json:"errors,omitempty"`
	Params  *Type           `json:"params,omitempty"`
	Payload *Type           `json:"payload,omitempty"`
	Subject string          `json:"subject,omitempty"`
	Signals map[string]Type `json:"signals,omitempty"`

	// Permission gates commands/queries/pages: an RBAC permission name
	// compared with spi.Subject.Permissions; enforced by the host for every
	// trigger (UI, MCP, endpoint), not only by the UI.
	Permission string `json:"permission,omitempty"`

	// route / page
	Path  string `json:"path,omitempty"`
	Route string `json:"route,omitempty"`
	Nav   *Nav   `json:"nav,omitempty"`
	Chunk string `json:"chunk,omitempty"`
	// Chunk/Export locate the client module of page, slot, cell, shell and
	// column (optional cell renderer) resources inside the client artifact.
	Export string `json:"export,omitempty"`

	// column
	List       string `json:"list,omitempty"`
	Overlay    string `json:"overlay,omitempty"`
	Field      string `json:"field,omitempty"`
	Filterable bool   `json:"filterable,omitempty"`
	Sortable   bool   `json:"sortable,omitempty"`
	Exportable bool   `json:"exportable,omitempty"`

	// navigation
	Groups     []NavGroup     `json:"groups,omitempty"`
	Patches    []NavPatch     `json:"patches,omitempty"`
	Workspaces []Workspace    `json:"workspaces,omitempty"`
	Palette    []PaletteEntry `json:"palette,omitempty"`

	// slot / cell target (slot id or column id)
	Target string `json:"target,omitempty"`
	// Props of a slot declaration (typed props passed to contributions).
	Props *Type `json:"props,omitempty"`
}

type ImplKind string

const (
	ImplCommand   ImplKind = "command"
	ImplAtomic    ImplKind = "atomic"
	ImplOperation ImplKind = "operation"
	ImplQuery     ImplKind = "query"
	ImplWorkflow  ImplKind = "workflow"
	// contact resolver / delivery policy implementations of #926 contracts
	ImplContactResolver ImplKind = "contactResolver"
	ImplDeliveryPolicy  ImplKind = "deliveryPolicy"
)

// Implementation binds a server export to a contract by ID. The contract may
// belong to another package (replacement).
type Implementation struct {
	Export     string   `json:"export"`
	Implements string   `json:"implements"`
	Kind       ImplKind `json:"kind"`
}

type Subscription struct {
	ID     string `json:"id"`
	Export string `json:"export"`
	Event  string `json:"event"`
}

type GuardPhase string

const (
	// GuardInput is a preliminary check of an ordinary command's input on a
	// read snapshot; it is not a race-free guarantee.
	GuardInput GuardPhase = "input"
	// GuardOperation runs inside the operation's/atomic command's transaction.
	GuardOperation GuardPhase = "operation"
	// GuardWrite runs for every write of an entity inside the writing tx.
	GuardWrite GuardPhase = "write"
)

// Guard is an additive check with its own stable ID (used as issue source).
// Authored as precheck(...) (input), guard(...) (operation) or
// writeRule(...) (write).
type Guard struct {
	ID     string     `json:"id"`
	Export string     `json:"export"`
	Target string     `json:"target"`
	Phase  GuardPhase `json:"phase"`
}

type Template struct {
	Export   string   `json:"export"`
	Notice   string   `json:"notice"`
	Channels []string `json:"channels"`
}

type Schedule struct {
	ID       string `json:"id"`
	Export   string `json:"export"`
	Cron     string `json:"cron"`
	Timezone string `json:"timezone,omitempty"`
}

type Endpoint struct {
	ID     string `json:"id"`
	Export string `json:"export"`
	Method string `json:"method"`
	Path   string `json:"path"`
	Public bool   `json:"public,omitempty"`
}

type Capabilities struct {
	Network []string `json:"network,omitempty"`
	Secrets []string `json:"secrets,omitempty"`
}

type Manifest struct {
	Schema          string           `json:"schema"`
	Namespace       string           `json:"namespace"`
	Resources       []Resource       `json:"resources"`
	Implementations []Implementation `json:"implementations,omitempty"`
	Subscriptions   []Subscription   `json:"subscriptions,omitempty"`
	Guards          []Guard          `json:"guards,omitempty"`
	Templates       []Template       `json:"templates,omitempty"`
	Schedules       []Schedule       `json:"schedules,omitempty"`
	Endpoints       []Endpoint       `json:"endpoints,omitempty"`
	Capabilities    Capabilities     `json:"capabilities,omitempty"`
	// Dependencies maps a foreign namespace to the catalog snapshot hash the
	// package was built against (contract resolution snapshot).
	Dependencies map[string]string `json:"dependencies,omitempty"`
}

var (
	namespacePattern = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)
	idPattern        = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-zA-Z0-9_]*)+$`)
)

// Find returns the resource with id.
func (m *Manifest) Find(id string) (Resource, bool) {
	for _, r := range m.Resources {
		if r.ID == id {
			return r, true
		}
	}
	return Resource{}, false
}

// Validate checks structural invariants that do not need a catalog:
// namespace ownership of declared IDs and uniqueness.
func (m *Manifest) Validate() error {
	if m.Schema != SchemaVersion {
		return fmt.Errorf("manifest: unsupported schema %q", m.Schema)
	}
	if !namespacePattern.MatchString(m.Namespace) {
		return fmt.Errorf("manifest: invalid namespace %q", m.Namespace)
	}
	seen := map[string]bool{}
	for _, r := range m.Resources {
		if !idPattern.MatchString(r.ID) {
			return fmt.Errorf("manifest: invalid resource id %q", r.ID)
		}
		if !strings.HasPrefix(r.ID, m.Namespace+".") {
			return fmt.Errorf("manifest: resource %q is outside namespace %q", r.ID, m.Namespace)
		}
		if seen[r.ID] {
			return fmt.Errorf("manifest: resource %q declared twice", r.ID)
		}
		seen[r.ID] = true
		if r.Kind == ResEntity || r.Kind == ResOverlay {
			fields := map[string]bool{}
			for _, f := range r.Fields {
				if fields[f.ID] {
					return fmt.Errorf("manifest: %s: field %q declared twice", r.ID, f.ID)
				}
				fields[f.ID] = true
			}
		}
	}
	exports := map[string]bool{}
	for _, i := range m.Implementations {
		if exports[i.Export] {
			return fmt.Errorf("manifest: export %q implements twice", i.Export)
		}
		exports[i.Export] = true
	}
	return nil
}

// ContractHashOf hashes the public, compatibility-relevant shape of a
// resource. Labels and descriptions are excluded.
func ContractHashOf(r Resource) string {
	shape := struct {
		ID      string          `json:"id"`
		Kind    ResourceKind    `json:"kind"`
		Atomic  bool            `json:"atomic,omitempty"`
		Fields  []Field         `json:"fields,omitempty"`
		Base    string          `json:"base,omitempty"`
		Input   *Type           `json:"input,omitempty"`
		Output  *Type           `json:"output,omitempty"`
		Errors  []string        `json:"errors,omitempty"`
		Params  *Type           `json:"params,omitempty"`
		Payload *Type           `json:"payload,omitempty"`
		Subject string          `json:"subject,omitempty"`
		Signals map[string]Type `json:"signals,omitempty"`
		Path    string          `json:"path,omitempty"`
	}{r.ID, r.Kind, r.Atomic, nil, r.Base, r.Input, r.Output, r.Errors, r.Params, r.Payload, r.Subject, r.Signals, r.Path}
	for _, f := range r.Fields {
		f.Label = nil
		shape.Fields = append(shape.Fields, f)
	}
	b, _ := json.Marshal(shape)
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
