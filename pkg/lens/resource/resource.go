// Package resource accounts retained Lens payloads and admission reservations.
// Its structural estimates are conservative accounting units, not Go heap limits.
package resource

import (
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/google/uuid"
	"reflect"
	"sync"
	"time"
)

const DefaultLimit int64 = 64 << 20
const DefaultEntryLimit int64 = 16 << 20

var ErrLimit = errors.New("lens resource limit exceeded")

type LimitKind string

const (
	LimitPayload   LimitKind = "payload"
	LimitAdmission LimitKind = "admission"
)

type LimitError struct {
	Kind             LimitKind
	Resource         string
	Requested, Limit int64
}

func (e *LimitError) Error() string {
	return fmt.Sprintf("%s: requested %d, limit %d: %v", e.Resource, e.Requested, e.Limit, ErrLimit)
}
func (e *LimitError) Unwrap() error { return ErrLimit }

// NewLimitError reports an individually oversized or invalid payload.
func NewLimitError(name string, n, limit int64) error {
	return &LimitError{Kind: LimitPayload, Resource: name, Requested: n, Limit: limit}
}

// NewAdmissionError reports temporary shared capacity pressure.
func NewAdmissionError(name string, n, limit int64) error {
	return &LimitError{Kind: LimitAdmission, Resource: name, Requested: n, Limit: limit}
}

type Budget struct {
	mu          sync.Mutex
	used, limit int64
}
type Reservation struct {
	released bool
	budget   *Budget
	bytes    int64
}

func NewBudget(limit int64) *Budget {
	if limit <= 0 {
		limit = DefaultLimit
	}
	return &Budget{limit: limit}
}
func (b *Budget) Reserve(n int64) (*Reservation, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if n < 0 || n > b.limit-b.used {
		return nil, NewAdmissionError("retained bytes", n, b.limit)
	}
	b.used += n
	return &Reservation{budget: b, bytes: n}, nil
}
func (r *Reservation) Release() {
	if r == nil {
		return
	}
	b := r.budget
	b.mu.Lock()
	defer b.mu.Unlock()
	if !r.released {
		b.used -= r.bytes
		r.released = true
	}
}
func (b *Budget) Used() int64  { b.mu.Lock(); defer b.mu.Unlock(); return b.used }
func (b *Budget) Limit() int64 { return b.limit }

// Estimate rejects cycles, unsupported mutable values and excessive nesting
// before a clone or JSON encoder can allocate a complete duplicate payload.
func Estimate(value any, limit int64) (int64, error) {
	if limit <= 0 {
		limit = DefaultEntryLimit
	}
	e := estimator{limit: limit, path: map[visit]bool{}}
	err := e.walk(reflect.ValueOf(value), 0)
	return e.size, err
}

// EstimateJSON guards serialization without claiming ownership of opaque,
// JSON-ignored state. Ownership boundaries must use Estimate instead.
func EstimateJSON(value any, limit int64) (int64, error) {
	if limit <= 0 {
		limit = DefaultEntryLimit
	}
	e := estimator{limit: limit, path: map[visit]bool{}, serialization: true}
	err := e.walk(reflect.ValueOf(value), 0)
	return e.size, err
}

// EstimateProjected performs strict ownership preflight except for exact types
// the caller removes entirely from its owned projection. It does not ignore
// json:"-" fields or any other retained state. Omitted values cost128 units.
func EstimateProjected(value any, limit int64, omittedTypes ...reflect.Type) (int64, error) {
	if limit <= 0 {
		limit = DefaultEntryLimit
	}
	omitted := make(map[reflect.Type]struct{}, len(omittedTypes))
	for _, kind := range omittedTypes {
		if kind != nil {
			omitted[kind] = struct{}{}
		}
	}
	e := estimator{limit: limit, path: map[visit]bool{}, omitted: omitted}
	err := e.walk(reflect.ValueOf(value), 0)
	return e.size, err
}

type visit struct {
	typ reflect.Type
	ptr uintptr
}
type estimator struct {
	size, limit   int64
	nodes         int
	serialization bool
	omitted       map[reflect.Type]struct{}
	path          map[visit]bool
}

