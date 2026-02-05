package applet

import (
	"fmt"
	"reflect"
	"sort"
	"strings"
)

type TypedRouterDescription struct {
	Methods []TypedMethodDescription   `json:"methods"`
	Types   map[string]TypedTypeObject `json:"types"`
}

type TypedMethodDescription struct {
	Name               string   `json:"name"`
	RequirePermissions []string `json:"requirePermissions,omitempty"`
	Params             TypeRef  `json:"params"`
	Result             TypeRef  `json:"result"`
}

type TypedTypeObject struct {
	Fields []TypedField `json:"fields"`
}

type TypedField struct {
	Name     string  `json:"name"`
	Optional bool    `json:"optional"`
	Type     TypeRef `json:"type"`
}

type TypeRef struct {
	Kind  string    `json:"kind"`
	Name  string    `json:"name,omitempty"`
	Elem  *TypeRef  `json:"elem,omitempty"`
	Value *TypeRef  `json:"value,omitempty"`
	Union []TypeRef `json:"union,omitempty"`
}

func DescribeTypedRPCRouter(r *TypedRPCRouter) (*TypedRouterDescription, error) {
	if r == nil {
		return nil, fmt.Errorf("router is nil")
	}

	defs := make(map[string]TypedTypeObject)
	seen := make(map[reflect.Type]bool)

	methods := make([]TypedMethodDescription, 0, len(r.procs))
	for _, p := range r.procs {
		params := describeType(p.paramType, defs, seen)
		result := describeType(p.resultType, defs, seen)
		methods = append(methods, TypedMethodDescription{
			Name:               p.name,
			RequirePermissions: append([]string(nil), p.requirePermissions...),
			Params:             params,
			Result:             result,
		})
	}

	sort.Slice(methods, func(i, j int) bool { return methods[i].Name < methods[j].Name })

	return &TypedRouterDescription{
		Methods: methods,
		Types:   defs,
	}, nil
}

func describeType(t reflect.Type, defs map[string]TypedTypeObject, seen map[reflect.Type]bool) TypeRef {
	if t == nil {
		return TypeRef{Kind: "unknown"}
	}

	for t.Kind() == reflect.Pointer {
		elem := t.Elem()
		ref := describeType(elem, defs, seen)
		return TypeRef{
			Kind: "union",
			Union: []TypeRef{
				ref,
				{Kind: "null"},
			},
		}
	}

	if isTime(t) || isUUID(t) {
		return TypeRef{Kind: "string"}
	}

	switch t.Kind() {
	case reflect.Invalid,
		reflect.Uintptr,
		reflect.Complex64, reflect.Complex128,
		reflect.Chan, reflect.Func, reflect.Interface,
		reflect.Pointer, reflect.UnsafePointer:
		return TypeRef{Kind: "unknown"}
	case reflect.String:
		return TypeRef{Kind: "string"}
	case reflect.Bool:
		return TypeRef{Kind: "boolean"}
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return TypeRef{Kind: "number"}
	case reflect.Slice, reflect.Array:
		elem := describeType(t.Elem(), defs, seen)
		return TypeRef{Kind: "array", Elem: &elem}
	case reflect.Map:
		if t.Key().Kind() != reflect.String {
			return TypeRef{Kind: "unknown"}
		}
		value := describeType(t.Elem(), defs, seen)
		return TypeRef{Kind: "record", Value: &value}
	case reflect.Struct:
		if t.Name() == "" {
			return TypeRef{Kind: "unknown"}
		}

		name := tsTypeName(t)
		if !seen[t] {
			seen[t] = true
			defs[name] = TypedTypeObject{
				Fields: describeStructFields(t, defs, seen),
			}
		}

		return TypeRef{Kind: "named", Name: name}
	default:
		return TypeRef{Kind: "unknown"}
	}
}

func describeStructFields(t reflect.Type, defs map[string]TypedTypeObject, seen map[reflect.Type]bool) []TypedField {
	fields := make([]TypedField, 0, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if f.Anonymous {
			continue
		}
		if f.PkgPath != "" {
			continue
		}

		jsonName, optional, skip := parseJSONTag(f.Tag.Get("json"), f.Name)
		if skip {
			continue
		}

		ref := describeType(f.Type, defs, seen)
		fields = append(fields, TypedField{
			Name:     jsonName,
			Optional: optional,
			Type:     ref,
		})
	}

	return fields
}

func parseJSONTag(tag string, fallback string) (string, bool, bool) {
	if tag == "-" {
		return "", false, true
	}
	if tag == "" {
		return lowerFirst(fallback), false, false
	}

	parts := strings.Split(tag, ",")
	var name string
	if parts[0] == "" {
		name = lowerFirst(fallback)
	} else {
		name = parts[0]
	}
	var optional bool
	for _, p := range parts[1:] {
		if p == "omitempty" {
			optional = true
		}
	}
	return name, optional, false
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}

func tsTypeName(t reflect.Type) string {
	if strings.HasSuffix(t.PkgPath(), "/rpc") {
		return t.Name()
	}
	pkgLast := pathLastSegment(t.PkgPath())
	if pkgLast == "" {
		return t.Name()
	}
	return exportName(pkgLast) + t.Name()
}

func exportName(s string) string {
	if s == "" {
		return s
	}
	parts := strings.FieldsFunc(s, func(r rune) bool { return r == '-' || r == '_' || r == '/' })
	var b strings.Builder
	for _, p := range parts {
		if p == "" {
			continue
		}
		b.WriteString(strings.ToUpper(p[:1]))
		if len(p) > 1 {
			b.WriteString(p[1:])
		}
	}
	return b.String()
}

func pathLastSegment(pkgPath string) string {
	pkgPath = strings.TrimSuffix(pkgPath, "/")
	if pkgPath == "" {
		return ""
	}
	i := strings.LastIndex(pkgPath, "/")
	if i == -1 {
		return pkgPath
	}
	return pkgPath[i+1:]
}

func isTime(t reflect.Type) bool {
	return t.PkgPath() == "time" && t.Name() == "Time"
}

func isUUID(t reflect.Type) bool {
	return t.PkgPath() == "github.com/google/uuid" && t.Name() == "UUID"
}
