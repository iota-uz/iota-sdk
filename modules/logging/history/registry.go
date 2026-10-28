// Package history records business actions: a versioned registry of event types and a
// writer that stores events in the caller's business transaction.
package history

import (
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
}

type Resource struct {
	Name string
	// LabelKey names the resource in the history UI; the name is shown when it is empty.
	LabelKey string
	// ReadPermission hides events about the resource, and its subjects in other events, from readers without it.
	ReadPermission *permission.Permission
	Fields         []Field
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
}

func NewRegistry() *Registry {
	return &Registry{definitions: make(map[definitionKey]Definition)}
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

// Resources lists every resource named by a registered event type, sorted by name.
func (r *Registry) Resources() []Resource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	byName := map[string]Resource{}
	for _, d := range r.definitions {
		for _, res := range d.Resources {
			if known, ok := byName[res.Name]; !ok || known.LabelKey == "" {
				byName[res.Name] = Resource{Name: res.Name, LabelKey: res.LabelKey}
			}
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
// keyPrefix.Created.Title, keyPrefix.Created.Summary and so on.
func Lifecycle(typePrefix, keyPrefix string, resource Resource) []Definition {
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
