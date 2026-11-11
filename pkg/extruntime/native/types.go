package native

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
	"github.com/iota-uz/iota-sdk/pkg/extruntime/spi"
)

// TypeOf derives the wire type (manifest.Type) of a Go value type, so the
// contract of a native resource is written once, as the Go struct the
// handler uses. Rules:
//
//   - struct: object. Every exported field needs a `json:"name"` tag
//     (`json:"-"` skips it); embedded structs are not supported.
//   - pointer field: optional key; its tag must carry `omitempty` (nullable
//     values are not supported, matching t.x().optional()).
//   - string: string, or by `wire` tag: "int64", "decimal,precision=P,scale=S",
//     "date", "datetime", "uuid", "enum=a|b|c", "string,maxLength=N".
//   - bool: bool; int, int8..int32, uint8..uint32: int (safe JSON integer).
//   - int64/uint64: int64 only with `wire:"int64"` and the `,string` json
//     option (int64 travels as a decimal string).
//   - time.Time: datetime; uuid.UUID: uuid; json.RawMessage: json.
//   - spi.ObjectRef or manifest.RefValue: ref, with `wire:"ref=<entity id>"`.
//   - slice (not []byte): array of the element type; a wire tag applies to
//     the elements.
//
// Anything else (maps, interfaces, floats, recursive types) is an error:
// declare the type explicitly in Spec instead.
func TypeOf[T any]() (manifest.Type, error) {
	return typeOf(reflect.TypeFor[T](), "", nil)
}

var (
	timeType     = reflect.TypeFor[time.Time]()
	uuidType     = reflect.TypeFor[uuid.UUID]()
	rawType      = reflect.TypeFor[json.RawMessage]()
	objectRef    = reflect.TypeFor[spi.ObjectRef]()
	refValueType = reflect.TypeFor[manifest.RefValue]()
)

type wireTag struct {
	kind   string
	params map[string]string
}

func parseWire(tag string) (wireTag, error) {
	w := wireTag{params: map[string]string{}}
	if tag == "" {
		return w, nil
	}
	parts := strings.Split(tag, ",")
	head := parts[0]
	if k, v, ok := strings.Cut(head, "="); ok {
		w.kind = k
		w.params[k] = v
	} else {
		w.kind = head
	}
	for _, p := range parts[1:] {
		k, v, ok := strings.Cut(p, "=")
		if !ok {
			return w, fmt.Errorf("wire tag %q: %q is not key=value", tag, p)
		}
		w.params[k] = v
	}
	return w, nil
}

