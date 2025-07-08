package crud

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// SelectType defines how the select field behaves in the UI
type SelectType string

const (
	// SelectTypeStatic renders as a regular HTML select with predefined options
	SelectTypeStatic SelectType = "static"
	// SelectTypeSearchable renders as an async search select component
	SelectTypeSearchable SelectType = "searchable"
	// SelectTypeCombobox renders as a multi-select component with search
	SelectTypeCombobox SelectType = "combobox"
)

// SelectOption represents a single option in a select field
type SelectOption struct {
	Value any // Can be string, int, bool, etc.
	Label string
}

// SelectField interface extends Field with select-specific functionality
type SelectField interface {
	Field

	// Select behavior control
	SelectType() SelectType
	SetSelectType(SelectType) SelectField

	// Options management for static selects
	Options() []SelectOption
	SetOptions([]SelectOption) SelectField

	// Dynamic options loader
	OptionsLoader() func(ctx context.Context) []SelectOption
	SetOptionsLoader(func(ctx context.Context) []SelectOption) SelectField

	// For searchable/async options
	Endpoint() string
	SetEndpoint(string) SelectField

	// UI configuration
	Placeholder() string
	SetPlaceholder(string) SelectField
	Multiple() bool
	SetMultiple(bool) SelectField

	// Value type control (underlying data type)
	ValueType() FieldType
	SetValueType(FieldType) SelectField

	// Fluent API helpers
	AsIntSelect() SelectField
	AsStringSelect() SelectField
	AsBoolSelect() SelectField
	AsSearchable(endpoint string) SelectField
	AsCombobox() SelectField
	WithStaticOptions(options ...SelectOption) SelectField
	WithSearchEndpoint(endpoint string) SelectField
	WithCombobox(endpoint string, multiple bool) SelectField

	// Cache control
	InvalidateOptionsCache()
	SetOptionsCacheTTL(ttl time.Duration) SelectField
}

type selectField struct {
	*field
	selectType    SelectType
	options       []SelectOption
	optionsLoader func(ctx context.Context) []SelectOption
	cachedOptions []SelectOption
	cacheTime     atomic.Int64
	cacheTTL      atomic.Int64
	cacheMu       sync.RWMutex
	endpoint      string
	placeholder   string
	multiple      bool
	valueType     FieldType
}

// NewSelectField creates a new select field with string value type by default
func NewSelectField(name string, opts ...FieldOption) SelectField {
	sf := &selectField{
		selectType: SelectTypeStatic,
		valueType:  StringFieldType, // Default to string
	}
	// Default cache TTL: 5 minutes
	sf.cacheTTL.Store(int64(5 * time.Minute))

	// Create the base field with the value type
	f := newField(
		name,
		sf.valueType,
		opts...,
	).(*field)

	sf.field = f
	// Mark this as a select field in attributes for identification
	sf.attrs["isSelectField"] = true
	return sf
}

// SelectType returns the select behavior type
func (f *selectField) SelectType() SelectType {
	return f.selectType
}

// SetSelectType sets the select behavior type
func (f *selectField) SetSelectType(t SelectType) SelectField {
	f.selectType = t
	return f
}

// Options returns the static options
func (f *selectField) Options() []SelectOption {
	return f.options
}

// SetOptions sets static options for the select
func (f *selectField) SetOptions(options []SelectOption) SelectField {
	f.options = options
	return f
}

// OptionsLoader returns the dynamic options loader function
func (f *selectField) OptionsLoader() func(ctx context.Context) []SelectOption {
	return f.optionsLoader
}

// SetOptionsLoader sets a function to dynamically load options
func (f *selectField) SetOptionsLoader(loader func(ctx context.Context) []SelectOption) SelectField {
	// Wrap loader with TTL-based caching & invalidation support.
	f.optionsLoader = func(ctx context.Context) []SelectOption {
		// Fast path under read lock
		f.cacheMu.RLock()
		if f.cachedOptions != nil {
			ttl := time.Duration(f.cacheTTL.Load())
			if ttl <= 0 || time.Since(time.Unix(0, f.cacheTime.Load())) < ttl {
				opts := f.cachedOptions
				f.cacheMu.RUnlock()
				return opts
			}
		}
		f.cacheMu.RUnlock()

		// Need to refresh cache under write lock
		f.cacheMu.Lock()
		defer f.cacheMu.Unlock()
		if f.cachedOptions == nil || (time.Duration(f.cacheTTL.Load()) > 0 && time.Since(time.Unix(0, f.cacheTime.Load())) >= time.Duration(f.cacheTTL.Load())) {
			f.cachedOptions = loader(ctx)
			f.cacheTime.Store(time.Now().UnixNano())
		}
		return f.cachedOptions
	}
	return f
}

