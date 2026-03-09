package frame

import (
	"fmt"
	"sort"
)

type Row map[string]any

type Builder struct {
	frame *Frame
	order []string
}

func NewBuilder(name string) *Builder {
	return &Builder{
		frame: &Frame{Name: name},
	}
}

func FromRows(name string, rows ...Row) (*FrameSet, error) {
	builder := NewBuilder(name)
	for _, row := range rows {
		if err := builder.Append(row); err != nil {
			return nil, err
		}
	}
	return builder.FrameSet()
}

func LongSeries(name string, rows ...LongSeriesRow) (*FrameSet, error) {
	builder := NewBuilder(name).
		String("category", RoleDimension).
		String("series", RoleSeries).
		Number("value", RoleMetric)
	for _, row := range rows {
		item := Row{
			"category": row.Category,
			"series":   row.Series,
			"value":    row.Value,
		}
		for key, value := range row.Extra {
			item[key] = value
		}
		if err := builder.Append(item); err != nil {
			return nil, err
		}
	}
	return builder.FrameSet()
}

type LongSeriesRow struct {
	Category string
	Series   string
	Value    any
	Extra    map[string]any
}

func (b *Builder) String(name string, role FieldRole) *Builder {
	return b.field(name, FieldTypeString, role)
}

func (b *Builder) Number(name string, role FieldRole) *Builder {
	return b.field(name, FieldTypeNumber, role)
}

func (b *Builder) Time(name string, role FieldRole) *Builder {
	return b.field(name, FieldTypeTime, role)
}

func (b *Builder) Bool(name string, role FieldRole) *Builder {
	return b.field(name, FieldTypeBoolean, role)
}

func (b *Builder) Localized(name string, role FieldRole) *Builder {
	return b.field(name, FieldTypeLocalized, role)
}

func (b *Builder) Append(row Row) error {
	if len(b.frame.Fields) == 0 {
		keys := make([]string, 0, len(row))
		for name := range row {
			keys = append(keys, name)
		}
		sort.Strings(keys)
		for _, name := range keys {
			value := row[name]
			b.frame.Fields = append(b.frame.Fields, Field{
				Name: name,
				Type: InferFieldType(value),
				Role: RoleDimension,
			})
			b.order = append(b.order, name)
		}
	} else {
		for _, name := range sortedMissingFields(b.frame.Fields, row) {
			b.frame.Fields = append(b.frame.Fields, Field{
				Name:   name,
				Type:   InferFieldType(row[name]),
				Role:   RoleDimension,
				Labels: map[string]string{},
			})
			b.order = append(b.order, name)
		}
	}
	return b.frame.AppendRow(row)
}

func (b *Builder) Frame() (*Frame, error) {
	if err := b.frame.Normalize(); err != nil {
		return nil, err
	}
	return b.frame.Clone(), nil
}

func (b *Builder) FrameSet() (*FrameSet, error) {
	fr, err := b.Frame()
	if err != nil {
		return nil, err
	}
	return NewFrameSet(fr)
}

func (b *Builder) field(name string, fieldType FieldType, role FieldRole) *Builder {
	for _, existing := range b.frame.Fields {
		if existing.Name == name {
			return b
		}
	}
	b.order = append(b.order, name)
	b.frame.Fields = append(b.frame.Fields, Field{
		Name:   name,
		Type:   fieldType,
		Role:   role,
		Labels: map[string]string{},
	})
	return b
}

func TimeBucketField(name string) Field {
	return Field{Name: name, Type: FieldTypeTime, Role: RoleTime, Labels: map[string]string{}}
}

func LabelField(name string) Field {
	return Field{Name: name, Type: FieldTypeString, Role: RoleDimension, Labels: map[string]string{}}
}

func MetricField(name string) Field {
	return Field{Name: name, Type: FieldTypeNumber, Role: RoleMetric, Labels: map[string]string{}}
}

func LinkField(name string) Field {
	return Field{Name: name, Type: FieldTypeString, Role: RoleLinkParam, Labels: map[string]string{}}
}

func (b *Builder) AppendStrict(row Row) error {
	for _, name := range b.order {
		if _, ok := row[name]; !ok {
			return fmt.Errorf("row missing field %q", name)
		}
	}
	for name := range row {
		if !containsField(b.order, name) {
			return fmt.Errorf("row has unexpected field %q", name)
		}
	}
	return b.Append(row)
}

func containsField(fields []string, name string) bool {
	for _, field := range fields {
		if field == name {
			return true
		}
	}
	return false
}

func sortedMissingFields(fields []Field, row Row) []string {
	existing := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		existing[field.Name] = struct{}{}
	}
	missing := make([]string, 0)
	for name := range row {
		if _, ok := existing[name]; ok {
			continue
		}
		missing = append(missing, name)
	}
	sort.Strings(missing)
	return missing
}
