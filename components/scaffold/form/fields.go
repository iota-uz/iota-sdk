package form

import (
	"context"
	"io"
	"time"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/input"
	"github.com/iota-uz/iota-sdk/components/base/radio"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
)

// --- FieldType enumerates supported input types ---
type FieldType string

const (
	FieldTypeText          FieldType = "text"
	FieldTypeCheckbox      FieldType = "checkbox"
	FieldTypeDate          FieldType = "date"
	FieldTypeDateTimeLocal FieldType = "datetime-local"
	FieldTypeEmail         FieldType = "email"
	FieldTypeMonth         FieldType = "month"
	FieldTypeNumber        FieldType = "number"
	FieldTypeColor         FieldType = "color"
	FieldTypeRadio         FieldType = "radio"
	FieldTypeTel           FieldType = "tel"
	FieldTypeTime          FieldType = "time"
	FieldTypeURL           FieldType = "url"
	FieldTypeTextarea      FieldType = "textarea"
	FieldTypeSelect        FieldType = "select"
)

// Option for SelectField and RadioField choices
type Option struct {
	Value string
	Label string
}

// --- Field interfaces ---

// Validator for custom field-level checks
type Validator interface {
	Validate(ctx context.Context, value any) error
}

// Field defines minimal metadata for form inputs and rendering
type Field interface {
	Component() templ.Component
	Type() FieldType
	Key() string
	Label() string
	Required() bool
	Attrs() templ.Attributes
	Validators() []Validator
}

// GenericField defines minimal metadata for form inputs and rendering
type GenericField[T any] interface {
	Field
	Default() T
	WithValue(value T) GenericField[T]
	Value() T
}

// TextField for single-line text inputs
type TextField interface {
	GenericField[string]
	MinLength() int
	MaxLength() int
}

// TextareaField for multi-line text inputs
type TextareaField interface {
	GenericField[string]
	MinLength() int
	MaxLength() int
}

// CheckboxField for boolean inputs
type CheckboxField interface {
	GenericField[bool]
}

// DateField for date inputs
type DateField interface {
	GenericField[time.Time]
	Min() time.Time
	Max() time.Time
}

// DateTimeLocalField for datetime-local inputs
type DateTimeLocalField interface {
	GenericField[time.Time]
	Min() time.Time
	Max() time.Time
}

// EmailField for email inputs
type EmailField interface {
	GenericField[string]
}

// MonthField for month inputs
type MonthField interface {
	GenericField[string]
	Min() string
	Max() string
}

// NumberField for numeric inputs
type NumberField interface {
	GenericField[float64]
	Min() float64
	Max() float64
}

// RadioField for radio button inputs
type RadioField interface {
	GenericField[string]
	Options() []Option
}

// TelField for telephone inputs
type TelField interface {
	GenericField[string]
}

// TimeField for time inputs
type TimeField interface {
	GenericField[string]
	Min() string
	Max() string
}

// ColorField for color inputs
type ColorField interface {
	GenericField[string]
}

// URLField for URL inputs
type URLField interface {
	GenericField[string]
}

// SelectField for dropdowns
type SelectField interface {
	GenericField[string]
	Options() []Option
}

// --- Implementations of Field interfaces ---

type textField struct {
	key        string
	label      string
	value      string
	defaultVal string
	required   bool
	minLen     int
	maxLen     int
	attrs      templ.Attributes
	validators []Validator
}

