package crud

import "time"

type DateField interface {
	Field

	MinDate() time.Time
	MaxDate() time.Time
	Format() string
	WeekdaysOnly() bool
}

func NewDateField(
	name string,
	opts ...FieldOption,
) DateField {
	f := newField(
		name,
		DateFieldType,
		opts...,
	).(*field)

	return &dateField{field: f}
}

type dateField struct {
	*field
}

func (d *dateField) MinDate() time.Time {
	if val, ok := d.attrs[MinDate].(time.Time); ok {
		return val
	}
	// Return the minimum possible time value
	return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (d *dateField) MaxDate() time.Time {
	if val, ok := d.attrs[MaxDate].(time.Time); ok {
		return val
	}
	// Return the maximum possible time value
	return time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
}

func (d *dateField) Format() string {
	if val, ok := d.attrs[Format].(string); ok {
		return val
	}
	return "2006-01-02"
}

func (d *dateField) WeekdaysOnly() bool {
	if val, ok := d.attrs[WeekdaysOnly].(bool); ok {
		return val
	}
	return false
}

func (d *dateField) AsDateField() (DateField, error) {
	return d, nil
}

type TimeField interface {
	Field

	Format() string
}

func NewTimeField(
	name string,
	opts ...FieldOption,
) TimeField {
	f := newField(
		name,
		TimeFieldType,
		opts...,
	).(*field)

	return &timeField{field: f}
}

type timeField struct {
	*field
}

func (t *timeField) Format() string {
	if val, ok := t.attrs[Format].(string); ok {
		return val
	}
	return "15:04:05"
}

func (t *timeField) AsTimeField() (TimeField, error) {
	return t, nil
}

type DateTimeField interface {
	Field

	MinDateTime() time.Time
	MaxDateTime() time.Time
	Format() string
	Timezone() string
	WeekdaysOnly() bool
}

func NewDateTimeField(
	name string,
	opts ...FieldOption,
) DateTimeField {
	f := newField(
		name,
		DateTimeFieldType,
		opts...,
	).(*field)

	return &dateTimeField{field: f}
}

type dateTimeField struct {
	*field
}

func (dt *dateTimeField) MinDateTime() time.Time {
	if val, ok := dt.attrs[MinDate].(time.Time); ok {
		return val
	}
	// Return the minimum possible time value
	return time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
}

func (dt *dateTimeField) MaxDateTime() time.Time {
	if val, ok := dt.attrs[MaxDate].(time.Time); ok {
		return val
	}
	// Return the maximum possible time value
	return time.Date(9999, 12, 31, 23, 59, 59, 999999999, time.UTC)
}

func (dt *dateTimeField) Format() string {
	if val, ok := dt.attrs[Format].(string); ok {
		return val
	}
	return "2006-01-02 15:04:05"
}

func (dt *dateTimeField) Timezone() string {
	if val, ok := dt.attrs[Timezone].(string); ok {
		return val
	}
	return "UTC"
}

func (dt *dateTimeField) WeekdaysOnly() bool {
	if val, ok := dt.attrs[WeekdaysOnly].(bool); ok {
		return val
	}
	return false
}

func (dt *dateTimeField) AsDateTimeField() (DateTimeField, error) {
	return dt, nil
}

type TimestampField interface {
	Field
}

func NewTimestampField(
	name string,
	opts ...FieldOption,
) TimestampField {
	f := newField(
		name,
		TimestampFieldType,
		opts...,
	).(*field)

	return &timestampField{field: f}
}

type timestampField struct {
	*field
}

func (t *timestampField) AsTimestampField() (TimestampField, error) {
	return t, nil
}
