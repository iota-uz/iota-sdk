package graph

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync/atomic"

	"github.com/santhosh-tekuri/jsonschema/v6"
	"github.com/santhosh-tekuri/jsonschema/v6/kind"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
)

// The keywords the format adds to JSON Schema:
//   - x-unique: true — the items of an array are unique; a string — the items' values of that property are unique;
//   - x-unique-ci: as x-unique, compared without case;
//   - x-max-ms: a duration (`30s`, `20m`, `2h`, `7d`, or milliseconds as an integer) of at most this many ms.
const vocabURL = "https://iota.uz/automation/graph/vocab"

var printer = message.NewPrinter(language.English)

var schemaSeq atomic.Int64

func automationVocab() *jsonschema.Vocabulary {
	return &jsonschema.Vocabulary{URL: vocabURL, Compile: compileVocab}
}

type vocabExt struct {
	unique   any // true or a property name
	uniqueCI any
	maxMs    int64
}

func compileVocab(_ *jsonschema.CompilerContext, obj map[string]any) (jsonschema.SchemaExt, error) {
	ext := &vocabExt{}
	found := false
	if v, ok := obj["x-unique"]; ok {
		if err := uniqueArg(v); err != nil {
			return nil, fmt.Errorf("x-unique: %w", err)
		}
		ext.unique, found = v, true
	}
	if v, ok := obj["x-unique-ci"]; ok {
		if err := uniqueArg(v); err != nil {
			return nil, fmt.Errorf("x-unique-ci: %w", err)
		}
		ext.uniqueCI, found = v, true
	}
	if v, ok := obj["x-max-ms"]; ok {
		n, ok := v.(json.Number)
		if !ok {
			return nil, fmt.Errorf("x-max-ms is a number of milliseconds")
		}
		ms, err := n.Int64()
		if err != nil || ms <= 0 {
			return nil, fmt.Errorf("x-max-ms is a positive integer")
		}
		ext.maxMs, found = ms, true
	}
	if !found {
		return nil, nil //nolint:nilnil // the vocabulary API: no keywords of ours, no extension
	}
	return ext, nil
}

func uniqueArg(v any) error {
	switch x := v.(type) {
	case bool:
		if !x {
			return fmt.Errorf("true or a property name")
		}
		return nil
	case string:
		if x == "" {
			return fmt.Errorf("true or a property name")
		}
		return nil
	}
	return fmt.Errorf("true or a property name")
}

// uniqueKind is an item whose value (or property value) repeats an earlier item's.
type uniqueKind struct {
	Index int
	Field string
	Value string
}

func (*uniqueKind) KeywordPath() []string { return []string{"x-unique"} }
func (k *uniqueKind) LocalizedString(*message.Printer) string {
	return fmt.Sprintf("%s is listed twice", k.Value)
}

// durationKind is a duration that is not one or exceeds the maximum.
type durationKind struct{ Message string }

func (*durationKind) KeywordPath() []string                     { return []string{"x-max-ms"} }
func (k *durationKind) LocalizedString(*message.Printer) string { return k.Message }

func (x *vocabExt) Validate(ctx *jsonschema.ValidatorContext, v any) {
	if x.unique != nil {
		checkUnique(ctx, v, x.unique, false)
	}
	if x.uniqueCI != nil {
		checkUnique(ctx, v, x.uniqueCI, true)
	}
	if x.maxMs > 0 {
		ms, ok := DurationMs(v)
		switch {
		case !ok:
			ctx.AddError(&durationKind{Message: "a duration like 30s, 20m, 2h or 7d"})
		case ms > x.maxMs:
			ctx.AddError(&durationKind{Message: fmt.Sprintf("at most %s", formatMs(x.maxMs))})
		}
	}
}

func checkUnique(ctx *jsonschema.ValidatorContext, v any, arg any, fold bool) {
	arr, ok := v.([]any)
	if !ok {
		return
	}
	field, _ := arg.(string)
	seen := map[string]bool{}
	for i, item := range arr {
		val := item
		if field != "" {
			obj, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if val, ok = obj[field]; !ok {
				continue
			}
		}
		k, err := canonicalJSON(val)
		if err != nil {
			continue
		}
		if fold {
			k = strings.ToLower(k)
		}
		if seen[k] {
			shown, err := json.Marshal(val)
			if err != nil {
				shown = []byte(k)
			}
			ctx.AddError(&uniqueKind{Index: i, Field: field, Value: string(shown)})
			continue
		}
		seen[k] = true
	}
}

