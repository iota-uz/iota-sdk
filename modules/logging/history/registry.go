// Package history records business actions: a versioned registry of event types and a
// writer that stores events in the caller's business transaction.
package history

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"sync"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/logging/domain/aggregates/businessevent"
)

var (
	eventTypePattern = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)+$`)
	namePattern      = regexp.MustCompile(`^[a-z][a-z0-9_]*(\.[a-z][a-z0-9_]*)*$`)
)

type ValueType string

const (
	ValueString   ValueType = "string"
	ValueInteger  ValueType = "integer"
	ValueDecimal  ValueType = "decimal"
	ValueBoolean  ValueType = "boolean"
	ValueDateTime ValueType = "datetime"
	ValueDate     ValueType = "date"
	ValueMoney    ValueType = "money"
)

type Field struct {
	Name     string
	LabelKey string
	Type     ValueType
	// Redact keeps only the fact that the field changed; its values are dropped before storage.
	Redact bool
	// ReadPermission hides stored values from readers without it.
	ReadPermission *permission.Permission
	// ValueKeyPrefix translates a stored code (an enumeration value such as a status or a type) for
	// reading: the value v is shown as the translation of ValueKeyPrefix + "." + v. A code without a
	// translation is shown as stored. Only string, integer and boolean fields take it.
	ValueKeyPrefix string
	// ValueLabel translates a stored code like ValueKeyPrefix, for codes a key cannot name. It wins
	// over ValueKeyPrefix; an empty result falls back to it, and then to the code itself.
	ValueLabel ValueLabelFunc
}

// ValueLabelFunc returns the reader-facing text of a stored code in the locale of ctx, or "" if it has none.
type ValueLabelFunc func(ctx context.Context, value string) string

// SubjectLabelsFunc returns the current captions of the given objects of one resource, keyed by ID,
// such as a document number or a person's name. It is called once per resource per page with every
// ID shown on it; IDs it omits keep the caption recorded with the event, if any, or their ID.
type SubjectLabelsFunc func(ctx context.Context, ids []string) (map[string]string, error)

// VisibilityFunc tells which objects of a resource the reader in ctx may see. It is called once per
// read, before pagination.
type VisibilityFunc func(ctx context.Context) (Visibility, error)

// Visibility is the set of objects of one resource a reader may see: every object, the listed IDs,
// the objects matching an SQL condition, or the union of the last two. The zero value shows none.
type Visibility struct {
	All bool
	IDs []string
	// Condition is an SQL boolean expression over {id}, the object ID as text, evaluated in the
	// tenant's database, e.g. `{id}::uuid IN (SELECT document_id FROM participants WHERE user_id = $1)`.
	// Its arguments are $1..$n of Args. The expression must not contain other placeholders.
	Condition string
	Args      []any
}

// VisibleAll lets the reader see every object of the resource.
func VisibleAll() Visibility { return Visibility{All: true} }

// VisibleIDs lets the reader see the listed objects only.
func VisibleIDs(ids ...string) Visibility { return Visibility{IDs: ids} }

// VisibleWhere lets the reader see the objects whose ID satisfies an SQL condition; see Visibility.Condition.
func VisibleWhere(condition string, args ...any) Visibility {
	return Visibility{Condition: condition, Args: args}
}

type Resource struct {
	Name string
	// LabelKey is the translation key of the resource's plural name, shown in the history filters and
	// event subjects. At least one definition naming the resource must set it; Lifecycle defaults it
	// to keyPrefix.Resource.
	LabelKey string
	// ReadPermission hides events about the resource, and its subjects in other events, from readers without it.
	ReadPermission *permission.Permission
	// Labels, when set, captions the objects of the resource on the history page and in its exports
	// with one batched call per page. An object it no longer knows, e.g. a deleted one, keeps the
	// caption recorded with its latest event shown (businessevent.Subject.Label).
	Labels SubjectLabelsFunc
	// Visible, when set, limits the reader to some objects of the resource: an event naming an object
	// outside the set is hidden as a whole, before pagination, so pages and cursors only count events
	// the reader may see. Without it every object of a readable resource is visible.
	Visible VisibilityFunc
	Fields  []Field
}

type Definition struct {
	Type           string
	Version        int
	Category       businessevent.Category
	TitleKey       string
	DescriptionKey string
	Resources      []Resource
	// SubjectOptional allows events that concern no particular object.
	SubjectOptional bool
}

func (d Definition) resource(name string) (Resource, bool) {
	for _, r := range d.Resources {
		if r.Name == name {
			return r, true
		}
	}
	return Resource{}, false
}

func (r Resource) field(name string) (Field, bool) {
	for _, f := range r.Fields {
		if f.Name == name {
			return f, true
		}
	}
	return Field{}, false
}

func (d Definition) validate() error {
	if !eventTypePattern.MatchString(d.Type) || d.Version < 1 || !d.Category.Valid() || d.TitleKey == "" || d.DescriptionKey == "" {
		return fmt.Errorf("invalid business event definition %s v%d", d.Type, d.Version)
	}
	if len(d.Resources) == 0 && !d.SubjectOptional {
		return fmt.Errorf("business event %s v%d has no resources", d.Type, d.Version)
	}
	resources := map[string]bool{}
	for _, r := range d.Resources {
		if !namePattern.MatchString(r.Name) || resources[r.Name] {
			return fmt.Errorf("invalid resource %q in business event %s v%d", r.Name, d.Type, d.Version)
		}
		resources[r.Name] = true
		fields := map[string]bool{}
		for _, f := range r.Fields {
			if f.Name == "" || fields[f.Name] || f.LabelKey == "" || !f.Type.valid() {
				return fmt.Errorf("invalid field %q of %s in business event %s v%d", f.Name, r.Name, d.Type, d.Version)
			}
			if (f.ValueKeyPrefix != "" || f.ValueLabel != nil) && f.Type != ValueString && f.Type != ValueInteger && f.Type != ValueBoolean {
				return fmt.Errorf("field %q of %s in business event %s v%d is a %s and cannot translate its values", f.Name, r.Name, d.Type, d.Version, f.Type)
			}
			fields[f.Name] = true
		}
	}
	return nil
}

type definitionKey struct {
	eventType string
	version   int
}

// Registry holds every version of every event type. Old versions stay registered so
// history written with them remains readable.
type Registry struct {
	mu          sync.RWMutex
	definitions map[definitionKey]Definition
	labelKeys   map[string]string
	labels      map[string]SubjectLabelsFunc
	visibility  map[string]VisibilityFunc
}

func NewRegistry() *Registry {
	return &Registry{
		definitions: make(map[definitionKey]Definition),
		labelKeys:   make(map[string]string),
		labels:      make(map[string]SubjectLabelsFunc),
		visibility:  make(map[string]VisibilityFunc),
	}
}

func (r *Registry) Register(defs ...Definition) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, d := range defs {
		if err := d.validate(); err != nil {
			return err
		}
		key := definitionKey{d.Type, d.Version}
		if _, exists := r.definitions[key]; exists {
			return fmt.Errorf("business event already registered: %s v%d", d.Type, d.Version)
		}
		for _, res := range d.Resources {
			if known := r.labelKeys[res.Name]; res.LabelKey != "" && known != "" && known != res.LabelKey {
				return fmt.Errorf("resource %q of business event %s v%d is labelled %q, already %q", res.Name, d.Type, d.Version, res.LabelKey, known)
			}
		}
		for _, res := range d.Resources {
			if res.LabelKey != "" {
				r.labelKeys[res.Name] = res.LabelKey
			}
			// A resource's caption provider and visibility apply to every event type naming it; the
			// first definition that sets one provides it.
			if _, ok := r.labels[res.Name]; !ok && res.Labels != nil {
				r.labels[res.Name] = res.Labels
			}
			if _, ok := r.visibility[res.Name]; !ok && res.Visible != nil {
				r.visibility[res.Name] = res.Visible
			}
		}
		r.definitions[key] = cloneDefinition(d)
	}
	return nil
}

func (r *Registry) Get(eventType string, version int) (Definition, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	d, ok := r.definitions[definitionKey{eventType, version}]
	return cloneDefinition(d), ok
}

func (r *Registry) Definitions() []Definition {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]Definition, 0, len(r.definitions))
	for _, d := range r.definitions {
		result = append(result, cloneDefinition(d))
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Type != result[j].Type {
			return result[i].Type < result[j].Type
		}
		return result[i].Version < result[j].Version
	})
	return result
}

// Resources lists every resource named by a registered event type, sorted by name, with its label
// key, caption provider and visibility.
func (r *Registry) Resources() []Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	byName := map[string]Resource{}
	for _, d := range r.definitions {
		for _, res := range d.Resources {
			byName[res.Name] = Resource{Name: res.Name, LabelKey: r.labelKeys[res.Name], Labels: r.labels[res.Name], Visible: r.visibility[res.Name]}
		}
	}
	result := make([]Resource, 0, len(byName))
	for _, res := range byName {
		result = append(result, res)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// Readable reports whether a reader may see events about the resource. A permission declared
// for it by any event type applies to all of them.
func (r *Registry) Readable(resource string, can func(permission.Permission) bool) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, d := range r.definitions {
		for _, res := range d.Resources {
			if res.Name == resource && res.ReadPermission != nil && !can(*res.ReadPermission) {
				return false
			}
		}
	}
	return true
}

func cloneDefinition(d Definition) Definition {
	resources := make([]Resource, len(d.Resources))
	for i, r := range d.Resources {
		r.Fields = append([]Field(nil), r.Fields...)
		resources[i] = r
	}
	d.Resources = resources
	return d
}

// Lifecycle defines the created, updated and deleted events of one resource: types are
// typePrefix.created, .updated and .deleted, and translation keys are
// keyPrefix.Created.Title, keyPrefix.Created.Summary and so on. The resource is named by
// keyPrefix.Resource unless it sets its own LabelKey.
func Lifecycle(typePrefix, keyPrefix string, resource Resource) []Definition {
	if resource.LabelKey == "" {
		resource.LabelKey = keyPrefix + ".Resource"
	}
	actions := []struct{ suffix, key string }{{"created", "Created"}, {"updated", "Updated"}, {"deleted", "Deleted"}}
	defs := make([]Definition, len(actions))
	for i, a := range actions {
		defs[i] = Definition{
			Type:           typePrefix + "." + a.suffix,
			Version:        1,
			Category:       businessevent.CategoryBusiness,
			TitleKey:       keyPrefix + "." + a.key + ".Title",
			DescriptionKey: keyPrefix + "." + a.key + ".Summary",
			Resources:      []Resource{resource},
		}
	}
	return defs
}

// WithResource applies configure to every resource named name in defs, e.g. to attach a caption
// provider or visibility that needs dependencies the module's static definitions do not have.
func WithResource(defs []Definition, name string, configure func(*Resource)) []Definition {
	result := make([]Definition, len(defs))
	for i, d := range defs {
		d = cloneDefinition(d)
		for j := range d.Resources {
			if d.Resources[j].Name == name {
				configure(&d.Resources[j])
			}
		}
		result[i] = d
	}
	return result
}
