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
	Name   string
	Fields []Field
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

func cloneDefinition(d Definition) Definition {
	resources := make([]Resource, len(d.Resources))
	for i, r := range d.Resources {
		resources[i] = Resource{Name: r.Name, Fields: append([]Field(nil), r.Fields...)}
	}
	d.Resources = resources
	return d
}
