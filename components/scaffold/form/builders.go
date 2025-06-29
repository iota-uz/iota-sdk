package form

import (
	"time"

	"github.com/a-h/templ"
)

// TextFieldBuilder builds a TextField
type TextFieldBuilder struct {
	key, label, defaultVal string
	required               bool
	minLen, maxLen         int
	attrs                  templ.Attributes
	validators             []Validator
}

func Text(key, label string) *TextFieldBuilder {
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

func (b *TextFieldBuilder) MinLen(v int) *TextFieldBuilder {
	b.minLen = v
	return b
}

func (b *TextFieldBuilder) MaxLen(v int) *TextFieldBuilder {
	b.maxLen = v
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

func Textarea(key, label string) *TextareaFieldBuilder {
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

func (b *TextareaFieldBuilder) MinLen(v int) *TextareaFieldBuilder {
	b.minLen = v
	return b
}

func (b *TextareaFieldBuilder) MaxLen(v int) *TextareaFieldBuilder {
	b.maxLen = v
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

func Checkbox(key, label string) *CheckboxFieldBuilder {
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
	key, label           string
	defaultVal, min, max time.Time
	required             bool
	attrs                templ.Attributes
	validators           []Validator
}

func Date(key, label string) *DateFieldBuilder {
	return &DateFieldBuilder{
		key:   key,
		label: label,
		attrs: templ.Attributes{},
	}
}

func (b *DateFieldBuilder) Default(val time.Time) *DateFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *DateFieldBuilder) Min(val time.Time) *DateFieldBuilder {
	b.min = val
	b.attrs["min"] = val.Format("2006-01-02")
	return b
}
func (b *DateFieldBuilder) Max(val time.Time) *DateFieldBuilder {
	b.max = val
	b.attrs["max"] = val.Format("2006-01-02")
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
	key, label           string
	defaultVal, min, max time.Time
	required             bool
	attrs                templ.Attributes
	validators           []Validator
}

func DateTime(key, label string) *DateTimeLocalFieldBuilder {
	return &DateTimeLocalFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *DateTimeLocalFieldBuilder) Default(val time.Time) *DateTimeLocalFieldBuilder {
	b.defaultVal = val
	return b
}
func (b *DateTimeLocalFieldBuilder) Min(val time.Time) *DateTimeLocalFieldBuilder {
	b.min = val
	return b
}
func (b *DateTimeLocalFieldBuilder) Max(val time.Time) *DateTimeLocalFieldBuilder {
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

func Email(key, label string) *EmailFieldBuilder {
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

func Month(key, label string) *MonthFieldBuilder {
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

func Tel(key, label string) *TelFieldBuilder {
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

func Time(key, label string) *TimeFieldBuilder {
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

func URL(key, label string) *URLFieldBuilder {
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

func Select(key, label string) *SelectFieldBuilder {
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

type ColorFieldBuilder struct {
	key, label, defaultVal string
	required               bool
	attrs                  templ.Attributes
	validators             []Validator
}

func Color(key, label string) *ColorFieldBuilder {
	return &ColorFieldBuilder{key: key, label: label, attrs: templ.Attributes{}}
}

func (b *ColorFieldBuilder) Default(val string) *ColorFieldBuilder {
	b.defaultVal = val
	return b
}

func (b *ColorFieldBuilder) Required() *ColorFieldBuilder {
	b.required = true
	return b
}

func (b *ColorFieldBuilder) Attrs(a templ.Attributes) *ColorFieldBuilder {
	b.attrs = a
	return b
}

func (b *ColorFieldBuilder) Validators(v []Validator) *ColorFieldBuilder {
	b.validators = v
	return b
}

func (b *ColorFieldBuilder) Build() ColorField {
	return &colorField{
		key:        b.key,
		label:      b.label,
		defaultVal: b.defaultVal,
		required:   b.required,
		attrs:      b.attrs,
		validators: b.validators,
	}
}

// SearchSelectFieldBuilder for async search select fields
type SearchSelectFieldBuilder struct {
	key, label, defaultVal, endpoint, placeholder string
	required                                      bool
	attrs                                         templ.Attributes
	validators                                    []Validator
}

func SearchSelect() *SearchSelectFieldBuilder {
	return &SearchSelectFieldBuilder{attrs: templ.Attributes{}}
}

func (b *SearchSelectFieldBuilder) Key(key string) *SearchSelectFieldBuilder {
	b.key = key
	return b
}

func (b *SearchSelectFieldBuilder) Label(label string) *SearchSelectFieldBuilder {
	b.label = label
	return b
}

func (b *SearchSelectFieldBuilder) Default(val string) *SearchSelectFieldBuilder {
	b.defaultVal = val
	return b
}

func (b *SearchSelectFieldBuilder) Endpoint(endpoint string) *SearchSelectFieldBuilder {
	b.endpoint = endpoint
	return b
}

func (b *SearchSelectFieldBuilder) Placeholder(placeholder string) *SearchSelectFieldBuilder {
	b.placeholder = placeholder
	return b
}

func (b *SearchSelectFieldBuilder) Required(required bool) *SearchSelectFieldBuilder {
	b.required = required
	return b
}

func (b *SearchSelectFieldBuilder) WithRequired(required bool) *SearchSelectFieldBuilder {
	b.required = required
	return b
}

func (b *SearchSelectFieldBuilder) WithValue(value string) *SearchSelectFieldBuilder {
	// This is handled in the field itself
	return b
}

func (b *SearchSelectFieldBuilder) Attrs(a templ.Attributes) *SearchSelectFieldBuilder {
	b.attrs = a
	return b
}

func (b *SearchSelectFieldBuilder) Validators(v []Validator) *SearchSelectFieldBuilder {
	b.validators = v
	return b
}

func (b *SearchSelectFieldBuilder) Build() SearchSelectField {
	return &searchSelectField{
		key:         b.key,
		label:       b.label,
		defaultVal:  b.defaultVal,
		endpoint:    b.endpoint,
		placeholder: b.placeholder,
		required:    b.required,
		attrs:       b.attrs,
		validators:  b.validators,
	}
}

// ComboboxFieldBuilder for multi-select fields
type ComboboxFieldBuilder struct {
	key, label, defaultVal, endpoint, placeholder string
	required, multiple, searchable                bool
	attrs                                         templ.Attributes
	validators                                    []Validator
}

func Combobox() *ComboboxFieldBuilder {
	return &ComboboxFieldBuilder{attrs: templ.Attributes{}, searchable: true}
}

func (b *ComboboxFieldBuilder) Key(key string) *ComboboxFieldBuilder {
	b.key = key
	return b
}

func (b *ComboboxFieldBuilder) Label(label string) *ComboboxFieldBuilder {
	b.label = label
	return b
}

func (b *ComboboxFieldBuilder) Default(val string) *ComboboxFieldBuilder {
	b.defaultVal = val
	return b
}

func (b *ComboboxFieldBuilder) Endpoint(endpoint string) *ComboboxFieldBuilder {
	b.endpoint = endpoint
	return b
}

func (b *ComboboxFieldBuilder) Placeholder(placeholder string) *ComboboxFieldBuilder {
	b.placeholder = placeholder
	return b
}

func (b *ComboboxFieldBuilder) Multiple(multiple bool) *ComboboxFieldBuilder {
	b.multiple = multiple
	return b
}

func (b *ComboboxFieldBuilder) Searchable(searchable bool) *ComboboxFieldBuilder {
	b.searchable = searchable
	return b
}

func (b *ComboboxFieldBuilder) Required(required bool) *ComboboxFieldBuilder {
	b.required = required
	return b
}

func (b *ComboboxFieldBuilder) WithRequired(required bool) *ComboboxFieldBuilder {
	b.required = required
	return b
}

func (b *ComboboxFieldBuilder) WithMultiple(multiple bool) *ComboboxFieldBuilder {
	b.multiple = multiple
	return b
}

func (b *ComboboxFieldBuilder) WithValue(value string) *ComboboxFieldBuilder {
	// This is handled in the field itself
	return b
}

func (b *ComboboxFieldBuilder) Attrs(a templ.Attributes) *ComboboxFieldBuilder {
	b.attrs = a
	return b
}

func (b *ComboboxFieldBuilder) Validators(v []Validator) *ComboboxFieldBuilder {
	b.validators = v
	return b
}

func (b *ComboboxFieldBuilder) Build() ComboboxField {
	return &comboboxField{
		key:         b.key,
		label:       b.label,
		defaultVal:  b.defaultVal,
		endpoint:    b.endpoint,
		placeholder: b.placeholder,
		multiple:    b.multiple,
		searchable:  b.searchable,
		required:    b.required,
		attrs:       b.attrs,
		validators:  b.validators,
	}
}
