package data

import (
	"crypto/sha1" //nolint:gosec // identifier shortening, not security
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

const maxIdentLen = 63

var (
	namespaceRe = regexp.MustCompile(`^[a-z][a-z0-9]*(\.[a-z][a-z0-9]*)*$`)
	nameRe      = regexp.MustCompile(`^[a-z][a-zA-Z0-9_]*$`)
	fieldKeyRe  = regexp.MustCompile(`^[A-Za-z][A-Za-z0-9_]*$`)
	identRe     = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)
)

// Record keys that are part of every record's wire form and therefore cannot
// be declared as field keys.
var reservedKeys = map[string]bool{
	"key": true, "rowVersion": true, "createdAt": true, "updatedAt": true, "baseKey": true,
}

// Physical columns every table owns.
var reservedColumns = map[string]bool{
	"tenant_id": true, "id": true, "created_at": true, "updated_at": true, "row_version": true, "base_key": true,
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}

func qualified(schema, table string) string {
	if schema == "" {
		return quoteIdent(table)
	}
	return quoteIdent(schema) + "." + quoteIdent(table)
}

// quoteLiteral renders a SQL string literal that is safe regardless of the
// standard_conforming_strings setting.
func quoteLiteral(s string) string {
	s = strings.ReplaceAll(s, "\x00", "")
	if strings.Contains(s, `\`) {
		return ` E'` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `'`, `''`) + `'`
	}
	return `'` + strings.ReplaceAll(s, `'`, `''`) + `'`
}

// snake converts camelCase to snake_case ("expiresOn" -> "expires_on").
func snake(s string) string {
	rs := []rune(s)
	var b strings.Builder
	for i, r := range rs {
		if unicode.IsUpper(r) {
			if i > 0 {
				prev := rs[i-1]
				nextLower := i+1 < len(rs) && unicode.IsLower(rs[i+1])
				if unicode.IsLower(prev) || unicode.IsDigit(prev) || (unicode.IsUpper(prev) && nextLower) {
					b.WriteByte('_')
				}
			}
			b.WriteRune(unicode.ToLower(r))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func shortHash(parts ...string) string {
	sum := sha1.Sum([]byte(strings.Join(parts, "\x00"))) //nolint:gosec
	return hex.EncodeToString(sum[:])[:8]
}

// shortName joins parts into an identifier of at most 63 bytes.
func shortName(parts ...string) string {
	n := strings.Join(parts, "_")
	if len(n) <= maxIdentLen {
		return n
	}
	return n[:maxIdentLen-9] + "_" + shortHash(parts...)
}

// entityLocation derives the physical schema and table of an entity or
// overlay from its resource ID alone: schema "x_<namespace with _>", table
// snake(<name>). Names never contain dots, so the namespace is everything up
// to the last dot.
func entityLocation(id string) (schema, table string, err error) {
	i := strings.LastIndexByte(id, '.')
	if i <= 0 || i == len(id)-1 {
		return "", "", fmt.Errorf("resource id %q has no namespace", id)
	}
	ns, name := id[:i], id[i+1:]
	if !namespaceRe.MatchString(ns) {
		return "", "", fmt.Errorf("invalid namespace %q in %q", ns, id)
	}
	if !nameRe.MatchString(name) {
		return "", "", fmt.Errorf("invalid resource name %q in %q", name, id)
	}
	schema = "x_" + strings.ReplaceAll(ns, ".", "_")
	table = snake(name)
	if len(schema) > maxIdentLen || len(table) > maxIdentLen {
		return "", "", fmt.Errorf("%q is too long for a PostgreSQL identifier", id)
	}
	return schema, table, nil
}

// assignColumn picks the immutable physical column of a field key at first
// apply: the snake_case key when it is valid and free, otherwise
// "f_<short hash>". A column is never renamed afterwards.
func assignColumn(key string, taken map[string]bool) string {
	c := snake(key)
	if len(c) <= maxIdentLen && identRe.MatchString(c) && !reservedColumns[c] && !taken[c] {
		return c
	}
	base := "f_" + shortHash(key)
	c = base
	for i := 2; taken[c]; i++ {
		c = fmt.Sprintf("%s_%d", base, i)
	}
	return c
}
