package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/pkg/extruntime/manifest"
)

// userRefEntity is the entity the {"$":"user"} placeholder refers to.
const userRefEntity = "core.user"

func isNullRaw(raw json.RawMessage) bool {
	t := bytes.TrimSpace(raw)
	return len(t) == 0 || bytes.Equal(t, []byte("null"))
}

// encodeJSON renders v without HTML escaping. Callers pass strings, numbers,
// booleans and manifest.RefValue, none of which can fail to encode; should one
// ever fail, the result is JSON null instead of a half-written value.
func encodeJSON(v any) json.RawMessage {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return json.RawMessage("null")
	}
	return bytes.TrimRight(buf.Bytes(), "\n")
}

// canonJSON re-encodes raw with sorted keys and verbatim numbers, the form
// jsonb values are compared in.
func canonJSON(raw []byte) (json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, err
	}
	return encodeJSON(v), nil
}

// validateWire checks raw against t. Null is judged by the caller.
func validateWire(t manifest.Type, raw json.RawMessage) error {
	t = withoutNull(t)
	if err := manifest.ValidateValue(t, raw); err != nil {
		return err
	}
	if t.Kind == manifest.KindDecimal && t.Precision > 0 && t.Scale == 0 {
		var s string
		_ = json.Unmarshal(raw, &s)
		if strings.Contains(s, ".") {
			return &manifest.PathError{Message: "fractional digits not allowed (scale 0)"}
		}
	}
	return nil
}

func normalizeDecimal(s string, scale int) string {
	neg := strings.HasPrefix(s, "-")
	s = strings.TrimPrefix(s, "-")
	intPart, frac, _ := strings.Cut(s, ".")
	if scale > 0 && len(frac) < scale {
		frac += strings.Repeat("0", scale-len(frac))
	}
	out := intPart
	if frac != "" {
		out += "." + frac
	}
	if neg && strings.Trim(out, "0.") != "" {
		out = "-" + out
	}
	return out
}

func normalizeDateTime(s string) (string, error) {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return "", err
	}
	return formatDateTime(t), nil
}

func formatDateTime(t time.Time) string {
	return t.Round(time.Microsecond).UTC().Format(time.RFC3339Nano)
}

// canonicalValue returns the one wire form PostgreSQL will give back for a
// valid value, so written and stored values compare equal.
func canonicalValue(t manifest.Type, raw json.RawMessage) (json.RawMessage, error) {
	if isNullRaw(raw) {
		return json.RawMessage("null"), nil
	}
	switch t.Kind {
	case manifest.KindString, manifest.KindEnum, manifest.KindDate:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return encodeJSON(s), nil
	case manifest.KindInt:
		var n json.Number
		dec := json.NewDecoder(bytes.NewReader(raw))
		dec.UseNumber()
		if err := dec.Decode(&n); err != nil {
			return nil, err
		}
		v, err := strconv.ParseInt(n.String(), 10, 64)
		if err != nil {
			return nil, err
		}
		return json.RawMessage(strconv.FormatInt(v, 10)), nil
	case manifest.KindInt64:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		v, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return nil, err
		}
		return encodeJSON(strconv.FormatInt(v, 10)), nil
	case manifest.KindDecimal:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		return encodeJSON(normalizeDecimal(s, t.Scale)), nil
	case manifest.KindBool:
		var b bool
		if err := json.Unmarshal(raw, &b); err != nil {
			return nil, err
		}
		return encodeJSON(b), nil
	case manifest.KindDateTime:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		n, err := normalizeDateTime(s)
		if err != nil {
			return nil, err
		}
		return encodeJSON(n), nil
	case manifest.KindUUID:
		var s string
		if err := json.Unmarshal(raw, &s); err != nil {
			return nil, err
		}
		u, err := uuid.Parse(s)
		if err != nil {
			return nil, err
		}
		return encodeJSON(u.String()), nil
	case manifest.KindJSON:
		return canonJSON(raw)
	case manifest.KindRef:
		var r manifest.RefValue
		if err := json.Unmarshal(raw, &r); err != nil {
			return nil, err
		}
		return encodeJSON(r), nil
	case manifest.KindObject, manifest.KindArray:
		// Composite values have no canonical scalar form; reported below.
	}
	return nil, fmt.Errorf("type %q has no canonical form", t.Kind)
}

