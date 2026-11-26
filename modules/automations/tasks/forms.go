package tasks

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/iota-uz/iota-sdk/pkg/composition"
)

// ContributeTaskForm registers a TaskForm with the automations module of the composition.
func ContributeTaskForm(b *composition.Builder, f TaskForm) {
	if f == nil {
		return
	}
	composition.Contribute(b, func(*composition.Container) ([]TaskForm, error) { return []TaskForm{f}, nil })
}

// ContributeTaskForms registers TaskForms a factory builds from the container (a form that needs a
// connector's client, say). The factory runs once when the application is composed.
func ContributeTaskForms(b *composition.Builder, factory func(*composition.Container) ([]TaskForm, error)) {
	if factory == nil {
		return
	}
	composition.Contribute(b, factory)
}

// FormLookup finds the TaskForm of a kind.
type FormLookup interface {
	Form(kind string) (TaskForm, bool, error)
}

// Forms is the registry of an application's TaskForms. A registry over a container collects the
// contributions on first use (they are read after the container is compiled); a duplicate kind is an
// error of every lookup, and Check reports it when the application starts.
type Forms struct {
	load func() ([]TaskForm, error)

	once   sync.Once
	byKind map[string]TaskForm
	err    error
}

// NewForms is a registry of the given forms.
func NewForms(list ...TaskForm) *Forms {
	return &Forms{load: func() ([]TaskForm, error) { return list, nil }}
}

// FormsOf is the registry of the forms contributed to the container (ContributeTaskForm).
func FormsOf(c *composition.Container) *Forms {
	return &Forms{load: func() ([]TaskForm, error) { return composition.Contributions[TaskForm](c) }}
}

func (f *Forms) init() {
	f.once.Do(func() {
		list, err := f.load()
		if err != nil {
			f.err = err
			return
		}
		f.byKind = map[string]TaskForm{}
		for _, form := range list {
			if form == nil {
				continue
			}
			kind := strings.TrimSpace(form.Kind())
			if kind == "" {
				f.err = fmt.Errorf("automations: a task form has no kind")
				return
			}
			if _, dup := f.byKind[kind]; dup {
				f.err = fmt.Errorf("automations: task form %q is contributed twice", kind)
				return
			}
			f.byKind[kind] = form
		}
	})
}

// Check reports a registry that cannot be built: a duplicate or empty kind.
func (f *Forms) Check() error {
	if f == nil {
		return nil
	}
	f.init()
	return f.err
}

// Form implements FormLookup.
func (f *Forms) Form(kind string) (TaskForm, bool, error) {
	if f == nil {
		return nil, false, nil
	}
	f.init()
	if f.err != nil {
		return nil, false, f.err
	}
	form, ok := f.byKind[kind]
	return form, ok, nil
}

// Kinds are the registered kinds, sorted.
func (f *Forms) Kinds() []string {
	if f == nil {
		return nil
	}
	f.init()
	out := make([]string, 0, len(f.byKind))
	for k := range f.byKind {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// FormKindOf reads the annotation x-iota-form of a human node's form (a JSON Schema): "" when there is
// none; ok is false for an annotation that is not a non-empty string.
func FormKindOf(form json.RawMessage) (string, bool) {
	if len(form) == 0 {
		return "", true
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(form, &doc); err != nil {
		return "", true
	}
	raw, present := doc[FormAnnotation]
	if !present {
		return "", true
	}
	var kind string
	if err := json.Unmarshal(raw, &kind); err != nil || strings.TrimSpace(kind) == "" {
		return "", false
	}
	return strings.TrimSpace(kind), true
}

// openKeys are the keywords of a form schema that constrain no field: an object schema of only these
// (and annotations, "x-…") accepts any fields object.
var openKeys = map[string]bool{
	"type": true, "title": true, "description": true, "$schema": true, "$id": true, "$comment": true,
	"examples": true, "default": true, "readOnly": true, "writeOnly": true, "deprecated": true,
}

// openSubschema is a subschema every value passes: true, or {} (annotations aside).
func openSubschema(raw json.RawMessage) bool {
	if strings.TrimSpace(string(raw)) == "true" {
		return true
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false
	}
	for k := range doc {
		if !strings.HasPrefix(k, "x-") && (!openKeys[k] || k == "type") {
			return false
		}
	}
	return true
}

// ConstrainsFields reports a human node's form schema that constrains the decision's fields: anything
// beyond an object type and annotations. A TaskForm without FormPreparer decides only a task whose
// schema does not, as its action runs before its fields can be checked.
func ConstrainsFields(schema json.RawMessage) bool {
	if len(schema) == 0 {
		return false
	}
	var doc map[string]json.RawMessage
	if err := json.Unmarshal(schema, &doc); err != nil {
		return true
	}
	for k, raw := range doc {
		switch {
		case strings.HasPrefix(k, "x-"), openKeys[k] && k != "type":
		case k == "type":
			var typ string
			if err := json.Unmarshal(raw, &typ); err != nil || typ != "object" {
				return true
			}
		case k == "additionalProperties" && openSubschema(raw):
		default:
			return true
		}
	}
	return false
}