func (f *textField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *textField) Key() string             { return f.key }
func (f *textField) Label() string           { return f.label }
func (f *textField) Type() FieldType         { return FieldTypeText }
func (f *textField) Value() string           { return f.value }
func (f *textField) Required() bool          { return f.required }
func (f *textField) Attrs() templ.Attributes { return f.attrs }
func (f *textField) Validators() []Validator { return f.validators }
func (f *textField) Default() string         { return f.defaultVal }
func (f *textField) MinLength() int          { return f.minLen }
func (f *textField) MaxLength() int          { return f.maxLen }
func (f *textField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type textareaField struct {
	key        string
	label      string
	defaultVal string
	value      string
	required   bool
	minLen     int
	maxLen     int
	attrs      templ.Attributes
	validators []Validator
}

func (f *textareaField) Component() templ.Component {
	attrs := templ.Attributes{
		"name": f.key,
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.TextArea(&input.TextAreaProps{
		Placeholder: f.label,
		Label:       f.label,
		Value:       mapping.Or(f.value, f.defaultVal),
		Attrs:       attrs,
	})
}

func (f *textareaField) Key() string             { return f.key }
func (f *textareaField) Label() string           { return f.label }
func (f *textareaField) Type() FieldType         { return FieldTypeTextarea }
func (f *textareaField) Value() string           { return f.value }
func (f *textareaField) Required() bool          { return f.required }
func (f *textareaField) Attrs() templ.Attributes { return f.attrs }
func (f *textareaField) Validators() []Validator { return f.validators }
func (f *textareaField) Default() string         { return f.defaultVal }
func (f *textareaField) MinLength() int          { return f.minLen }
func (f *textareaField) MaxLength() int          { return f.maxLen }
func (f *textareaField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type checkboxField struct {
	key        string
	label      string
	defaultVal bool
	value      bool
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *checkboxField) Component() templ.Component {
	attrs := templ.Attributes{
		"name": f.key,
		"type": string(FieldTypeCheckbox),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Checkbox(&input.CheckboxProps{
		Label:   f.label,
		Checked: f.value,
		Attrs:   attrs,
		ID:      f.key,
	})
}
func (f *checkboxField) Key() string             { return f.key }
func (f *checkboxField) Label() string           { return f.label }
func (f *checkboxField) Type() FieldType         { return FieldTypeCheckbox }
func (f *checkboxField) Value() bool             { return f.value }
func (f *checkboxField) Required() bool          { return f.required }
func (f *checkboxField) Attrs() templ.Attributes { return f.attrs }
func (f *checkboxField) Validators() []Validator { return f.validators }
func (f *checkboxField) Default() bool           { return f.defaultVal }
func (f *checkboxField) WithValue(value bool) GenericField[bool] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type dateField struct {
	key        string
	label      string
	value      time.Time
	defaultVal time.Time
	min        time.Time
	max        time.Time
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *dateField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"value": f.defaultVal,
		"type":  string(FieldTypeDate),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Date(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *dateField) Key() string             { return f.key }
func (f *dateField) Label() string           { return f.label }
func (f *dateField) Type() FieldType         { return FieldTypeDate }
func (f *dateField) Required() bool          { return f.required }
func (f *dateField) Attrs() templ.Attributes { return f.attrs }
func (f *dateField) Validators() []Validator { return f.validators }
func (f *dateField) Default() time.Time      { return f.defaultVal }
func (f *dateField) Value() time.Time        { return f.value }
func (f *dateField) Min() time.Time          { return f.min }
func (f *dateField) Max() time.Time          { return f.max }
func (f *dateField) WithValue(value time.Time) GenericField[time.Time] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type dateTimeLocalField struct {
	key        string
	label      string
	value      time.Time
	defaultVal time.Time
	min        time.Time
	max        time.Time
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *dateTimeLocalField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"type":  string(FieldTypeDateTimeLocal),
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *dateTimeLocalField) Key() string             { return f.key }
func (f *dateTimeLocalField) Label() string           { return f.label }
func (f *dateTimeLocalField) Type() FieldType         { return FieldTypeDateTimeLocal }
func (f *dateTimeLocalField) Value() time.Time        { return f.value }
func (f *dateTimeLocalField) Required() bool          { return f.required }
func (f *dateTimeLocalField) Attrs() templ.Attributes { return f.attrs }
func (f *dateTimeLocalField) Validators() []Validator { return f.validators }
func (f *dateTimeLocalField) Default() time.Time      { return f.defaultVal }
func (f *dateTimeLocalField) Min() time.Time          { return f.min }
func (f *dateTimeLocalField) Max() time.Time          { return f.max }
func (f *dateTimeLocalField) WithValue(value time.Time) GenericField[time.Time] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type emailField struct {
	key        string
	label      string
	defaultVal string
	value      string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *emailField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"type":  string(FieldTypeEmail),
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Email(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *emailField) Key() string             { return f.key }
func (f *emailField) Label() string           { return f.label }
func (f *emailField) Type() FieldType         { return FieldTypeEmail }
func (f *emailField) Value() string           { return f.value }
func (f *emailField) Required() bool          { return f.required }
func (f *emailField) Attrs() templ.Attributes { return f.attrs }
func (f *emailField) Validators() []Validator { return f.validators }
func (f *emailField) Default() string         { return f.defaultVal }
func (f *emailField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type monthField struct {
	key        string
	label      string
	value      string
	defaultVal string
	min        string
	max        string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *monthField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"type":  string(FieldTypeMonth),
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *monthField) Key() string             { return f.key }
func (f *monthField) Label() string           { return f.label }
func (f *monthField) Type() FieldType         { return FieldTypeMonth }
func (f *monthField) Value() string           { return f.value }
func (f *monthField) Required() bool          { return f.required }
func (f *monthField) Attrs() templ.Attributes { return f.attrs }
func (f *monthField) Validators() []Validator { return f.validators }
func (f *monthField) Default() string         { return f.defaultVal }
func (f *monthField) Min() string             { return f.min }
func (f *monthField) Max() string             { return f.max }
func (f *monthField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type numberField struct {
	key        string
	label      string
	value      float64
	defaultVal float64
	min        float64
	max        float64
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *numberField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"type":  string(FieldTypeNumber),
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Number(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *numberField) Key() string             { return f.key }
func (f *numberField) Label() string           { return f.label }
func (f *numberField) Type() FieldType         { return FieldTypeNumber }
func (f *numberField) Value() float64          { return f.value }
func (f *numberField) Required() bool          { return f.required }
func (f *numberField) Attrs() templ.Attributes { return f.attrs }
func (f *numberField) Validators() []Validator { return f.validators }
func (f *numberField) Default() float64        { return f.defaultVal }
func (f *numberField) Min() float64            { return f.min }
func (f *numberField) Max() float64            { return f.max }
func (f *numberField) WithValue(value float64) GenericField[float64] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type radioField struct {
	key        string
	label      string
	value      string
	defaultVal string
	options    []Option
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *radioField) Component() templ.Component {
	children := []templ.Component{}
	selected := mapping.Or(f.value, f.defaultVal)
	for _, opt := range f.options {
		children = append(children, radio.CardItem(radio.CardItemProps{
			Name:    f.key,
			Value:   opt.Value,
			Checked: opt.Value == selected,
		}))
	}
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		return radio.RadioGroup(radio.RadioGroupProps{
			Label:       f.label,
			Orientation: radio.OrientationHorizontal,
		}).Render(templ.WithChildren(ctx, templ.Join(children...)), w)
	})
}
func (f *radioField) Key() string             { return f.key }
func (f *radioField) Label() string           { return f.label }
func (f *radioField) Type() FieldType         { return FieldTypeRadio }
func (f *radioField) Value() string           { return f.value }
func (f *radioField) Required() bool          { return f.required }
func (f *radioField) Attrs() templ.Attributes { return f.attrs }
func (f *radioField) Validators() []Validator { return f.validators }
func (f *radioField) Options() []Option       { return f.options }
func (f *radioField) Default() string         { return f.defaultVal }
func (f *radioField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type telField struct {
	key        string
	label      string
	value      string
	defaultVal string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *telField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"type":  string(FieldTypeTel),
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *telField) Key() string             { return f.key }
func (f *telField) Label() string           { return f.label }
func (f *telField) Type() FieldType         { return FieldTypeTel }
func (f *telField) Value() string           { return f.value }
func (f *telField) Required() bool          { return f.required }
func (f *telField) Attrs() templ.Attributes { return f.attrs }
func (f *telField) Validators() []Validator { return f.validators }
func (f *telField) Default() string         { return f.defaultVal }
func (f *telField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type timeField struct {
	key        string
	label      string
	value      string
	defaultVal string
	min        string
	max        string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *timeField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"type":  string(FieldTypeTime),
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *timeField) Key() string             { return f.key }
func (f *timeField) Label() string           { return f.label }
func (f *timeField) Type() FieldType         { return FieldTypeTime }
func (f *timeField) Value() string           { return f.value }
func (f *timeField) Required() bool          { return f.required }
func (f *timeField) Attrs() templ.Attributes { return f.attrs }
func (f *timeField) Validators() []Validator { return f.validators }
func (f *timeField) Default() string         { return f.defaultVal }
func (f *timeField) Min() string             { return f.min }
func (f *timeField) Max() string             { return f.max }
func (f *timeField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type urlField struct {
	key        string
	label      string
	defaultVal string
	required   bool
	value      string
	attrs      templ.Attributes
	validators []Validator
}

func (f *urlField) Value() string {
	return f.value
}

func (f *urlField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"type":  string(FieldTypeURL),
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *urlField) Key() string             { return f.key }
func (f *urlField) Label() string           { return f.label }
func (f *urlField) Type() FieldType         { return FieldTypeURL }
func (f *urlField) Required() bool          { return f.required }
func (f *urlField) Attrs() templ.Attributes { return f.attrs }
func (f *urlField) Validators() []Validator { return f.validators }
func (f *urlField) Default() string         { return f.defaultVal }
func (f *urlField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type selectField struct {
	key        string
	label      string
	value      string
	defaultVal string
	options    []Option
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *selectField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"value": mapping.Or(f.value, f.defaultVal),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return base.Select(&base.SelectProps{
		Label:       f.label,
		Placeholder: f.label,
		Attrs:       attrs,
	})
}
func (f *selectField) Key() string             { return f.key }
func (f *selectField) Label() string           { return f.label }
func (f *selectField) Type() FieldType         { return FieldTypeSelect }
func (f *selectField) Value() string           { return f.value }
func (f *selectField) Required() bool          { return f.required }
func (f *selectField) Attrs() templ.Attributes { return f.attrs }
func (f *selectField) Validators() []Validator { return f.validators }
func (f *selectField) Options() []Option       { return f.options }
func (f *selectField) Default() string         { return f.defaultVal }
func (f *selectField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}

type colorField struct {
	key        string
	label      string
	defaultVal string
	value      string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *colorField) Component() templ.Component {
	attrs := templ.Attributes{
		"name":  f.key,
		"value": mapping.Or(f.defaultVal, f.value),
	}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	return input.Color(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}

func (f *colorField) Key() string             { return f.key }
func (f *colorField) Label() string           { return f.label }
func (f *colorField) Type() FieldType         { return FieldTypeColor }
func (f *colorField) Value() string           { return f.value }
func (f *colorField) Required() bool          { return f.required }
func (f *colorField) Attrs() templ.Attributes { return f.attrs }
func (f *colorField) Validators() []Validator { return f.validators }
func (f *colorField) Default() string         { return f.defaultVal }
func (f *colorField) WithValue(value string) GenericField[string] {
	newField := *f // Create a copy
	newField.value = value
	return &newField
}