// bindValue validates raw and returns the placeholder expression for it,
// binding the argument in a. For refs keyType decides the key encoding.
func bindValue(t manifest.Type, keyType KeyType, raw json.RawMessage, a *Args) (string, error) {
	if err := validateWire(t, raw); err != nil {
		return "", err
	}
	canon, err := canonicalValue(t, raw)
	if err != nil {
		return "", err
	}
	switch t.Kind {
	case manifest.KindString, manifest.KindEnum:
		var s string
		_ = json.Unmarshal(canon, &s)
		return a.Add(s) + "::text", nil
	case manifest.KindInt:
		v, _ := strconv.ParseInt(string(canon), 10, 64)
		return a.Add(v) + "::bigint", nil
	case manifest.KindInt64:
		var s string
		_ = json.Unmarshal(canon, &s)
		v, _ := strconv.ParseInt(s, 10, 64)
		return a.Add(v) + "::bigint", nil
	case manifest.KindBool:
		var b bool
		_ = json.Unmarshal(canon, &b)
		return a.Add(b) + "::boolean", nil
	case manifest.KindDecimal, manifest.KindDate, manifest.KindDateTime, manifest.KindUUID:
		var s string
		_ = json.Unmarshal(canon, &s)
		cast := map[manifest.TypeKind]string{
			manifest.KindDecimal: "numeric", manifest.KindDate: "date",
			manifest.KindDateTime: "timestamptz", manifest.KindUUID: "uuid",
		}[t.Kind]
		return a.Add(s) + "::text::" + cast, nil
	case manifest.KindJSON:
		return a.Add(string(canon)) + "::text::jsonb", nil
	case manifest.KindRef:
		var r manifest.RefValue
		_ = json.Unmarshal(raw, &r)
		key, err := keyType.Canonical(r.Key)
		if err != nil {
			return "", err
		}
		return keyType.bind(key, a), nil
	case manifest.KindObject, manifest.KindArray:
		// Composite values are never bound as a single parameter; reported below.
	}
	return "", fmt.Errorf("type %q cannot be bound", t.Kind)
}

// refKey extracts the canonical key of a ref wire value.
func refKey(raw json.RawMessage, keyType KeyType) (string, error) {
	var r manifest.RefValue
	if err := json.Unmarshal(raw, &r); err != nil {
		return "", err
	}
	return keyType.Canonical(r.Key)
}

type readKind int

const (
	readText readKind = iota
	readBool
	readTime
)

// reader describes how one column is selected and turned into wire form.
type reader struct {
	expr      string
	kind      readKind
	typ       manifest.Type
	refEntity string
}

func newReader(alias, column string, t manifest.Type) reader {
	col := alias + "." + quoteIdent(column)
	r := reader{typ: t, refEntity: t.Entity}
	switch t.Kind {
	case manifest.KindBool:
		r.kind, r.expr = readBool, col
	case manifest.KindDateTime:
		r.kind, r.expr = readTime, col
	case manifest.KindString, manifest.KindInt, manifest.KindInt64, manifest.KindDecimal,
		manifest.KindDate, manifest.KindUUID, manifest.KindEnum, manifest.KindJSON,
		manifest.KindRef, manifest.KindObject, manifest.KindArray:
		fallthrough
	default:
		r.kind, r.expr = readText, col+"::text"
	}
	return r
}

func (r reader) dest() any {
	switch r.kind {
	case readBool:
		return new(*bool)
	case readTime:
		return new(*time.Time)
	case readText:
		fallthrough
	default:
		return new(*string)
	}
}

// wire converts a scanned destination. present=false means the key is omitted
// (NULL in a non-nullable optional field).
func (r reader) wire(dest any) (json.RawMessage, bool, error) {
	switch d := dest.(type) {
	case **bool:
		if *d == nil {
			return r.null()
		}
		return encodeJSON(**d), true, nil
	case **time.Time:
		if *d == nil {
			return r.null()
		}
		return encodeJSON(formatDateTime(**d)), true, nil
	case **string:
		if *d == nil {
			return r.null()
		}
		s := **d
		switch r.typ.Kind {
		case manifest.KindInt:
			return json.RawMessage(s), true, nil
		case manifest.KindJSON:
			c, err := canonJSON([]byte(s))
			return c, true, err
		case manifest.KindRef:
			return encodeJSON(manifest.RefValue{Entity: r.refEntity, Key: s}), true, nil
		case manifest.KindString, manifest.KindInt64, manifest.KindDecimal, manifest.KindBool,
			manifest.KindDate, manifest.KindDateTime, manifest.KindUUID, manifest.KindEnum,
			manifest.KindObject, manifest.KindArray:
			return encodeJSON(s), true, nil
		default:
			return encodeJSON(s), true, nil
		}
	}
	return nil, false, fmt.Errorf("unexpected scan destination %T", dest)
}

func (r reader) null() (json.RawMessage, bool, error) {
	if r.typ.Nullable {
		return json.RawMessage("null"), true, nil
	}
	return nil, false, nil
}
