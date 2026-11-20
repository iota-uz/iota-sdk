package expr

import (
	"bytes"
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// TemplateProgram is a checked `{{ }}` template.
type TemplateProgram struct {
	parts []templatePart
}

type templatePart struct {
	text    string
	program *Program
}

// TemplateExpressions lists the `{{ }}` expressions of a template (trimmed) with their offsets in text.
// An expression ends at the first `}}` outside its string literals and braces, so `{{ {"a": {"b": 1}} }}`
// is one expression; a `{{` without its end is text.
func TemplateExpressions(text string) []TemplateExpression {
	var out []TemplateExpression
	for at := 0; ; {
		start := strings.Index(text[at:], "{{")
		if start < 0 {
			return out
		}
		start += at
		end := expressionEnd(text, start+2)
		if end < 0 {
			return out
		}
		raw := text[start+2 : end]
		lead := len(raw) - len(strings.TrimLeftFunc(raw, unicode.IsSpace)) // as TrimSpace trims Source
		out = append(out, TemplateExpression{Source: strings.TrimSpace(raw), Offset: start + 2 + lead, Start: start, End: end + 2})
		at = end + 2
	}
}

// expressionEnd is the index of the `}}` that closes an expression starting at from, or -1. It follows the
// CEL lexer: quoted, raw (r"...") and triple-quoted strings, and // comments to the end of the line.
func expressionEnd(text string, from int) int {
	depth := 0
	for i := from; i < len(text); i++ {
		c := text[i]
		switch {
		case c == '/' && i+1 < len(text) && text[i+1] == '/':
			nl := strings.IndexByte(text[i:], '\n')
			if nl < 0 {
				return -1
			}
			i += nl
		case c == '"' || c == '\'':
			raw := i > from && (text[i-1] == 'r' || text[i-1] == 'R') && (i-1 == from || !isIdentByte(text[i-2]))
			end := stringEnd(text, i, raw)
			if end < 0 {
				return -1
			}
			i = end
		case c == '{' || c == '[' || c == '(':
			depth++
		case c == ']' || c == ')':
			depth--
		case c == '}':
			if depth <= 0 && i+1 < len(text) && text[i+1] == '}' {
				return i
			}
			depth--
		}
	}
	return -1
}

func isIdentByte(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// stringEnd is the index of the last quote of the string literal opening at start, or -1.
func stringEnd(text string, start int, raw bool) int {
	q := text[start]
	delim := string(q)
	if strings.HasPrefix(text[start:], strings.Repeat(string(q), 3)) {
		delim = strings.Repeat(string(q), 3)
	}
	for i := start + len(delim); i < len(text); i++ {
		if !raw && text[i] == '\\' {
			i++
			continue
		}
		if len(delim) == 1 && text[i] == '\n' {
			return -1 // a one-quote string ends on its line
		}
		if strings.HasPrefix(text[i:], delim) {
			return i + len(delim) - 1
		}
	}
	return -1
}

// TemplateExpression is one `{{ }}` part of a template.
type TemplateExpression struct {
	Source     string
	Offset     int // where Source starts in the template
	Start, End int // the whole `{{ ... }}`
}

// Template checks every `{{ }}` expression of text (any type); unbalanced braces are text. Issue positions
// are positions in text.
func Template(e *Env, text string) (*TemplateProgram, []Issue) {
	t := &TemplateProgram{}
	var issues []Issue
	at := 0
	for _, x := range TemplateExpressions(text) {
		if x.Start > at {
			t.parts = append(t.parts, templatePart{text: text[at:x.Start]})
		}
		at = x.End
		if x.Source == "" {
			line, col := position(text, x.Offset)
			issues = append(issues, Issue{Code: IssueSyntax, Message: "an empty {{ }}", Line: line, Column: col})
			continue
		}
		p, iss := e.Check(x.Source, TypeAny)
		for _, i := range iss {
			i.Line, i.Column = shift(text, x.Offset, i.Line, i.Column)
			issues = append(issues, i)
		}
		if p != nil {
			t.parts = append(t.parts, templatePart{program: p})
		}
	}
	if at < len(text) {
		t.parts = append(t.parts, templatePart{text: text[at:]})
	}
	if len(issues) > 0 {
		return nil, issues
	}
	return t, nil
}

// position is the 1-based line and column of a byte offset.
func position(text string, offset int) (int, int) {
	line, col := 1, 1
	for i, r := range text {
		if i >= offset {
			break
		}
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}

// shift moves a position inside an expression starting at offset to a position in text.
func shift(text string, offset, line, col int) (int, int) {
	l0, c0 := position(text, offset)
	if line <= 0 {
		return l0, c0
	}
	if line == 1 {
		return l0, c0 + col - 1
	}
	return l0 + line - 1, col
}

// Render renders the template: null is "", a list its elements joined with ", ", an object JSON (declared
// properties in declaration order, map and dyn keys sorted), a double as JavaScript writes it, a timestamp
// RFC 3339 in UTC with milliseconds.
func (t *TemplateProgram) Render(vars map[string]any) (string, error) {
	var b strings.Builder
	for _, p := range t.parts {
		if p.program == nil {
			b.WriteString(p.text)
			continue
		}
		v, err := p.program.Eval(vars)
		if err != nil {
			return "", err
		}
		s, err := renderText(v, p.program.env.provider.fieldOf(p.program.out))
		if err != nil {
			return "", &EvalError{Code: CodeExpressionFailed, Message: err.Error()}
		}
		b.WriteString(s)
	}
	return b.String(), nil
}

// RenderValue renders one value as a template part, with f its declaration (DynField when unknown).
func RenderValue(v any, f Field) (string, error) { return renderText(v, f) }

func renderText(v any, f Field) (string, error) {
	f = f.Unwrap()
	switch x := v.(type) {
	case nil:
		return "", nil
	case []any:
		elem := DynField
		if f.Kind == List {
			elem = *f.Elem
		}
		parts := make([]string, 0, len(x))
		for _, e := range x {
			var s string
			var err error
			switch e.(type) {
			case []any, map[string]any:
				// A list or an object inside a list is JSON: flattening it would lose its shape.
				var b bytes.Buffer
				err = writeJSON(&b, e, elem)
				s = b.String()
			default:
				s, err = renderText(e, elem)
			}
			if err != nil {
				return "", err
			}
			parts = append(parts, s)
		}
		return strings.Join(parts, ", "), nil
	case map[string]any:
		var b bytes.Buffer
		if err := writeJSON(&b, x, f); err != nil {
			return "", err
		}
		return b.String(), nil
	}
	return scalarText(v)
}

func scalarText(v any) (string, error) {
	switch x := v.(type) {
	case string:
		return x, nil
	case bool:
		return strconv.FormatBool(x), nil
	case int64:
		return strconv.FormatInt(x, 10), nil
	case uint64:
		return strconv.FormatUint(x, 10), nil
	case float64:
		return FormatDouble(x), nil
	case time.Time:
		return x.UTC().Format("2006-01-02T15:04:05.000Z07:00"), nil
	case time.Duration:
		return strconv.FormatFloat(x.Seconds(), 'f', -1, 64) + "s", nil
	}
	return fmt.Sprint(v), nil
}

// writeJSON writes a value as JSON: declared objects in declaration order, other maps by sorted keys,
// doubles as JavaScript writes them.
func writeJSON(b *bytes.Buffer, v any, f Field) error {
	f = f.Unwrap()
	switch x := v.(type) {
	case nil:
		b.WriteString("null")
	case string:
		enc := json.NewEncoder(b)
		enc.SetEscapeHTML(false)
		if err := enc.Encode(x); err != nil {
			return err
		}
		b.Truncate(b.Len() - 1) // the encoder's newline
	case bool, int64, uint64:
		s, _ := scalarText(x)
		b.WriteString(s)
	case float64:
		s := FormatDouble(x)
		if s == "NaN" || strings.HasSuffix(s, "Infinity") {
			s = "null"
		}
		b.WriteString(s)
	case time.Time, time.Duration:
		s, _ := scalarText(x)
		return writeJSON(b, s, StringField)
	case []any:
		elem := DynField
		if f.Kind == List {
			elem = *f.Elem
		}
		b.WriteByte('[')
		for i, e := range x {
			if i > 0 {
				b.WriteByte(',')
			}
			if err := writeJSON(b, e, elem); err != nil {
				return err
			}
		}
		b.WriteByte(']')
	case map[string]any:
		var keys []string
		props := map[string]Field{}
		if f.Kind == Object {
			for _, p := range f.Props {
				if _, ok := x[p.Name]; ok {
					keys = append(keys, p.Name)
					props[p.Name] = p.Field
				}
			}
		} else {
			for k := range x {
				keys = append(keys, k)
			}
			sort.Strings(keys)
		}
		b.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				b.WriteByte(',')
			}
			_ = writeJSON(b, k, StringField)
			b.WriteByte(':')
			pf, ok := props[k]
			if !ok {
				pf = DynField
				if f.Kind == Map {
					pf = *f.Elem
				}
			}
			if err := writeJSON(b, x[k], pf); err != nil {
				return err
			}
		}
		b.WriteByte('}')
	default:
		raw, err := json.Marshal(x)
		if err != nil {
			return err
		}
		b.Write(raw)
	}
	return nil
}