// InvalidateOptionsCache clears the cached options forcing the next call to
// OptionsLoader to reload them.
func (f *selectField) InvalidateOptionsCache() {
	f.cacheMu.Lock()
	defer f.cacheMu.Unlock()
	f.cachedOptions = nil
	f.cacheTime.Store(0)
}

// SetOptionsCacheTTL sets the time-to-live for the cached options. A zero or
// negative duration disables TTL (cache never expires until invalidated
// manually). Returns the same SelectField for chaining.
func (f *selectField) SetOptionsCacheTTL(ttl time.Duration) SelectField {
	f.cacheTTL.Store(int64(ttl))
	return f
}

// Endpoint returns the API endpoint for searchable selects
func (f *selectField) Endpoint() string {
	return f.endpoint
}

// SetEndpoint sets the API endpoint for searchable selects
func (f *selectField) SetEndpoint(endpoint string) SelectField {
	f.endpoint = endpoint
	// Auto-set type to searchable when endpoint is provided
	if endpoint != "" {
		f.selectType = SelectTypeSearchable
	}
	return f
}

// Placeholder returns the placeholder text
func (f *selectField) Placeholder() string {
	return f.placeholder
}

// SetPlaceholder sets the placeholder text
func (f *selectField) SetPlaceholder(placeholder string) SelectField {
	f.placeholder = placeholder
	return f
}

// Multiple returns whether multiple selection is allowed
func (f *selectField) Multiple() bool {
	return f.multiple
}

// SetMultiple sets whether multiple selection is allowed
func (f *selectField) SetMultiple(multiple bool) SelectField {
	f.multiple = multiple
	// Auto-set type to combobox when multiple is enabled
	if multiple {
		f.selectType = SelectTypeCombobox
	}
	return f
}

// ValueType returns the underlying value data type
func (f *selectField) ValueType() FieldType {
	return f.valueType
}

// SetValueType sets the underlying value data type
func (f *selectField) SetValueType(t FieldType) SelectField {
	f.valueType = t
	// Recreate the base field with the new type
	newField := newField(
		f.field.name,
		t,
		// Preserve existing options
		func(field *field) {
			field.key = f.field.key
			field.readonly = f.field.readonly
			field.hidden = f.field.hidden
			field.searchable = f.field.searchable
			field.attrs = f.field.attrs
			field.initialValueFn = f.field.initialValueFn
			field.rules = f.field.rules
		},
	).(*field)
	f.field = newField
	// Ensure the select field marker is preserved
	f.attrs["isSelectField"] = true
	return f
}

// Fluent API helpers

// AsIntSelect configures the field to store integer values
func (f *selectField) AsIntSelect() SelectField {
	return f.SetValueType(IntFieldType)
}

// AsStringSelect configures the field to store string values
func (f *selectField) AsStringSelect() SelectField {
	return f.SetValueType(StringFieldType)
}

// AsBoolSelect configures the field to store boolean values
func (f *selectField) AsBoolSelect() SelectField {
	return f.SetValueType(BoolFieldType)
}

// AsSearchable configures the field as a searchable select with the given endpoint
func (f *selectField) AsSearchable(endpoint string) SelectField {
	f.selectType = SelectTypeSearchable
	f.endpoint = endpoint
	return f
}

// AsCombobox configures the field as a multi-select combobox
func (f *selectField) AsCombobox() SelectField {
	f.selectType = SelectTypeCombobox
	f.multiple = true
	return f
}

// WithStaticOptions is a convenience method to set static options
func (f *selectField) WithStaticOptions(options ...SelectOption) SelectField {
	f.selectType = SelectTypeStatic
	f.options = options
	return f
}

// WithSearchEndpoint is a convenience method to configure searchable select
func (f *selectField) WithSearchEndpoint(endpoint string) SelectField {
	return f.AsSearchable(endpoint)
}

// WithCombobox is a convenience method to configure combobox with endpoint
func (f *selectField) WithCombobox(endpoint string, multiple bool) SelectField {
	f.selectType = SelectTypeCombobox
	f.endpoint = endpoint
	f.multiple = multiple
	return f
}

// Field interface type conversion methods

func (f *selectField) AsStringField() (StringField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsIntField() (IntField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsBoolField() (BoolField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsFloatField() (FloatField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsDecimalField() (DecimalField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsDateField() (DateField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsTimeField() (TimeField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsDateTimeField() (DateTimeField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsTimestampField() (TimestampField, error) {
	return nil, ErrFieldTypeMismatch
}

func (f *selectField) AsUUIDField() (UUIDField, error) {
	return nil, ErrFieldTypeMismatch
}