func (w wireTag) int(key string) (int, error) {
	v, ok := w.params[key]
	if !ok {
		return 0, nil
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 0 {
		return 0, fmt.Errorf("wire %s=%q is not a non-negative integer", key, v)
	}
	return n, nil
}

func typeOf(t reflect.Type, tag string, visiting []reflect.Type) (manifest.Type, error) {
	w, err := parseWire(tag)
	if err != nil {
		return manifest.Type{}, err
	}
	for _, v := range visiting {
		if v == t {
			return manifest.Type{}, fmt.Errorf("%s is recursive", t)
		}
	}
	switch {
	case t == timeType:
		if w.kind != "" && w.kind != "datetime" {
			return manifest.Type{}, fmt.Errorf("time.Time travels as datetime; wire %q needs a string field", w.kind)
		}
		return manifest.Type{Kind: manifest.KindDateTime}, nil
	case t == uuidType:
		return manifest.Type{Kind: manifest.KindUUID}, noWire(t, w)
	case t == rawType:
		return manifest.Type{Kind: manifest.KindJSON}, noWire(t, w)
	case t == objectRef || t == refValueType:
		if w.kind != "ref" || w.params["ref"] == "" {
			return manifest.Type{}, fmt.Errorf("%s needs `wire:\"ref=<entity id>\"`", t)
		}
		return manifest.Type{Kind: manifest.KindRef, Entity: w.params["ref"]}, nil
	}
	switch t.Kind() {
	case reflect.String:
		return stringType(w)
	case reflect.Bool:
		return manifest.Type{Kind: manifest.KindBool}, noWire(t, w)
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Uint8, reflect.Uint16, reflect.Uint32:
		// int is 64-bit on the supported platforms; a value beyond 2^53 fails
		// wire validation instead of losing precision.
		return manifest.Type{Kind: manifest.KindInt}, noWire(t, w)
	case reflect.Int64, reflect.Uint64:
		return manifest.Type{}, fmt.Errorf("%s: use `wire:\"int64\"` with the `,string` json option (int64 travels as a decimal string)", t)
	case reflect.Slice:
		if t.Elem().Kind() == reflect.Uint8 {
			return manifest.Type{}, fmt.Errorf("%s is not supported (encode binary as a string)", t)
		}
		items, err := typeOf(t.Elem(), tag, append(visiting, t))
		if err != nil {
			return manifest.Type{}, err
		}
		return manifest.Type{Kind: manifest.KindArray, Items: &items}, nil
	case reflect.Struct:
		if err := noWire(t, w); err != nil {
			return manifest.Type{}, err
		}
		return objectType(t, append(visiting, t))
	}
	return manifest.Type{}, fmt.Errorf("%s (%s) has no wire type; declare the contract type explicitly", t, t.Kind())
}

func noWire(t reflect.Type, w wireTag) error {
	if w.kind != "" {
		return fmt.Errorf("wire %q does not apply to %s", w.kind, t)
	}
	return nil
}

func stringType(w wireTag) (manifest.Type, error) {
	switch w.kind {
	case "", "string":
		n, err := w.int("maxLength")
		return manifest.Type{Kind: manifest.KindString, MaxLength: n}, err
	case "int64":
		return manifest.Type{Kind: manifest.KindInt64}, nil
	case "decimal":
		p, err := w.int("precision")
		if err != nil {
			return manifest.Type{}, err
		}
		s, err := w.int("scale")
		return manifest.Type{Kind: manifest.KindDecimal, Precision: p, Scale: s}, err
	case "date":
		return manifest.Type{Kind: manifest.KindDate}, nil
	case "datetime":
		return manifest.Type{Kind: manifest.KindDateTime}, nil
	case "uuid":
		return manifest.Type{Kind: manifest.KindUUID}, nil
	case "enum":
		vals := strings.Split(w.params["enum"], "|")
		if len(vals) == 0 || vals[0] == "" {
			return manifest.Type{}, fmt.Errorf("wire enum needs values: `wire:\"enum=a|b\"`")
		}
		return manifest.Type{Kind: manifest.KindEnum, Values: vals}, nil
	}
	return manifest.Type{}, fmt.Errorf("unknown wire type %q for a string field", w.kind)
}

func objectType(t reflect.Type, visiting []reflect.Type) (manifest.Type, error) {
	out := manifest.Type{Kind: manifest.KindObject}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			return manifest.Type{}, fmt.Errorf("%s.%s: embedded fields are not supported", t, f.Name)
		}
		if !f.IsExported() {
			continue
		}
		jt, ok := f.Tag.Lookup("json")
		if !ok {
			return manifest.Type{}, fmt.Errorf("%s.%s needs a json tag", t, f.Name)
		}
		name, rest, _ := strings.Cut(jt, ",")
		if name == "-" && rest == "" {
			continue
		}
		if name == "" {
			return manifest.Type{}, fmt.Errorf("%s.%s: json tag needs a name", t, f.Name)
		}
		opts := strings.Split(rest, ",")
		has := func(o string) bool {
			for _, x := range opts {
				if x == o {
					return true
				}
			}
			return false
		}
		ft := f.Type
		optional := false
		if ft.Kind() == reflect.Pointer {
			if !has("omitempty") {
				return manifest.Type{}, fmt.Errorf("%s.%s: a pointer field is an optional key and needs `omitempty` (nullable values are not supported)", t, f.Name)
			}
			ft, optional = ft.Elem(), true
		}
		var pt manifest.Type
		var err error
		wire := f.Tag.Get("wire")
		if (ft.Kind() == reflect.Int64 || ft.Kind() == reflect.Uint64) && ft != timeType {
			if wire != "int64" || !has("string") {
				return manifest.Type{}, fmt.Errorf("%s.%s (%s): use `wire:\"int64\"` with the `,string` json option (int64 travels as a decimal string)", t, f.Name, ft)
			}
			pt = manifest.Type{Kind: manifest.KindInt64}
		} else {
			if has("string") {
				return manifest.Type{}, fmt.Errorf("%s.%s: the `,string` json option is only supported on int64 fields", t, f.Name)
			}
			pt, err = typeOf(ft, wire, visiting)
			if err != nil {
				return manifest.Type{}, fmt.Errorf("%s.%s: %w", t, f.Name, err)
			}
		}
		if !optional && has("omitempty") {
			return manifest.Type{}, fmt.Errorf("%s.%s: omitempty on a required field would drop zero values; make it a pointer to declare an optional key", t, f.Name)
		}
		pt.Optional = optional
		out.Properties = append(out.Properties, manifest.Property{Name: name, Type: pt})
	}
	return out, nil
}
