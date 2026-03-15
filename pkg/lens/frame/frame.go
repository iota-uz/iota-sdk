package frame

import (
	"fmt"
	"reflect"
	"slices"
	"time"
)

type FieldType string

const (
	FieldTypeString    FieldType = "string"
	FieldTypeNumber    FieldType = "number"
	FieldTypeBoolean   FieldType = "boolean"
	FieldTypeTime      FieldType = "time"
	FieldTypeUnknown   FieldType = "unknown"
	FieldTypeLocalized FieldType = "localized"
)

type FieldRole string

const (
	RoleTime      FieldRole = "time"
	RoleDimension FieldRole = "dimension"
	RoleMetric    FieldRole = "metric"
	RoleSeries    FieldRole = "series"
	RoleID        FieldRole = "id"
	RoleLinkParam FieldRole = "link_param"
)

type Field struct {
	Name   string
	Type   FieldType
	Role   FieldRole
	Labels map[string]string
	// Values are expected to hold scalar/value-like cells (numbers, strings, bools, time values).
	// Nested mutable reference types should be normalized by callers before storing in a frame.
	Values        []any
	FormatterHint string
}

// Clone duplicates the field metadata and cell slice. Nested map/slice cell values are copied
// recursively so cloned frames cannot mutate the original frame state.
func (f Field) Clone() Field {
	labels := make(map[string]string, len(f.Labels))
	for k, v := range f.Labels {
		labels[k] = v
	}
	values := make([]any, len(f.Values))
	for i, value := range f.Values {
		values[i] = cloneValue(value)
	}
	return Field{
		Name:          f.Name,
		Type:          f.Type,
		Role:          f.Role,
		Labels:        labels,
		Values:        values,
		FormatterHint: f.FormatterHint,
	}
}

type FrameMeta struct {
	Title       string
	Description string
	Labels      map[string]string
}

type Frame struct {
	Name       string
	Fields     []Field
	RowCount   int
	Meta       FrameMeta
	fieldIndex map[string]int
}

func New(name string, fields ...Field) (*Frame, error) {
	fr := &Frame{Name: name, Fields: make([]Field, len(fields))}
	for i, field := range fields {
		fr.Fields[i] = field.Clone()
	}
	if err := fr.Normalize(); err != nil {
		return nil, err
	}
	return fr, nil
}

func (f *Frame) Normalize() error {
	rowCount := 0
	for i := range f.Fields {
		if i == 0 {
			rowCount = len(f.Fields[i].Values)
			continue
		}
		if len(f.Fields[i].Values) != rowCount {
			return fmt.Errorf("frame %s field %s has %d values, expected %d", f.Name, f.Fields[i].Name, len(f.Fields[i].Values), rowCount)
		}
	}
	f.RowCount = rowCount
	f.buildFieldIndex()
	return nil
}

func (f *Frame) Clone() *Frame {
	if f == nil {
		return nil
	}
	fields := make([]Field, len(f.Fields))
	for i, field := range f.Fields {
		fields[i] = field.Clone()
	}
	labels := make(map[string]string, len(f.Meta.Labels))
	for k, v := range f.Meta.Labels {
		labels[k] = v
	}
	return &Frame{
		Name:     f.Name,
		Fields:   fields,
		RowCount: f.RowCount,
		Meta: FrameMeta{
			Title:       f.Meta.Title,
			Description: f.Meta.Description,
			Labels:      labels,
		},
	}
}

func (f *Frame) buildFieldIndex() {
	f.fieldIndex = make(map[string]int, len(f.Fields))
	for i := range f.Fields {
		f.fieldIndex[f.Fields[i].Name] = i
	}
}

func (f *Frame) Field(name string) (*Field, bool) {
	i, ok := f.fieldIndex[name]
	if !ok || i >= len(f.Fields) {
		return nil, false
	}
	return &f.Fields[i], true
}

func (f *Frame) MustField(name string) Field {
	field, ok := f.Field(name)
	if !ok {
		panic(fmt.Sprintf("field %q not found in frame %q", name, f.Name))
	}
	return field.Clone()
}

func (f *Frame) Rows() []map[string]any {
	if f == nil || f.RowCount == 0 {
		return nil
	}
	rows := make([]map[string]any, f.RowCount)
	for row := 0; row < f.RowCount; row++ {
		item := make(map[string]any, len(f.Fields))
		for _, field := range f.Fields {
			item[field.Name] = field.Values[row]
		}
		rows[row] = item
	}
	return rows
}

func (f *Frame) AppendRow(row map[string]any) error {
	if len(f.Fields) == 0 {
		names := make([]string, 0, len(row))
		for name := range row {
			names = append(names, name)
		}
		slices.Sort(names)
		for _, name := range names {
			f.Fields = append(f.Fields, Field{
				Name:   name,
				Type:   InferFieldType(row[name]),
				Values: []any{row[name]},
			})
		}
		f.RowCount = 1
		f.buildFieldIndex()
		return nil
	}

	for i := range f.Fields {
		value, ok := row[f.Fields[i].Name]
		if !ok {
			value = nil
		}
		f.Fields[i].Values = append(f.Fields[i].Values, value)
	}
	f.RowCount++
	f.buildFieldIndex()
	return nil
}

type FrameSet struct {
	Frames []*Frame
}

func NewFrameSet(frames ...*Frame) (*FrameSet, error) {
	fs := &FrameSet{}
	for _, fr := range frames {
		if fr == nil {
			continue
		}
		if err := fr.Normalize(); err != nil {
			return nil, err
		}
		fs.Frames = append(fs.Frames, fr.Clone())
	}
	return fs, nil
}

func (fs *FrameSet) Clone() *FrameSet {
	if fs == nil {
		return nil
	}
	frames := make([]*Frame, 0, len(fs.Frames))
	for _, fr := range fs.Frames {
		frames = append(frames, fr.Clone())
	}
	return &FrameSet{Frames: frames}
}

func (fs *FrameSet) Primary() *Frame {
	if fs == nil || len(fs.Frames) == 0 {
		return nil
	}
	return fs.Frames[0]
}

func InferFieldType(value any) FieldType {
	for value != nil {
		rv := reflect.ValueOf(value)
		if rv.Kind() != reflect.Pointer {
			break
		}
		if rv.IsNil() {
			return FieldTypeUnknown
		}
		value = rv.Elem().Interface()
	}
	switch value.(type) {
	case string:
		return FieldTypeString
	case bool:
		return FieldTypeBoolean
	case time.Time:
		return FieldTypeTime
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64:
		return FieldTypeNumber
	default:
		return FieldTypeUnknown
	}
}

func cloneValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		cloned := make(map[string]any, len(v))
		for key, item := range v {
			cloned[key] = cloneValue(item)
		}
		return cloned
	case []any:
		cloned := make([]any, len(v))
		for i, item := range v {
			cloned[i] = cloneValue(item)
		}
		return cloned
	default:
		return v
	}
}