var durationRe = regexp.MustCompile(`^([1-9][0-9]{0,5})(s|m|h|d)$`)

// canonicalJSON writes a JSON value so equal values give equal text: numbers by their exact value (1 and
// 1.0 are the same), object keys sorted.
func canonicalJSON(v any) (string, error) {
	switch x := v.(type) {
	case json.Number:
		r, ok := new(big.Rat).SetString(string(x))
		if !ok {
			return "", fmt.Errorf("%s is not a number", x)
		}
		return "n:" + r.RatString(), nil
	case float64:
		r := new(big.Rat)
		if r.SetFloat64(x) == nil {
			return "", fmt.Errorf("%v is not a number", x)
		}
		return "n:" + r.RatString(), nil
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			p, err := canonicalJSON(e)
			if err != nil {
				return "", err
			}
			parts[i] = p
		}
		return "[" + strings.Join(parts, ",") + "]", nil
	case map[string]any:
		keys := sortedKeys(x)
		parts := make([]string, len(keys))
		for i, k := range keys {
			p, err := canonicalJSON(x[k])
			if err != nil {
				return "", err
			}
			key, err := json.Marshal(k)
			if err != nil {
				return "", err
			}
			parts[i] = string(key) + ":" + p
		}
		return "{" + strings.Join(parts, ",") + "}", nil
	}
	b, err := json.Marshal(v)
	return string(b), err
}

// DurationMs reads a duration param: `30s`, `20m`, `2h`, `7d` or an integer of milliseconds.
func DurationMs(v any) (int64, bool) {
	switch x := v.(type) {
	case string:
		m := durationRe.FindStringSubmatch(x)
		if m == nil {
			return 0, false
		}
		n, _ := strconv.ParseInt(m[1], 10, 64)
		unit := map[string]int64{"s": 1000, "m": 60_000, "h": 3_600_000, "d": 86_400_000}[m[2]]
		return n * unit, true
	case json.Number:
		// 1000 and 1e3 are the same integer; 1.5 is not one.
		r, ok := new(big.Rat).SetString(string(x))
		if !ok || !r.IsInt() || !r.Num().IsInt64() {
			return 0, false
		}
		n := r.Num().Int64()
		return n, n > 0
	case int:
		return int64(x), x > 0
	case int64:
		return x, x > 0
	case float64:
		return int64(x), x > 0 && x == float64(int64(x))
	}
	return 0, false
}

func formatMs(ms int64) string {
	switch {
	case ms%86_400_000 == 0:
		return fmt.Sprintf("%dd", ms/86_400_000)
	case ms%3_600_000 == 0:
		return fmt.Sprintf("%dh", ms/3_600_000)
	case ms%60_000 == 0:
		return fmt.Sprintf("%dm", ms/60_000)
	}
	return fmt.Sprintf("%dms", ms)
}

// noLoader refuses every external resource; local references and the embedded drafts need none.
type noLoader struct{}

func (noLoader) Load(url string) (any, error) {
	return nil, fmt.Errorf("%s: a schema of the format references no external resource", url)
}

// compileSchema compiles a JSON Schema (draft 2020-12 by default) with the format's keywords.
func compileSchema(raw json.RawMessage) (*jsonschema.Schema, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft2020)
	c.UseLoader(noLoader{}) // the format reads no files and no URLs: only the schema itself and the drafts
	c.RegisterVocabulary(automationVocab())
	c.AssertVocabs()
	url := fmt.Sprintf("mem://automation/%d.json", schemaSeq.Add(1))
	if err := c.AddResource(url, doc); err != nil {
		return nil, err
	}
	return c.Compile(url)
}

// schemaIssue is one leaf of a validation error, with its path in the instance.
type schemaIssue struct {
	Path    []any
	Message string
	Limit   bool // a length or count limit
}

