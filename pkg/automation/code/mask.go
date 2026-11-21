package code

import (
	"bytes"
	"encoding/json"
	"io"
	"net/url"
	"sort"
	"strings"
)

// Mask is what a secret is replaced with.
const Mask = "***"

// Masker replaces secrets with Mask: their exact values, their JSON-escaped and URL-encoded forms.
type Masker interface {
	Mask(s string) string
	// MaskJSON masks every string, key and number of a JSON value and returns valid JSON. A value that is
	// not JSON is masked as text and returned as a JSON string.
	MaskJSON(v json.RawMessage) json.RawMessage
}

type masker struct {
	r *strings.Replacer // nil: nothing to mask
}

// NewMasker returns a Masker for the values of secrets. Empty values are ignored.
func NewMasker(secrets map[string]string) Masker {
	forms := map[string]bool{}
	for _, v := range secrets {
		if v == "" {
			continue
		}
		for _, f := range secretForms(v) {
			if f != "" {
				forms[f] = true
			}
		}
	}
	if len(forms) == 0 {
		return masker{}
	}
	list := make([]string, 0, len(forms))
	for f := range forms {
		list = append(list, f)
	}
	// Longest first, so a form that contains another is replaced whole.
	sort.Slice(list, func(i, j int) bool {
		if len(list[i]) != len(list[j]) {
			return len(list[i]) > len(list[j])
		}
		return list[i] < list[j]
	})
	pairs := make([]string, 0, 2*len(list))
	for _, f := range list {
		pairs = append(pairs, f, Mask)
	}
	return masker{r: strings.NewReplacer(pairs...)}
}

// secretForms are the spellings of a secret that can reach the output: raw, JSON-escaped (as Go and as
// JSON.stringify escape), URL-encoded (query, path and encodeURIComponent).
func secretForms(v string) []string {
	forms := []string{v, url.QueryEscape(v), url.PathEscape(v), encodeURIComponent(v)}
	// JSON-escaped, and escaped twice for output that embeds serialized JSON.
	for _, once := range jsonEscapes(v) {
		forms = append(forms, once)
		forms = append(forms, jsonEscapes(once)...)
	}
	return forms
}

// jsonEscapes are the bodies of v as a JSON string: as JSON.stringify writes it and as Go writes it with
// HTML escaping.
func jsonEscapes(v string) []string {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		return []string{jsJSONEscape(v)}
	}
	html := strings.TrimSuffix(strings.TrimSuffix(strings.TrimPrefix(b.String(), `"`), "\n"), `"`)
	return []string{jsJSONEscape(v), html}
}

// jsJSONEscape is the body of JSON.stringify(v) without the quotes: only ", \ and control characters are
// escaped, and lone surrogates cannot occur in a Go string.
func jsJSONEscape(v string) string {
	var b strings.Builder
	for _, r := range v {
		switch r {
		case '"':
			b.WriteString(`\"`)
		case '\\':
			b.WriteString(`\\`)
		case '\b':
			b.WriteString(`\b`)
		case '\f':
			b.WriteString(`\f`)
		case '\n':
			b.WriteString(`\n`)
		case '\r':
			b.WriteString(`\r`)
		case '\t':
			b.WriteString(`\t`)
		default:
			if r < 0x20 {
				const hex = "0123456789abcdef"
				b.WriteString(`\u00`)
				b.WriteByte(hex[r>>4])
				b.WriteByte(hex[r&15])
			} else {
				b.WriteRune(r)
			}
		}
	}
	return b.String()
}

// encodeURIComponent is JavaScript's encodeURIComponent.
func encodeURIComponent(v string) string {
	const unreserved = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_.!~*'()"
	const hex = "0123456789ABCDEF"
	var b strings.Builder
	for i := 0; i < len(v); i++ {
		c := v[i]
		if strings.IndexByte(unreserved, c) >= 0 {
			b.WriteByte(c)
			continue
		}
		b.WriteByte('%')
		b.WriteByte(hex[c>>4])
		b.WriteByte(hex[c&15])
	}
	return b.String()
}

func (m masker) Mask(s string) string {
	if m.r == nil {
		return s
	}
	return m.r.Replace(s)
}

func (m masker) MaskJSON(v json.RawMessage) json.RawMessage {
	if v == nil {
		return nil
	}
	if m.r == nil {
		return v
	}
	var b bytes.Buffer
	dec := json.NewDecoder(bytes.NewReader(v))
	dec.UseNumber()
	if err := m.maskValue(dec, &b); err != nil {
		return quoteJSON(m.Mask(string(v)))
	}
	if _, err := dec.Token(); err != io.EOF {
		return quoteJSON(m.Mask(string(v)))
	}
	return b.Bytes()
}

func (m masker) maskValue(dec *json.Decoder, b *bytes.Buffer) error {
	t, err := dec.Token()
	if err != nil {
		return err
	}
	switch v := t.(type) {
	case json.Delim:
		switch v {
		case '{':
			b.WriteByte('{')
			for i := 0; dec.More(); i++ {
				if i > 0 {
					b.WriteByte(',')
				}
				kt, err := dec.Token()
				if err != nil {
					return err
				}
				key, ok := kt.(string)
				if !ok {
					return errNotJSON
				}
				b.Write(quoteJSON(m.Mask(key)))
				b.WriteByte(':')
				if err := m.maskValue(dec, b); err != nil {
					return err
				}
			}
			if _, err := dec.Token(); err != nil {
				return err
			}
			b.WriteByte('}')
		case '[':
			b.WriteByte('[')
			for i := 0; dec.More(); i++ {
				if i > 0 {
					b.WriteByte(',')
				}
				if err := m.maskValue(dec, b); err != nil {
					return err
				}
			}
			if _, err := dec.Token(); err != nil {
				return err
			}
			b.WriteByte(']')
		default:
			return errNotJSON
		}
	case string:
		b.Write(quoteJSON(m.Mask(v)))
	case json.Number:
		if masked := m.Mask(v.String()); masked != v.String() {
			b.Write(quoteJSON(masked))
		} else {
			b.WriteString(v.String())
		}
	case bool:
		if v {
			b.WriteString("true")
		} else {
			b.WriteString("false")
		}
	case nil:
		b.WriteString("null")
	default:
		return errNotJSON
	}
	return nil
}

type jsonError string

func (e jsonError) Error() string { return string(e) }

const errNotJSON = jsonError("not JSON")

// quoteJSON is s as a JSON string, without HTML escaping.
func quoteJSON(s string) json.RawMessage {
	var b bytes.Buffer
	enc := json.NewEncoder(&b)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(s); err != nil {
		return json.RawMessage(`""`)
	}
	return bytes.TrimSuffix(b.Bytes(), []byte("\n"))
}
