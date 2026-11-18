package data

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// record is one row in wire form, keyed by physical column so it can be
// rendered through any version's field keys.
type record struct {
	key        string
	created    time.Time
	updated    time.Time
	rowVersion int64
	vals       map[string]json.RawMessage
}

// projection is the set of fields selected and rendered for a caller.
type projection struct {
	m       *model
	alias   string
	fields  []*fieldModel
	readers []reader
}

func (m *model) project(alias string, visible func(*fieldModel) bool) *projection {
	p := &projection{m: m, alias: alias}
	for _, f := range m.Fields {
		if visible != nil && !visible(f) {
			continue
		}
		p.fields = append(p.fields, f)
		p.readers = append(p.readers, newReader(alias, f.Column, f.Type))
	}
	return p
}

func (p *projection) selectList() string {
	cols := []string{p.alias + "." + quoteIdent(p.m.KeyCol) + "::text"}
	if p.m.hasMeta {
		cols = append(cols, p.alias+".created_at", p.alias+".updated_at", p.alias+".row_version::text")
	}
	for _, r := range p.readers {
		cols = append(cols, r.expr)
	}
	return strings.Join(cols, ", ")
}

func (p *projection) scan(rows pgx.Rows) (*record, error) {
	rec := &record{vals: make(map[string]json.RawMessage, len(p.fields))}
	var rv string
	dests := []any{&rec.key}
	if p.m.hasMeta {
		dests = append(dests, &rec.created, &rec.updated, &rv)
	}
	fd := make([]any, len(p.readers))
	for i, r := range p.readers {
		fd[i] = r.dest()
	}
	dests = append(dests, fd...)
	if err := rows.Scan(dests...); err != nil {
		return nil, err
	}
	if rv != "" {
		rec.rowVersion, _ = strconv.ParseInt(rv, 10, 64)
	}
	for i, r := range p.readers {
		raw, present, err := r.wire(fd[i])
		if err != nil {
			return nil, err
		}
		if present {
			rec.vals[p.fields[i].Column] = raw
		}
	}
	return rec, nil
}

// collect scans every row and closes rows.
func (p *projection) collect(rows pgx.Rows) ([]*record, error) {
	defer rows.Close()
	var out []*record
	for rows.Next() {
		r, err := p.scan(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// marshal renders a record through the projection's field keys.
func (p *projection) marshal(r *record) json.RawMessage {
	var b bytes.Buffer
	b.WriteString(`{"key":`)
	b.Write(encodeJSON(r.key))
	for _, f := range p.fields {
		v, ok := r.vals[f.Column]
		if !ok {
			continue
		}
		b.WriteByte(',')
		b.Write(encodeJSON(f.Key))
		b.WriteByte(':')
		b.Write(v)
	}
	if p.m.hasMeta && !r.created.IsZero() {
		fmt.Fprintf(&b, `,"createdAt":%s,"updatedAt":%s,"rowVersion":%d`,
			encodeJSON(formatDateTime(r.created)), encodeJSON(formatDateTime(r.updated)), r.rowVersion)
	}
	b.WriteByte('}')
	return b.Bytes()
}

func marshalList(p *projection, recs []*record) json.RawMessage {
	var b bytes.Buffer
	b.WriteByte('[')
	for i, r := range recs {
		if i > 0 {
			b.WriteByte(',')
		}
		b.Write(p.marshal(r))
	}
	b.WriteByte(']')
	return b.Bytes()
}

func (r *record) value(f *fieldModel) string {
	if v, ok := r.vals[f.Column]; ok {
		return string(v)
	}
	return "null"
}

// recordValue is the stored form of a canonical wire value: NULL in a field
// that cannot be JSON null is omitted.
func recordValue(f *fieldModel, canon json.RawMessage) (json.RawMessage, bool) {
	if isNullRaw(canon) {
		if f.Type.Nullable {
			return json.RawMessage("null"), true
		}
		return nil, false
	}
	return canon, true
}