func (e *estimator) add(n int64) error {
	if n < 0 || n > e.limit-e.size {
		return NewLimitError("entry bytes", e.size+n, e.limit)
	}
	e.size += n
	return nil
}
func (e *estimator) walk(v reflect.Value, depth int) error {
	e.nodes++
	if e.nodes > 1000000 {
		return NewLimitError("structural_nodes", int64(e.nodes), 1000000)
	}
	if depth > 128 {
		return NewLimitError("structural_depth", int64(depth), 128)
	}
	if !v.IsValid() {
		return e.add(1)
	}
	if _, omitted := e.omitted[v.Type()]; omitted {
		return e.add(128)
	}
	if size, native, err := decimalEstimate(v, e.limit-e.size); native {
		if err != nil {
			return err
		}
		return e.add(size)
	}
	if v.Type() == reflect.TypeFor[time.Time]() {
		return e.add(64)
	}
	if v.Type() == reflect.TypeFor[*time.Time]() {
		return e.add(96)
	}
	if v.Type() == reflect.TypeFor[uuid.UUID]() {
		return e.add(64)
	}
	if v.Type() == reflect.TypeFor[*uuid.UUID]() {
		return e.add(96)
	}
	if v.Type() == reflect.TypeFor[uuid.NullUUID]() {
		return e.add(96)
	}
	if v.Type() == reflect.TypeFor[*uuid.NullUUID]() {
		return e.add(128)
	}
	if v.Type() != reflect.TypeFor[json.RawMessage]() && (v.Type().Implements(reflect.TypeFor[json.Marshaler]()) || reflect.PointerTo(v.Type()).Implements(reflect.TypeFor[json.Marshaler]()) || v.Type().Implements(reflect.TypeFor[encoding.TextMarshaler]()) || reflect.PointerTo(v.Type()).Implements(reflect.TypeFor[encoding.TextMarshaler]())) {
		return fmt.Errorf("unsupported custom lens JSON marshaler %s", v.Type())
	}
	//nolint:exhaustive // Only reference and interface kinds need cycle tracking or unwrapping.
	switch v.Kind() {
	case reflect.Interface:
		if v.IsNil() {
			return e.add(1)
		}
		return e.walk(v.Elem(), depth+1)
	case reflect.Pointer, reflect.Map, reflect.Slice:
		if v.IsNil() {
			if v.Kind() != reflect.Pointer {
				return e.add(32)
			}
			return e.add(1)
		}
		key := visit{v.Type(), v.Pointer()}
		if e.path[key] {
			return fmt.Errorf("cyclic lens value")
		}
		e.path[key] = true
		defer delete(e.path, key)
	}
	if err := e.add(32); err != nil {
		return err
	}
	//nolint:exhaustive // Unsupported wire kinds are explicitly rejected by default.
	switch v.Kind() {
	case reflect.Pointer:
		return e.walk(v.Elem(), depth+1)
	case reflect.String:
		return e.add(int64(v.Len()) * 6)
	case reflect.Map:
		if v.Type().Key().Kind() != reflect.String {
			return fmt.Errorf("unsupported lens map key %s", v.Type().Key())
		}
		iter := v.MapRange()
		for iter.Next() {
			if err := e.walk(iter.Key(), depth+1); err != nil {
				return err
			}
			if err := e.walk(iter.Value(), depth+1); err != nil {
				return err
			}
		}
	case reflect.Slice, reflect.Array:
		if v.Kind() == reflect.Slice && v.Type().Elem().Kind() == reflect.Uint8 && !hasCustomEncoder(v.Type().Elem()) {
			multiplier := int64(2)
			if v.Type() == reflect.TypeFor[json.RawMessage]() {
				multiplier = 6
			}
			return e.add(int64(v.Len()) * multiplier)
		}
		for i := 0; i < v.Len(); i++ {
			if err := e.walk(v.Index(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Struct:
		for i := 0; i < v.NumField(); i++ {
			f := v.Type().Field(i)
			if e.serialization && (f.PkgPath != "" || f.Tag.Get("json") == "-") {
				continue
			}
			if f.PkgPath != "" && v.Type().PkgPath() == "github.com/iota-uz/iota-sdk/pkg/lens/frame" && v.Type().Name() == "Frame" && f.Name == "fieldIndex" {
				if err := e.add(int64(v.Field(i).Len()) * 64); err != nil {
					return err
				}
				continue
			}
			if f.PkgPath != "" && v.Field(i).IsZero() {
				if err := e.add(32); err != nil {
					return err
				}
				continue
			}
			if f.PkgPath != "" {
				return fmt.Errorf("unsupported lens value with private fields: %s", v.Type())
			}
			if err := e.walk(v.Field(i), depth+1); err != nil {
				return err
			}
		}
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64, reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Float32, reflect.Float64:
	default:
		return fmt.Errorf("unsupported lens value %s", v.Kind())
	}
	return nil
}

// StatusCode distinguishes admission pressure from an individually oversized result.
func StatusCode(err error) (int, bool) {
	var e *LimitError
	if !errors.As(err, &e) {
		return 0, false
	}
	if e.Kind == LimitAdmission {
		return 503, true
	}
	return 413, true
}

// Replace atomically changes an existing reservation without double charging it.
func (b *Budget) Replace(old *Reservation, n int64) (*Reservation, error) {
	if old == nil {
		return b.Reserve(n)
	}
	b.mu.Lock()
	defer b.mu.Unlock()
	if old.budget != b || old.released {
		return nil, fmt.Errorf("invalid lens reservation replacement")
	}
	if n < 0 || n-old.bytes > b.limit-b.used {
		return nil, NewAdmissionError("retained bytes", n, b.limit)
	}
	b.used += n - old.bytes
	old.bytes = n
	return old, nil
}

func hasCustomEncoder(kind reflect.Type) bool {
	jsonEncoder, textEncoder := reflect.TypeFor[json.Marshaler](), reflect.TypeFor[encoding.TextMarshaler]()
	return kind.Implements(jsonEncoder) || reflect.PointerTo(kind).Implements(jsonEncoder) || kind.Implements(textEncoder) || reflect.PointerTo(kind).Implements(textEncoder)
}
