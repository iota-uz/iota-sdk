package openapi

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Schema is a JSON Schema (2020-12) object.
type Schema = map[string]any

var (
	timeType = reflect.TypeFor[time.Time]()
	uuidType = reflect.TypeFor[uuid.UUID]()
	rawType  = reflect.TypeFor[json.RawMessage]()
)

// Registry derives JSON Schemas from Go types and collects them as OpenAPI
// components. A named struct becomes one component referenced by "$ref", so
// recursive types and types shared by many operations are described once.
type Registry struct {
	mu      sync.Mutex
	schemas map[string]Schema
	byType  map[reflect.Type]string
	owner   map[string]reflect.Type
}

// NewRegistry returns an empty registry.
func NewRegistry() *Registry {
	return &Registry{schemas: map[string]Schema{}, byType: map[reflect.Type]string{}, owner: map[string]reflect.Type{}}
}

// Ref is a reference to a component schema.
func Ref(name string) Schema { return Schema{"$ref": "#/components/schemas/" + name} }

// Add registers a hand-written component schema.
func (r *Registry) Add(name string, s Schema) Schema {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.schemas[name] = s
	return Ref(name)
}

// Schemas returns the collected components.
func (r *Registry) Schemas() map[string]Schema {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]Schema, len(r.schemas))
	for k, v := range r.schemas {
		out[k] = v
	}
	return out
}

// Of returns the schema of t (a reference for a named struct). The wire form
// is what encoding/json produces.
func Of[T any](r *Registry) Schema { return r.Of(reflect.TypeFor[T]()) }

// Of returns the schema of t.
func (r *Registry) Of(t reflect.Type) Schema {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.of(t)
}

func (r *Registry) of(t reflect.Type) Schema {
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	switch t {
	case timeType:
		return Schema{"type": "string", "format": "date-time"}
	case uuidType:
		return Schema{"type": "string", "format": "uuid"}
	case rawType:
		return Schema{}
	}
	switch t.Kind() {
	case reflect.Bool:
		return Schema{"type": "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		return Schema{"type": "integer"}
	case reflect.Int64, reflect.Uint64:
		return Schema{"type": "integer", "format": "int64"}
	case reflect.Float32, reflect.Float64:
		return Schema{"type": "number"}
	case reflect.String:
		return Schema{"type": "string"}
	case reflect.Slice, reflect.Array:
		if t.Elem().Kind() == reflect.Uint8 {
			return Schema{"type": "string", "contentEncoding": "base64"}
		}
		return Schema{"type": "array", "items": r.of(t.Elem())}
	case reflect.Map:
		return Schema{"type": "object", "additionalProperties": r.of(t.Elem())}
	case reflect.Struct:
		return r.structSchema(t)
	case reflect.Interface:
		return Schema{}
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Chan, reflect.Func, reflect.Pointer, reflect.UnsafePointer:
		return Schema{}
	}
	return Schema{}
}

func (r *Registry) structSchema(t reflect.Type) Schema {
	if t.Name() == "" {
		return r.buildStruct(t)
	}
	if name, ok := r.byType[t]; ok {
		return Ref(name)
	}
	name := t.Name()
	if owner, taken := r.owner[name]; taken && owner != t {
		name = pascal(lastSegment(t.PkgPath())) + name
	}
	r.byType[t] = name
	r.owner[name] = t
	r.schemas[name] = Schema{} // reserved: a recursive type refers to itself
	r.schemas[name] = r.buildStruct(t)
	return Ref(name)
}

func lastSegment(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}

func pascal(s string) string {
	if s == "" {
		return s
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func (r *Registry) buildStruct(t reflect.Type) Schema {
	props := Schema{}
	var required []string
	r.collect(t, props, &required)
	s := Schema{"type": "object", "properties": props}
	if len(required) > 0 {
		sort.Strings(required)
		req := make([]any, len(required))
		for i, n := range required {
			req[i] = n
		}
		s["required"] = req
	}
	return s
}

func (r *Registry) collect(t reflect.Type, props Schema, required *[]string) {
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		tag := f.Tag.Get("json")
		if tag == "-" {
			continue
		}
		name, opts, _ := strings.Cut(tag, ",")
		if f.Anonymous && name == "" {
			ft := f.Type
			for ft.Kind() == reflect.Pointer {
				ft = ft.Elem()
			}
			if ft.Kind() == reflect.Struct {
				r.collect(ft, props, required)
				continue
			}
		}
		if !f.IsExported() {
			continue
		}
		if name == "" {
			name = f.Name
		}
		fs := r.of(f.Type)
		if strings.Contains(","+opts+",", ",string,") {
			fs = Schema{"type": "string"}
		}
		omit := strings.Contains(","+opts+",", ",omitempty,") || strings.Contains(","+opts+",", ",omitzero,")
		if f.Type.Kind() == reflect.Pointer && !omit {
			fs = Schema{"oneOf": []any{fs, Schema{"type": "null"}}}
		}
		props[name] = fs
		if !omit {
			*required = append(*required, name)
		}
	}
}
