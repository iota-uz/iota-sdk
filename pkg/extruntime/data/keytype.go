package data

import (
	"fmt"
	"strconv"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// KeyType is the SQL type of a record key. Own entities always use KeyUUID;
// native entities declare theirs.
type KeyType string

const (
	KeyUUID  KeyType = "uuid"
	KeyInt   KeyType = "int"   // PostgreSQL integer (serial)
	KeyInt64 KeyType = "int64" // PostgreSQL bigint
	KeyText  KeyType = "text"
)

func (k KeyType) valid() bool {
	switch k {
	case KeyUUID, KeyInt, KeyInt64, KeyText:
		return true
	}
	return false
}

// Wire is the manifest name of the key type ("uuid" | "int64" | "string"),
// as published in Resource.KeyType and the developer kit: a serial integer
// travels as int64, text as string.
func (k KeyType) Wire() string {
	switch k {
	case KeyUUID:
		return "uuid"
	case KeyInt, KeyInt64:
		return "int64"
	case KeyText:
		fallthrough
	default:
		return "string"
	}
}

// SQL is the column type.
func (k KeyType) SQL() string {
	switch k {
	case KeyUUID:
		return "uuid"
	case KeyInt:
		return "integer"
	case KeyInt64:
		return "bigint"
	case KeyText:
		fallthrough
	default:
		return "text"
	}
}

// Canonical normalizes a wire key ("042" -> "42", upper-case uuid -> lower).
func (k KeyType) Canonical(s string) (string, error) {
	switch k {
	case KeyUUID:
		u, err := uuid.Parse(s)
		if err != nil {
			return "", fmt.Errorf("key %q is not a uuid", s)
		}
		return u.String(), nil
	case KeyInt:
		n, err := strconv.ParseInt(s, 10, 32)
		if err != nil {
			return "", fmt.Errorf("key %q is not an integer", s)
		}
		return strconv.FormatInt(n, 10), nil
	case KeyInt64:
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return "", fmt.Errorf("key %q is not an int64", s)
		}
		return strconv.FormatInt(n, 10), nil
	case KeyText:
		fallthrough
	default:
		if s == "" {
			return "", fmt.Errorf("key is empty")
		}
		return s, nil
	}
}

// bindExpr returns the placeholder expression for a canonical key and the Go
// argument to bind.
func (k KeyType) bind(canonical string, a *Args) string {
	switch k {
	case KeyInt, KeyInt64:
		n, _ := strconv.ParseInt(canonical, 10, 64)
		return a.Add(n) + "::" + k.SQL()
	case KeyUUID:
		return a.Add(canonical) + "::text::uuid"
	case KeyText:
		fallthrough
	default:
		return a.Add(canonical) + "::text"
	}
}

// wireType is the manifest type used when a key is filtered or sorted.
func (k KeyType) wireType() manifest.Type {
	switch k {
	case KeyUUID:
		return manifest.Type{Kind: manifest.KindUUID}
	case KeyInt, KeyInt64:
		return manifest.Type{Kind: manifest.KindInt64}
	case KeyText:
		fallthrough
	default:
		return manifest.Type{Kind: manifest.KindString}
	}
}

// Args collects positional SQL arguments. Fragments returned by this package
// use "$n" placeholders numbered from the builder, so several fragments can
// be combined into one statement.
type Args struct {
	vals []any
}

// NewArgs starts numbering after the first n already-used placeholders.
func NewArgs(used int) *Args {
	return &Args{vals: make([]any, used)}
}

// Add appends v and returns its placeholder.
func (a *Args) Add(v any) string {
	a.vals = append(a.vals, v)
	return "$" + strconv.Itoa(len(a.vals))
}

// Values returns the arguments in placeholder order (including the n unused
// leading slots, nil, of NewArgs).
func (a *Args) Values() []any { return a.vals }
