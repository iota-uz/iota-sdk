package form

import (
	"context"
	"fmt"
	"io"

	"github.com/a-h/templ"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/input"
	"github.com/iota-uz/iota-sdk/components/base/radio"
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
	Key() string
	Label() string
	Type() FieldType
	Required() bool
	Attrs() templ.Attributes
	Validators() []Validator
}

// TextField for single-line text inputs
type TextField interface {
	Field
	Default() string
	MinLength() int
	MaxLength() int
}

// TextareaField for multi-line text inputs
type TextareaField interface {
	Field
	Default() string
	MinLength() int
	MaxLength() int
}

// CheckboxField for boolean inputs
type CheckboxField interface {
	Field
	Default() bool
}

// DateField for date inputs
type DateField interface {
	Field
	Default() string
	Min() string
	Max() string
}

// DateTimeLocalField for datetime-local inputs
type DateTimeLocalField interface {
	Field
	Default() string
	Min() string
	Max() string
}

// EmailField for email inputs
type EmailField interface {
	Field
	Default() string
}

// MonthField for month inputs
type MonthField interface {
	Field
	Default() string
	Min() string
	Max() string
}

// NumberField for numeric inputs
type NumberField interface {
	Field
	Default() float64
	Min() float64
	Max() float64
}

// RadioField for radio button inputs
type RadioField interface {
	Field
	Options() []Option
	Default() string
}

// TelField for telephone inputs
type TelField interface {
	Field
	Default() string
}

// TimeField for time inputs
type TimeField interface {
	Field
	Default() string
	Min() string
	Max() string
}

// URLField for URL inputs
type URLField interface {
	Field
	Default() string
}

// SelectField for dropdowns
type SelectField interface {
	Field
	Options() []Option
	Default() string
}

// --- Implementations of Field interfaces ---

type textField struct {
	key        string
	label      string
	defaultVal string
	required   bool
	minLen     int
	maxLen     int
	attrs      templ.Attributes
	validators []Validator
}

func (f *textField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *textField) Key() string             { return f.key }
func (f *textField) Label() string           { return f.label }
func (f *textField) Type() FieldType         { return FieldTypeText }
func (f *textField) Required() bool          { return f.required }
func (f *textField) Attrs() templ.Attributes { return f.attrs }
func (f *textField) Validators() []Validator { return f.validators }
func (f *textField) Default() string         { return f.defaultVal }
func (f *textField) MinLength() int          { return f.minLen }
func (f *textField) MaxLength() int          { return f.maxLen }

type textareaField struct {
	key        string
	label      string
	defaultVal string
	required   bool
	minLen     int
	maxLen     int
	attrs      templ.Attributes
	validators []Validator
}

func (f *textareaField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	return input.TextArea(&input.TextAreaProps{
		Placeholder: f.label,
		Label:       f.label,
		Value:       f.defaultVal,
		Attrs:       attrs,
	})
}
func (f *textareaField) Key() string             { return f.key }
func (f *textareaField) Label() string           { return f.label }
func (f *textareaField) Type() FieldType         { return FieldTypeTextarea }
func (f *textareaField) Required() bool          { return f.required }
func (f *textareaField) Attrs() templ.Attributes { return f.attrs }
func (f *textareaField) Validators() []Validator { return f.validators }
func (f *textareaField) Default() string         { return f.defaultVal }
func (f *textareaField) MinLength() int          { return f.minLen }
func (f *textareaField) MaxLength() int          { return f.maxLen }

type checkboxField struct {
	key        string
	label      string
	defaultVal bool
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *checkboxField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	return input.Checkbox(&input.CheckboxProps{
		Label:   f.label,
		Checked: f.defaultVal,
		Attrs:   attrs,
		ID:      f.key,
	})
}
func (f *checkboxField) Key() string             { return f.key }
func (f *checkboxField) Label() string           { return f.label }
func (f *checkboxField) Type() FieldType         { return FieldTypeCheckbox }
func (f *checkboxField) Required() bool          { return f.required }
func (f *checkboxField) Attrs() templ.Attributes { return f.attrs }
func (f *checkboxField) Validators() []Validator { return f.validators }
func (f *checkboxField) Default() bool           { return f.defaultVal }