// validateInstance validates v (JSON values with json.Number) and flattens the errors to leaves.
func validateInstance(s *jsonschema.Schema, v any) []schemaIssue {
	err := s.Validate(v)
	if err == nil {
		return nil
	}
	ve, ok := err.(*jsonschema.ValidationError)
	if !ok {
		return []schemaIssue{{Message: err.Error()}}
	}
	var out []schemaIssue
	var walk func(e *jsonschema.ValidationError)
	walk = func(e *jsonschema.ValidationError) {
		if len(e.Causes) > 0 {
			for _, c := range e.Causes {
				walk(c)
			}
			return
		}
		base := instancePath(v, e.InstanceLocation)
		switch k := e.ErrorKind.(type) {
		case *kind.Required:
			for _, m := range k.Missing {
				out = append(out, schemaIssue{Path: append(clone(base), m), Message: "required"})
			}
		case *kind.AdditionalProperties:
			for _, p := range k.Properties {
				out = append(out, schemaIssue{Path: append(clone(base), p), Message: "not allowed here"})
			}
		case *uniqueKind:
			p := append(clone(base), k.Index)
			if k.Field != "" {
				p = append(p, k.Field)
			}
			out = append(out, schemaIssue{Path: p, Message: k.LocalizedString(printer)})
		case *kind.MaxItems, *kind.MaxLength, *kind.MaxProperties:
			out = append(out, schemaIssue{Path: base, Message: e.ErrorKind.LocalizedString(printer), Limit: true})
		default:
			out = append(out, schemaIssue{Path: base, Message: e.ErrorKind.LocalizedString(printer)})
		}
	}
	walk(ve)
	// The validator's order of causes is not stable: sort, so the errors and their cap are deterministic.
	sort.SliceStable(out, func(i, j int) bool {
		a, b := pathOf("", out[i].Path), pathOf("", out[j].Path)
		if a != b {
			return a < b
		}
		return out[i].Message < out[j].Message
	})
	return out
}

func clone(p []any) []any { return append([]any(nil), p...) }

// instancePath turns a location of string tokens into keys and indices by walking the instance.
func instancePath(v any, loc []string) []any {
	out := make([]any, 0, len(loc))
	cur := v
	for _, tok := range loc {
		switch c := cur.(type) {
		case []any:
			i, err := strconv.Atoi(tok)
			if err == nil {
				out = append(out, i)
				if i >= 0 && i < len(c) {
					cur = c[i]
				} else {
					cur = nil
				}
				continue
			}
		case map[string]any:
			out = append(out, tok)
			cur = c[tok]
			continue
		}
		out = append(out, tok)
		cur = nil
	}
	return out
}

// pathOf writes a path as the format names it: `nodes[2].params.expression`.
func pathOf(prefix string, path []any) string {
	var b strings.Builder
	b.WriteString(prefix)
	for _, p := range path {
		switch x := p.(type) {
		case int:
			fmt.Fprintf(&b, "[%d]", x)
		default:
			if b.Len() > 0 {
				b.WriteByte('.')
			}
			fmt.Fprint(&b, x)
		}
	}
	return b.String()
}

// jsonValue re-reads any Go value as JSON values with json.Number, the form schemas validate.
func jsonValue(v any) (any, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	var out any
	dec := json.NewDecoder(bytes.NewReader(b))
	dec.UseNumber()
	if err := dec.Decode(&out); err != nil {
		return nil, err
	}
	return out, nil
}

// ValidateJSON checks a JSON value against a JSON Schema as it is: no defaults are applied and nothing
// is dropped (a form's submitted fields, unlike Parse of params). The first issue is the error.
func ValidateJSON(schema, value json.RawMessage) error {
	s, err := compileSchema(schema)
	if err != nil {
		return fmt.Errorf("schema: %w", err)
	}
	v, err := jsonschema.UnmarshalJSON(bytes.NewReader(value))
	if err != nil {
		return fmt.Errorf("not JSON: %w", err)
	}
	if issues := validateInstance(s, v); len(issues) > 0 {
		return errors.New(issues[0].Message)
	}
	return nil
}