type dateField struct {
	key        string
	label      string
	defaultVal string
	min        string
	max        string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *dateField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	attrs["type"] = string(FieldTypeDate)
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
func (f *dateField) Default() string         { return f.defaultVal }
func (f *dateField) Min() string             { return f.min }
func (f *dateField) Max() string             { return f.max }

type dateTimeLocalField struct {
	key        string
	label      string
	defaultVal string
	min        string
	max        string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *dateTimeLocalField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	attrs["type"] = string(FieldTypeDateTimeLocal)
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *dateTimeLocalField) Key() string             { return f.key }
func (f *dateTimeLocalField) Label() string           { return f.label }
func (f *dateTimeLocalField) Type() FieldType         { return FieldTypeDateTimeLocal }
func (f *dateTimeLocalField) Required() bool          { return f.required }
func (f *dateTimeLocalField) Attrs() templ.Attributes { return f.attrs }
func (f *dateTimeLocalField) Validators() []Validator { return f.validators }
func (f *dateTimeLocalField) Default() string         { return f.defaultVal }
func (f *dateTimeLocalField) Min() string             { return f.min }
func (f *dateTimeLocalField) Max() string             { return f.max }

type emailField struct {
	key        string
	label      string
	defaultVal string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *emailField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	return input.Email(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *emailField) Key() string             { return f.key }
func (f *emailField) Label() string           { return f.label }
func (f *emailField) Type() FieldType         { return FieldTypeEmail }
func (f *emailField) Required() bool          { return f.required }
func (f *emailField) Attrs() templ.Attributes { return f.attrs }
func (f *emailField) Validators() []Validator { return f.validators }
func (f *emailField) Default() string         { return f.defaultVal }

type monthField struct {
	key        string
	label      string
	defaultVal string
	min        string
	max        string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *monthField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	attrs["type"] = string(FieldTypeMonth)
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *monthField) Key() string             { return f.key }
func (f *monthField) Label() string           { return f.label }
func (f *monthField) Type() FieldType         { return FieldTypeMonth }
func (f *monthField) Required() bool          { return f.required }
func (f *monthField) Attrs() templ.Attributes { return f.attrs }
func (f *monthField) Validators() []Validator { return f.validators }
func (f *monthField) Default() string         { return f.defaultVal }
func (f *monthField) Min() string             { return f.min }
func (f *monthField) Max() string             { return f.max }

type numberField struct {
	key        string
	label      string
	defaultVal float64
	min        float64
	max        float64
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *numberField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = fmt.Sprint(f.defaultVal)
	return input.Number(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *numberField) Key() string             { return f.key }
func (f *numberField) Label() string           { return f.label }
func (f *numberField) Type() FieldType         { return FieldTypeNumber }
func (f *numberField) Required() bool          { return f.required }
func (f *numberField) Attrs() templ.Attributes { return f.attrs }
func (f *numberField) Validators() []Validator { return f.validators }
func (f *numberField) Default() float64        { return f.defaultVal }
func (f *numberField) Min() float64            { return f.min }
func (f *numberField) Max() float64            { return f.max }

type radioField struct {
	key        string
	label      string
	defaultVal string
	options    []Option
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *radioField) Component() templ.Component {
	children := []templ.Component{}
	for _, opt := range f.options {
		children = append(children, radio.CardItem(radio.CardItemProps{
			Name:    f.key,
			Value:   opt.Value,
			Checked: opt.Value == f.defaultVal,
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
func (f *radioField) Required() bool          { return f.required }
func (f *radioField) Attrs() templ.Attributes { return f.attrs }
func (f *radioField) Validators() []Validator { return f.validators }
func (f *radioField) Options() []Option       { return f.options }
func (f *radioField) Default() string         { return f.defaultVal }

type telField struct {
	key        string
	label      string
	defaultVal string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *telField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	attrs["type"] = string(FieldTypeTel)
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *telField) Key() string             { return f.key }
func (f *telField) Label() string           { return f.label }
func (f *telField) Type() FieldType         { return FieldTypeTel }
func (f *telField) Required() bool          { return f.required }
func (f *telField) Attrs() templ.Attributes { return f.attrs }
func (f *telField) Validators() []Validator { return f.validators }
func (f *telField) Default() string         { return f.defaultVal }

type timeField struct {
	key        string
	label      string
	defaultVal string
	min        string
	max        string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *timeField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	attrs["type"] = string(FieldTypeTime)
	return input.Text(&input.Props{
		Placeholder: f.label,
		Label:       f.label,
		Attrs:       attrs,
	})
}
func (f *timeField) Key() string             { return f.key }
func (f *timeField) Label() string           { return f.label }
func (f *timeField) Type() FieldType         { return FieldTypeTime }
func (f *timeField) Required() bool          { return f.required }
func (f *timeField) Attrs() templ.Attributes { return f.attrs }
func (f *timeField) Validators() []Validator { return f.validators }
func (f *timeField) Default() string         { return f.defaultVal }
func (f *timeField) Min() string             { return f.min }
func (f *timeField) Max() string             { return f.max }

type urlField struct {
	key        string
	label      string
	defaultVal string
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *urlField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	attrs["value"] = f.defaultVal
	attrs["type"] = string(FieldTypeURL)
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

type selectField struct {
	key        string
	label      string
	defaultVal string
	options    []Option
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func (f *selectField) Component() templ.Component {
	attrs := templ.Attributes{}
	for k, v := range f.attrs {
		attrs[k] = v
	}
	attrs["name"] = f.key
	return base.Select(&base.SelectProps{
		Label:       f.label,
		Placeholder: f.label,
		Attrs:       attrs,
	})
}
func (f *selectField) Key() string             { return f.key }
func (f *selectField) Label() string           { return f.label }
func (f *selectField) Type() FieldType         { return FieldTypeSelect }
func (f *selectField) Required() bool          { return f.required }
func (f *selectField) Attrs() templ.Attributes { return f.attrs }
func (f *selectField) Validators() []Validator { return f.validators }
func (f *selectField) Options() []Option       { return f.options }
func (f *selectField) Default() string         { return f.defaultVal }

// --- Fluent Builders ---

// TextFieldBuilder builds a TextField
type TextFieldBuilder struct {
	key, label, defaultVal string
	required               bool
	minLen, maxLen         int
	attrs                  templ.Attributes
	validators             []Validator
}

func NewTextField(key, label string) *TextFieldBuilder {
	return &TextFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *TextFieldBuilder) Default(val string) *TextFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *TextFieldBuilder) Required() *TextFieldBuilder {
	b.required = true
	return b
}
func (b *TextFieldBuilder) MinLen(min int) *TextFieldBuilder {
	b.minLen = min
	return b
}
func (b *TextFieldBuilder) MaxLen(max int) *TextFieldBuilder {
	b.maxLen = max
	return b
}
func (b *TextFieldBuilder) Attrs(a templ.Attributes) *TextFieldBuilder {
	b.attrs = a
	return b
}
func (b *TextFieldBuilder) Validators(v []Validator) *TextFieldBuilder {
	b.validators = v
	return b
}

func (b *TextFieldBuilder) Build() TextField {
	return &textField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		required:   b.required,
		minLen:     b.minLen,
		maxLen:     b.maxLen,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// TextareaFieldBuilder builds a TextareaField
type TextareaFieldBuilder struct {
	key, label, defaultVal string
	required               bool
	minLen, maxLen         int
	attrs                  templ.Attributes
	validators             []Validator
}

func NewTextareaField(key, label string) *TextareaFieldBuilder {
	return &TextareaFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *TextareaFieldBuilder) Default(val string) *TextareaFieldBuilder {
	b.defaultVal = val
	return b
}

func (b *TextareaFieldBuilder) Required() *TextareaFieldBuilder {
	b.required = true
	return b
}

func (b *TextareaFieldBuilder) MinLen(min int) *TextareaFieldBuilder {
	b.minLen = min
	return b
}

func (b *TextareaFieldBuilder) MaxLen(max int) *TextareaFieldBuilder {
	b.maxLen = max
	return b
}

func (b *TextareaFieldBuilder) Attrs(a templ.Attributes) *TextareaFieldBuilder {
	b.attrs = a
	return b
}

func (b *TextareaFieldBuilder) Validators(v []Validator) *TextareaFieldBuilder {
	b.validators = v
	return b
}

func (b *TextareaFieldBuilder) Build() TextareaField {
	return &textareaField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		required:   b.required,
		minLen:     b.minLen,
		maxLen:     b.maxLen,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// CheckboxFieldBuilder builds a CheckboxField
type CheckboxFieldBuilder struct {
	key        string
	label      string
	defaultVal bool
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func NewCheckboxField(key, label string) *CheckboxFieldBuilder {
	return &CheckboxFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *CheckboxFieldBuilder) Default(val bool) *CheckboxFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *CheckboxFieldBuilder) Required() *CheckboxFieldBuilder {
	b.required = true
	return b
}
func (b *CheckboxFieldBuilder) Attrs(a templ.Attributes) *CheckboxFieldBuilder {
	b.attrs = a
	return b
}
func (b *CheckboxFieldBuilder) Validators(v []Validator) *CheckboxFieldBuilder {
	b.validators = v
	return b
}

func (b *CheckboxFieldBuilder) Build() CheckboxField {
	return &checkboxField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// DateFieldBuilder builds a DateField
type DateFieldBuilder struct {
	key, label, defaultVal, min, max string
	required                         bool
	attrs                            templ.Attributes
	validators                       []Validator
}

func NewDateField(key, label string) *DateFieldBuilder {
	return &DateFieldBuilder{
		key:   key,
		label: label,
		attrs: templ.Attributes{},
	}
}

func (b *DateFieldBuilder) Default(val string) *DateFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *DateFieldBuilder) Min(val string) *DateFieldBuilder {
	b.min = val
	b.attrs["min"] = val
	return b
}
func (b *DateFieldBuilder) Max(val string) *DateFieldBuilder {
	b.max = val
	b.attrs["max"] = val
	return b
}
func (b *DateFieldBuilder) Required() *DateFieldBuilder {
	b.required = true
	b.attrs["required"] = true
	return b
}
func (b *DateFieldBuilder) Attrs(a templ.Attributes) *DateFieldBuilder {
	for k, v := range a {
		b.attrs[k] = v
	}
	return b
}
func (b *DateFieldBuilder) Validators(v []Validator) *DateFieldBuilder {
	b.validators = v
	return b
}

func (b *DateFieldBuilder) Build() DateField {
	return &dateField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		min:        b.min,
		max:        b.max,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// DateTimeLocalFieldBuilder builds a DateTimeLocalField
type DateTimeLocalFieldBuilder struct {
	key, label, defaultVal, min, max string
	required                         bool
	attrs                            templ.Attributes
	validators                       []Validator
}

func NewDateTimeLocalField(key, label string) *DateTimeLocalFieldBuilder {
	return &DateTimeLocalFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *DateTimeLocalFieldBuilder) Default(val string) *DateTimeLocalFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *DateTimeLocalFieldBuilder) Min(val string) *DateTimeLocalFieldBuilder {
	b.min = val
	return b
}
func (b *DateTimeLocalFieldBuilder) Max(val string) *DateTimeLocalFieldBuilder {
	b.max = val
	return b
}
func (b *DateTimeLocalFieldBuilder) Required() *DateTimeLocalFieldBuilder {
	b.required = true
	return b
}
func (b *DateTimeLocalFieldBuilder) Attrs(a templ.Attributes) *DateTimeLocalFieldBuilder {
	b.attrs = a
	return b
}
func (b *DateTimeLocalFieldBuilder) Validators(v []Validator) *DateTimeLocalFieldBuilder {
	b.validators = v
	return b
}

func (b *DateTimeLocalFieldBuilder) Build() DateTimeLocalField {
	return &dateTimeLocalField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		min:        b.min,
		max:        b.max,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// EmailFieldBuilder builds an EmailField
type EmailFieldBuilder struct {
	key, label, defaultVal string
	required               bool
	attrs                  templ.Attributes
	validators             []Validator
}

func NewEmailField(key, label string) *EmailFieldBuilder {
	return &EmailFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *EmailFieldBuilder) Default(val string) *EmailFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *EmailFieldBuilder) Required() *EmailFieldBuilder {
	b.required = true
	return b
}
func (b *EmailFieldBuilder) Attrs(a templ.Attributes) *EmailFieldBuilder {
	b.attrs = a
	return b
}
func (b *EmailFieldBuilder) Validators(v []Validator) *EmailFieldBuilder {
	b.validators = v
	return b
}

func (b *EmailFieldBuilder) Build() EmailField {
	return &emailField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// MonthFieldBuilder builds a MonthField
type MonthFieldBuilder struct {
	key, label, defaultVal, min, max string
	required                         bool
	attrs                            templ.Attributes
	validators                       []Validator
}

func NewMonthField(key, label string) *MonthFieldBuilder {
	return &MonthFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *MonthFieldBuilder) Default(val string) *MonthFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *MonthFieldBuilder) Min(val string) *MonthFieldBuilder {
	b.min = val
	return b
}
func (b *MonthFieldBuilder) Max(val string) *MonthFieldBuilder {
	b.max = val
	return b
}
func (b *MonthFieldBuilder) Required() *MonthFieldBuilder {
	b.required = true
	return b
}
func (b *MonthFieldBuilder) Attrs(a templ.Attributes) *MonthFieldBuilder {
	b.attrs = a
	return b
}
func (b *MonthFieldBuilder) Validators(v []Validator) *MonthFieldBuilder {
	b.validators = v
	return b
}

func (b *MonthFieldBuilder) Build() MonthField {
	return &monthField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		min:        b.min,
		max:        b.max,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// NumberFieldBuilder builds a NumberField
type NumberFieldBuilder struct {
	key        string
	label      string
	defaultVal float64
	min, max   float64
	required   bool
	attrs      templ.Attributes
	validators []Validator
}

func NewNumberField(key, label string) *NumberFieldBuilder {
	return &NumberFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *NumberFieldBuilder) Default(val float64) *NumberFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *NumberFieldBuilder) Min(val float64) *NumberFieldBuilder {
	b.min = val
	return b
}
func (b *NumberFieldBuilder) Max(val float64) *NumberFieldBuilder {
	b.max = val
	return b
}
func (b *NumberFieldBuilder) Required() *NumberFieldBuilder {
	b.required = true
	return b
}
func (b *NumberFieldBuilder) Attrs(a templ.Attributes) *NumberFieldBuilder {
	b.attrs = a
	return b
}
func (b *NumberFieldBuilder) Validators(v []Validator) *NumberFieldBuilder {
	b.validators = v
	return b
}

func (b *NumberFieldBuilder) Build() NumberField {
	return &numberField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		min:        b.min,
		max:        b.max,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// RadioFieldBuilder builds a RadioField
type RadioFieldBuilder struct {
	key, label, defaultVal string
	options                []Option
	required               bool
	attrs                  templ.Attributes
	validators             []Validator
}

func NewRadioField(key, label string) *RadioFieldBuilder {
	return &RadioFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *RadioFieldBuilder) Options(opts []Option) *RadioFieldBuilder {
	b.options = opts
	return b
}
func (b *RadioFieldBuilder) Default(val string) *RadioFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *RadioFieldBuilder) Required() *RadioFieldBuilder {
	b.required = true
	return b
}
func (b *RadioFieldBuilder) Attrs(a templ.Attributes) *RadioFieldBuilder {
	b.attrs = a
	return b
}
func (b *RadioFieldBuilder) Validators(v []Validator) *RadioFieldBuilder {
	b.validators = v
	return b
}

func (b *RadioFieldBuilder) Build() RadioField {
	return &radioField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		options:    b.options,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// TelFieldBuilder builds a TelField
type TelFieldBuilder struct {
	key, label, defaultVal string
	required               bool
	attrs                  templ.Attributes
	validators             []Validator
}

func NewTelField(key, label string) *TelFieldBuilder {
	return &TelFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *TelFieldBuilder) Default(val string) *TelFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *TelFieldBuilder) Required() *TelFieldBuilder {
	b.required = true
	return b
}
func (b *TelFieldBuilder) Attrs(a templ.Attributes) *TelFieldBuilder {
	b.attrs = a
	return b
}
func (b *TelFieldBuilder) Validators(v []Validator) *TelFieldBuilder {
	b.validators = v
	return b
}

func (b *TelFieldBuilder) Build() TelField {
	return &telField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// TimeFieldBuilder builds a TimeField
type TimeFieldBuilder struct {
	key, label, defaultVal, min, max string
	required                         bool
	attrs                            templ.Attributes
	validators                       []Validator
}

func NewTimeField(key, label string) *TimeFieldBuilder {
	return &TimeFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *TimeFieldBuilder) Default(val string) *TimeFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *TimeFieldBuilder) Min(val string) *TimeFieldBuilder {
	b.min = val
	return b
}
func (b *TimeFieldBuilder) Max(val string) *TimeFieldBuilder {
	b.max = val
	return b
}
func (b *TimeFieldBuilder) Required() *TimeFieldBuilder {
	b.required = true
	return b
}
func (b *TimeFieldBuilder) Attrs(a templ.Attributes) *TimeFieldBuilder {
	b.attrs = a
	return b
}
func (b *TimeFieldBuilder) Validators(v []Validator) *TimeFieldBuilder {
	b.validators = v
	return b
}

func (b *TimeFieldBuilder) Build() TimeField {
	return &timeField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		min:        b.min,
		max:        b.max,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// URLFieldBuilder builds a URLField
type URLFieldBuilder struct {
	key, label, defaultVal string
	required               bool
	attrs                  templ.Attributes
	validators             []Validator
}

func NewURLField(key, label string) *URLFieldBuilder {
	return &URLFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *URLFieldBuilder) Default(val string) *URLFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *URLFieldBuilder) Required() *URLFieldBuilder {
	b.required = true
	return b
}
func (b *URLFieldBuilder) Attrs(a templ.Attributes) *URLFieldBuilder {
	b.attrs = a
	return b
}
func (b *URLFieldBuilder) Validators(v []Validator) *URLFieldBuilder {
	b.validators = v
	return b
}

func (b *URLFieldBuilder) Build() URLField {
	return &urlField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// SelectFieldBuilder builds a SelectField
type SelectFieldBuilder struct {
	key, label, defaultVal string
	options                []Option
	required               bool
	attrs                  templ.Attributes
	validators             []Validator
}

func NewSelectField(key, label string) *SelectFieldBuilder {
	return &SelectFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *SelectFieldBuilder) Default(val string) *SelectFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *SelectFieldBuilder) Options(opts []Option) *SelectFieldBuilder {
	b.options = opts
	return b
}
func (b *SelectFieldBuilder) Required() *SelectFieldBuilder {
	b.required = true
	return b
}
func (b *SelectFieldBuilder) Attrs(a templ.Attributes) *SelectFieldBuilder {
	b.attrs = a
	return b
}
func (b *SelectFieldBuilder) Validators(v []Validator) *SelectFieldBuilder {
	b.validators = v
	return b
}

func (b *SelectFieldBuilder) Build() SelectField {
	return &selectField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		options:    b.options,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}
