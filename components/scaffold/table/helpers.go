package table

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/components/scaffold/form"
	"github.com/iota-uz/iota-sdk/pkg/crud"
)

// --- Interfaces ---

type RowOpt func(r *tableRowImpl)
type ColumnOpt func(c *tableColumnImpl)

type TableColumn interface {
	Key() string
	Label() string
	Class() string
	Width() string
	Sortable() bool
	SortDir() SortDirection
	Editable() bool
	EditableField() crud.Field
	SortURL() string
}

type TableCell interface {
	Component(col TableColumn, editMode bool, withValue bool, fieldAttrs templ.Attributes) templ.Component
}

type tableCellImpl struct {
	component templ.Component
	value     any
}

func (c *tableCellImpl) convertValueToString(value any, fieldType crud.FieldType) string {
	if value == nil {
		return ""
	}

	switch fieldType {
	case crud.IntFieldType:
		switch v := value.(type) {
		case int:
			return strconv.Itoa(v)
		case int64:
			return strconv.FormatInt(v, 10)
		case int32:
			return strconv.FormatInt(int64(v), 10)
		}
	case crud.BoolFieldType:
		if v, ok := value.(bool); ok {
			return strconv.FormatBool(v)
		}
	case crud.FloatFieldType:
		switch v := value.(type) {
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64)
		case float32:
			return strconv.FormatFloat(float64(v), 'f', -1, 32)
		}
	case crud.StringFieldType, crud.DecimalFieldType, crud.UUIDFieldType:
		return fmt.Sprintf("%v", value)
	case crud.JSONFieldType:
		// For JSON fields, return as string
		return fmt.Sprintf("%v", value)
	case crud.DateFieldType, crud.TimeFieldType, crud.DateTimeFieldType, crud.TimestampFieldType:
		// For date/time types, format as string
		if t, ok := value.(time.Time); ok {
			return t.Format(time.RFC3339)
		}
		return fmt.Sprintf("%v", value)
	}

	// Default: convert to string
	return fmt.Sprintf("%v", value)
}

func (c *tableCellImpl) handleSelectField(ctx context.Context, selectField crud.SelectField, currentValue any, fieldAttrs templ.Attributes) templ.Component {
	// Convert current value to string for comparison
	var valueStr string
	if currentValue != nil {
		valueStr = c.convertValueToString(currentValue, selectField.ValueType())
	}

	switch selectField.SelectType() {
	case crud.SelectTypeStatic:
		// Get options
		options := selectField.Options()
		if options == nil && selectField.OptionsLoader() != nil {
			options = selectField.OptionsLoader()(ctx)
		}

		// Convert to form options
		formOptions := make([]form.Option, len(options))
		for i, opt := range options {
			// Convert value to string for HTML rendering
			var optValueStr string
			switch v := opt.Value.(type) {
			case string:
				optValueStr = v
			case int:
				optValueStr = strconv.Itoa(v)
			case int64:
				optValueStr = strconv.FormatInt(v, 10)
			case bool:
				optValueStr = strconv.FormatBool(v)
			case float64:
				optValueStr = strconv.FormatFloat(v, 'f', -1, 64)
			case uuid.UUID:
				optValueStr = v.String()
			default:
				optValueStr = fmt.Sprintf("%v", v)
			}

			formOptions[i] = form.Option{
				Value: optValueStr,
				Label: opt.Label,
			}
		}

		builder := form.Select(selectField.Name(), "").
			Options(formOptions)

		if selectField.Placeholder() != "" {
			fieldAttrs["data-placeholder"] = selectField.Placeholder()
			// Set placeholder through attributes since the builder doesn't have a method
			builder = builder.Attrs(fieldAttrs)
		}

		if selectField.Readonly() {
			fieldAttrs["disabled"] = true
			builder = builder.Attrs(fieldAttrs)
		}

		if len(selectField.Rules()) > 0 {
			builder = builder.Required()
		}

		if valueStr != "" {
			builder = builder.Default(valueStr)
		}

		return builder.Build().Component()

	case crud.SelectTypeSearchable:
		builder := form.SearchSelect().
			Key(selectField.Name()).
			Label("").
			Endpoint(selectField.Endpoint()).
			Placeholder(selectField.Placeholder())

		if selectField.Readonly() {
			fieldAttrs["disabled"] = true
			builder = builder.Attrs(fieldAttrs)
		}

		if len(selectField.Rules()) > 0 {
			builder = builder.WithRequired(true)
		}

		if valueStr != "" {
			builder = builder.WithValue(valueStr)
		}

		return builder.Build().Component()

	case crud.SelectTypeCombobox:
		builder := form.Combobox().
			Key(selectField.Name()).
			Label("").
			Endpoint(selectField.Endpoint()).
			Placeholder(selectField.Placeholder()).
			Multiple(selectField.Multiple())

		if selectField.Readonly() {
			fieldAttrs["disabled"] = true
			builder = builder.Attrs(fieldAttrs)
		}

		if len(selectField.Rules()) > 0 {
			builder = builder.WithRequired(true)
		}

		if valueStr != "" {
			builder = builder.WithValue(valueStr)
		}

		return builder.Build().Component()

	default:
		// Fallback to regular select
		return form.Select(selectField.Name(), "").Build().Component()
	}
}

func (c *tableCellImpl) Component(col TableColumn, editMode bool, withValue bool, fieldAttrs templ.Attributes) templ.Component {
	field := col.EditableField()
	if col.Editable() && field != nil && editMode {
		if field.Hidden() {
			return nil
		}
		if field.Key() && field.Readonly() {
			return nil
		}

		ctx := context.TODO()
		var currentValue any
		if withValue {
			if c.value != nil && !(c.value == nil || reflect.ValueOf(c.value).IsZero()) {
				currentValue = c.value
			} else if field.InitialValue(ctx) != nil {
				currentValue = field.InitialValue(ctx)
			}
		}

		switch field.Type() {
		case crud.StringFieldType:
			// Check if this is actually a select field
			if selectField, ok := field.(crud.SelectField); ok {
				return c.handleSelectField(ctx, selectField, currentValue, fieldAttrs)
			}

			sf, err := field.AsStringField()
			if err != nil {
				return nil
			}

			builder := form.Text(field.Name(), "")

			if sf.MaxLen() > 0 {
				builder = builder.MaxLen(sf.MaxLen())
			}
			if sf.MinLen() > 0 {
				builder = builder.MinLen(sf.MinLen())
			}

			if sf.Multiline() {
				textareaBuilder := form.Textarea(field.Name(), "")
				if sf.MaxLen() > 0 {
					textareaBuilder = textareaBuilder.MaxLen(sf.MaxLen())
				}
				if sf.MinLen() > 0 {
					textareaBuilder = textareaBuilder.MinLen(sf.MinLen())
				}

				if field.Readonly() {
					textareaBuilder = textareaBuilder.Attrs(templ.Attributes{"disabled": true})
				}

				if len(field.Rules()) > 0 {
					textareaBuilder = textareaBuilder.Required()
				}

				if currentValue != nil {
					if strVal, ok := currentValue.(string); ok {
						textareaBuilder = textareaBuilder.Default(strVal)
					}
				}

				return textareaBuilder.Build().Component()
			}

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				if strVal, ok := currentValue.(string); ok {
					builder = builder.Default(strVal)
				}
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.IntFieldType:
			// Check if this is actually a select field with int values
			if selectField, ok := field.(crud.SelectField); ok {
				return c.handleSelectField(ctx, selectField, currentValue, fieldAttrs)
			}

			intField, err := field.AsIntField()
			if err != nil {
				return nil
			}

			builder := form.NewNumberField(field.Name(), "")

			if intField.Min() != 0 {
				builder = builder.Min(float64(intField.Min()))
			}
			if intField.Max() != 0 {
				builder = builder.Max(float64(intField.Max()))
			}

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				switch v := currentValue.(type) {
				case int:
					builder = builder.Default(float64(v))
				case int64:
					builder = builder.Default(float64(v))
				case float64:
					builder = builder.Default(v)
				}
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.BoolFieldType:
			// Check if this is actually a select field with bool values
			if selectField, ok := field.(crud.SelectField); ok {
				return c.handleSelectField(ctx, selectField, currentValue, fieldAttrs)
			}

			builder := form.Checkbox(field.Name(), "")

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				if boolVal, ok := currentValue.(bool); ok {
					builder = builder.Default(boolVal)
				}
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.FloatFieldType:
			floatField, err := field.AsFloatField()
			if err != nil {
				return nil
			}

			builder := form.NewNumberField(field.Name(), "")

			if floatField.Min() != 0 {
				builder = builder.Min(floatField.Min())
			}
			if floatField.Max() != 0 {
				builder = builder.Max(floatField.Max())
			}

			attrs := templ.Attributes{}
			if floatField.Step() != 0 {
				attrs["step"] = fmt.Sprintf("%f", floatField.Step())
			} else {
				attrs["step"] = "any"
			}

			if field.Readonly() {
				attrs["disabled"] = true
			}

			maps.Copy(attrs, fieldAttrs)

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				if floatVal, ok := currentValue.(float64); ok {
					builder = builder.Default(floatVal)
				}
			}

			return builder.Attrs(attrs).Build().Component()

		case crud.DateFieldType:
			builder := form.Date(field.Name(), "")

			dateField, err := field.AsDateField()
			if err == nil {
				if !dateField.MinDate().IsZero() {
					builder = builder.Min(dateField.MinDate())
				}
				if !dateField.MaxDate().IsZero() {
					builder = builder.Max(dateField.MaxDate())
				}
			}

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				if timeVal, ok := currentValue.(time.Time); ok && !timeVal.IsZero() {
					builder = builder.Default(timeVal)
				}
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.TimeFieldType:
			builder := form.Time(field.Name(), "")

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				if timeVal, ok := currentValue.(time.Time); ok && !timeVal.IsZero() {
					builder = builder.Default(timeVal.Format("15:04"))
				}
			}
			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.DateTimeFieldType:
			builder := form.DateTime(field.Name(), "")

			dateTimeField, err := field.AsDateTimeField()
			if err == nil {
				if !dateTimeField.MinDateTime().IsZero() {
					builder = builder.Min(dateTimeField.MinDateTime())
				}
				if !dateTimeField.MaxDateTime().IsZero() {
					builder = builder.Max(dateTimeField.MaxDateTime())
				}
			}

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				if timeVal, ok := currentValue.(time.Time); ok && !timeVal.IsZero() {
					builder = builder.Default(timeVal)
				}
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.UUIDFieldType:
			// Check if this is actually a select field
			if selectField, ok := field.(crud.SelectField); ok {
				return c.handleSelectField(ctx, selectField, currentValue, fieldAttrs)
			}

			builder := form.Text(field.Name(), "")

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				switch v := currentValue.(type) {
				case string:
					builder = builder.Default(v)
				case uuid.UUID:
					builder = builder.Default(v.String())
				}
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.TimestampFieldType:
			// Timestamp fields are treated like datetime fields
			builder := form.DateTime(field.Name(), "")

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			if currentValue != nil {
				switch v := currentValue.(type) {
				case time.Time:
					builder = builder.Default(v)
				}
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		case crud.DecimalFieldType:
			decimalField, err := field.AsDecimalField()
			if err != nil {
				return nil
			}

			builder := form.NewNumberField(field.Name(), "")

			if decimalField.Min() != "" {
				if minVal, err := strconv.ParseFloat(decimalField.Min(), 64); err == nil {
					builder = builder.Min(minVal)
				}
			}
			if decimalField.Max() != "" {
				if maxVal, err := strconv.ParseFloat(decimalField.Max(), 64); err == nil {
					builder = builder.Max(maxVal)
				}
			}

			attrs := templ.Attributes{}
			if decimalField.Scale() > 0 {
				step := 1.0
				for range decimalField.Scale() {
					step /= 10
				}
				attrs["step"] = fmt.Sprintf("%f", step)
			} else {
				attrs["step"] = "any"
			}

			if field.Readonly() {
				attrs["disabled"] = true
			}

			maps.Copy(attrs, fieldAttrs)
			// Set decimal value if present
			// if value != nil && !value.IsZero() {
			// 	// Use AsDecimal to handle all possible decimal value types
			// 	if decimalStr, err := value.AsDecimal(); err == nil {
			// 		// Validate it's a proper number format and set the value directly in attrs
			// 		if _, err := strconv.ParseFloat(decimalStr, 64); err == nil {
			// 			attrs["value"] = decimalStr
			// 		}
			// 	}
			// }

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			} else if currentValue != nil {
				// Handle direct decimal values (fallback for when value is nil)
				if strVal, ok := currentValue.(string); ok {
					if floatVal, err := strconv.ParseFloat(strVal, 64); err == nil {
						builder = builder.Default(floatVal)
					}
				}
			}

			return builder.Attrs(attrs).Build().Component()

		case crud.JSONFieldType:
			// Handle JSON field as a textarea for editing
			builder := form.Textarea(field.Name(), "")

			if field.Readonly() {
				fieldAttrs["disabled"] = true
			}

			if len(field.Rules()) > 0 {
				builder = builder.Required()
			}

			// Convert JSON value to formatted string for editing
			if currentValue != nil {
				var jsonStr string
				if str, ok := currentValue.(string); ok {
					jsonStr = str
				} else {
					// Pretty print JSON for better editing experience
					if jsonBytes, err := json.MarshalIndent(currentValue, "", "  "); err == nil {
						jsonStr = string(jsonBytes)
					} else {
						jsonStr = fmt.Sprintf("%v", currentValue)
					}
				}
				builder = builder.Default(jsonStr)
			}

			return builder.Attrs(fieldAttrs).Build().Component()

		default:
			builder := form.Text(field.Name(), field.Name())
			if currentValue != nil {
				builder = builder.Default(fmt.Sprintf("%v", currentValue))
			}
			return builder.Build().Component()
		}
	}
	return c.component
}

type TableRow interface {
	Cells() []TableCell
	Attrs() templ.Attributes
	ApplyOpts(opts ...RowOpt) TableRow
}

// --- Private Implementations ---

type tableColumnImpl struct {
	key           string
	label         string
	class         string
	width         string
	sortable      bool
	sortDir       SortDirection
	sortURL       string
	editable      bool
	editableField crud.Field
}

func (c *tableColumnImpl) Key() string               { return c.key }
func (c *tableColumnImpl) Label() string             { return c.label }
func (c *tableColumnImpl) Class() string             { return c.class }
func (c *tableColumnImpl) Width() string             { return c.width }
func (c *tableColumnImpl) Sortable() bool            { return c.sortable }
func (c *tableColumnImpl) SortDir() SortDirection    { return c.sortDir }
func (c *tableColumnImpl) SortURL() string           { return c.sortURL }
func (c *tableColumnImpl) Editable() bool            { return c.editable }
func (c *tableColumnImpl) EditableField() crud.Field { return c.editableField }

type tableRowImpl struct {
	cells []TableCell
	attrs templ.Attributes
}

func (r *tableRowImpl) Cells() []TableCell {
	return r.cells
}

func (r *tableRowImpl) Attrs() templ.Attributes {
	return r.attrs
}

func (r *tableRowImpl) ApplyOpts(opts ...RowOpt) TableRow {
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// --- Row Options ---

func WithDrawer(fetchURL string) RowOpt {
	return func(r *tableRowImpl) {
		r.attrs["class"] = r.attrs["class"].(string) + " cursor-pointer hover:bg-surface-500 transition-colors"
		r.attrs["hx-get"] = fetchURL
		r.attrs["hx-target"] = "#view-drawer"
		r.attrs["hx-swap"] = "innerHTML"
	}
}

// --- Column Options ---

func WithClass(classes string) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.class = classes
	}
}

func WithSortableState(sortable bool) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.sortable = sortable
	}
}

// WithSortable enables sorting for a column
func WithSortable() ColumnOpt {
	return func(c *tableColumnImpl) {
		c.sortable = true
	}
}

func WithSortDir(sortDir SortDirection) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.sortDir = sortDir
	}
}

func WithSortURL(sortURL string) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.sortURL = sortURL
	}
}

func WithEditableColumn(field crud.Field) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.editable = true
		c.editableField = field
	}
}

// --- Table Configuration ---

type TableConfigOpt func(c *TableConfig)

func WithEditable(config TableEditableConfig) TableConfigOpt {
	return func(c *TableConfig) {
		c.Editable = config
	}
}

func WithInfiniteScroll(hasMore bool, page, perPage int) TableConfigOpt {
	return func(c *TableConfig) {
		c.Infinite.HasMore = hasMore
		c.Infinite.Page = page
		c.Infinite.PerPage = perPage
	}
}

type InfiniteScrollConfig struct {
	HasMore bool
	Page    int
	PerPage int
}

type TableEditableConfig struct {
	Key               string
	Enabled           bool
	WithoutDelete     bool
	WithoutCreate     bool
	CreateLabel       string
	ActionColumnLabel string
}

type TableConfig struct {
	Title      string
	DataURL    string
	Filters    []templ.Component
	Actions    []templ.Component // Actions like Create button
	Columns    []TableColumn
	Rows       []TableRow
	Infinite   *InfiniteScrollConfig
	SideFilter templ.Component
	Editable   TableEditableConfig

	// Sorting configuration
	CurrentSort      string // Current sort field
	CurrentSortOrder string // Current sort order (asc/desc)

	// Optional: reference to definition for advanced usage
	definition *TableDefinition
}

func NewTableConfig(title, dataURL string, opts ...TableConfigOpt) *TableConfig {
	t := &TableConfig{
		Title:    title,
		DataURL:  dataURL,
		Infinite: &InfiniteScrollConfig{},
		Columns:  []TableColumn{},
		Filters:  []templ.Component{},
		Actions:  []templ.Component{},
		Rows:     []TableRow{},
	}
	for _, o := range opts {
		o(t)
	}
	return t
}

func Column(key, label string, opts ...ColumnOpt) TableColumn {
	col := &tableColumnImpl{
		key:      key,
		label:    label,
		sortable: false, // Default to not sortable
	}
	for _, opt := range opts {
		opt(col)
	}
	return col
}

func Row(cells ...TableCell) TableRow {
	return &tableRowImpl{
		cells: cells,
		attrs: templ.Attributes{
			"class": "hide-on-load",
		},
	}
}

func Cell(component templ.Component, value any) TableCell {
	return &tableCellImpl{
		component: component,
		value:     value,
	}
}

func (c *TableConfig) AddCols(cols ...TableColumn) *TableConfig {
	c.Columns = append(c.Columns, cols...)
	return c
}

// UpdateColumnsWithSorting updates column sorting information based on current request
func (c *TableConfig) UpdateColumnsWithSorting(r *http.Request) *TableConfig {
	// Get current sort parameters from request
	c.CurrentSort = UseSortQuery(r)
	c.CurrentSortOrder = UseOrderQuery(r)

	// Update each sortable column with proper sort URL and direction
	for i, col := range c.Columns {
		if colImpl, ok := col.(*tableColumnImpl); ok && colImpl.sortable {
			// Generate sort URL for this column
			colImpl.sortURL = GenerateSortURLWithParams(
				c.DataURL,
				colImpl.key,
				c.CurrentSort,
				c.CurrentSortOrder,
				r.URL.Query(),
			)

			// Set sort direction if this is the current sort field
			colImpl.sortDir = GetSortDirection(colImpl.key, c.CurrentSort, c.CurrentSortOrder)

			// Update the column in the slice
			c.Columns[i] = colImpl
		}
	}

	return c
}

func (c *TableConfig) AddFilters(filters ...templ.Component) *TableConfig {
	c.Filters = append(c.Filters, filters...)
	return c
}

func (c *TableConfig) AddRows(rows ...TableRow) *TableConfig {
	c.Rows = append(c.Rows, rows...)
	return c
}

func (c *TableConfig) SetSideFilter(filter templ.Component) *TableConfig {
	c.SideFilter = filter
	return c
}

func (c *TableConfig) AddActions(actions ...templ.Component) *TableConfig {
	c.Actions = append(c.Actions, actions...)
	return c
}

// --- Query Parameter Utilities ---

// UseSearchQuery gets the "Search" query parameter from the request
func UseSearchQuery(r *http.Request) string {
	return r.URL.Query().Get(QueryParamSearch)
}

// UsePageQuery gets the "page" query parameter from the request and converts it to int
func UsePageQuery(r *http.Request) int {
	pageStr := r.URL.Query().Get(QueryParamPage)
	if pageStr == "" {
		return 1 // default to page 1
	}
	page, err := strconv.Atoi(pageStr)
	if err != nil || page < 1 {
		return 1
	}
	return page
}

// UseLimitQuery gets the "limit" query parameter from the request and converts it to int
func UseLimitQuery(r *http.Request) int {
	limitStr := r.URL.Query().Get(QueryParamLimit)
	if limitStr == "" {
		return 20 // default to 20 items per page
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		return 20
	}
	if limit > 100 {
		return 100 // cap at maximum
	}
	return limit
}

// UseSortQuery gets the "sort" query parameter from the request
func UseSortQuery(r *http.Request) string {
	return r.URL.Query().Get(QueryParamSort)
}

// UseOrderQuery gets the "order" query parameter from the request (asc/desc)
func UseOrderQuery(r *http.Request) string {
	order := r.URL.Query().Get(QueryParamOrder)
	// Only return a value if explicitly set, otherwise return empty string
	if order == SortDirectionAsc.String() || order == SortDirectionDesc.String() {
		return order
	}
	return ""
}

// GenerateSortURL generates a sort URL for a column based on current sort state
func GenerateSortURL(baseURL, fieldKey, currentSortField, currentSortOrder string) string {
	return GenerateSortURLWithParams(baseURL, fieldKey, currentSortField, currentSortOrder, nil)
}

// GenerateSortURLWithParams generates a sort URL for a column with additional query parameters
func GenerateSortURLWithParams(baseURL, fieldKey, currentSortField, currentSortOrder string, existingParams url.Values) string {
	params := url.Values{}

	// Copy existing parameters if provided
	for k, v := range existingParams {
		params[k] = v
	}

	// Debug: log input parameters
	// fmt.Printf("GenerateSortURL: field=%s, currentField=%s, currentOrder=%s\n", fieldKey, currentSortField, currentSortOrder)

	// If clicking on the same field, cycle through: none -> asc -> desc -> none
	if fieldKey == currentSortField {
		switch currentSortOrder {
		case SortDirectionAsc.String():
			params.Set(QueryParamSort, fieldKey)
			params.Set(QueryParamOrder, SortDirectionDesc.String())
		case SortDirectionDesc.String():
			// Reset to no sorting (remove sort/order params)
			params.Del(QueryParamSort)
			params.Del(QueryParamOrder)
		default:
			// Empty or no order -> start with asc
			params.Set(QueryParamSort, fieldKey)
			params.Set(QueryParamOrder, SortDirectionAsc.String())
		}
	} else {
		// Different field, start with ascending
		params.Set(QueryParamSort, fieldKey)
		params.Set(QueryParamOrder, SortDirectionAsc.String())
	}

	if len(params) == 0 {
		return baseURL
	}

	return baseURL + "?" + params.Encode()
}

// GetSortDirection returns the sort direction for a field
func GetSortDirection(fieldKey, currentSortField, currentSortOrder string) SortDirection {
	if fieldKey == currentSortField {
		return ParseSortDirection(currentSortOrder)
	}
	return SortDirectionNone
}

// --- New methods for separation of concerns ---

// ToDefinition extracts the table definition from config
func (c *TableConfig) ToDefinition() TableDefinition {
	if c.definition != nil {
		return *c.definition
	}

	// Build definition from current config
	builder := NewTableDefinition(c.Title, c.DataURL).
		WithColumns(c.Columns...).
		WithFilters(c.Filters...).
		WithActions(c.Actions...).
		WithSideFilter(c.SideFilter)

	if c.Infinite != nil {
		builder.WithInfiniteScroll(true)
	}

	return builder.Build()
}

// ToData extracts the table data from config
func (c *TableConfig) ToData() *TableData {
	data := NewTableData().WithRows(c.Rows...)

	if c.Infinite != nil {
		// Calculate total from hasMore flag
		// This is approximate but works for infinite scroll
		total := int64(c.Infinite.Page * c.Infinite.PerPage)
		if c.Infinite.HasMore {
			total++ // Indicate there's at least one more item
		}
		data.WithPagination(c.Infinite.Page, c.Infinite.PerPage, total)
	}

	return data
}

// FromDefinitionAndData creates a TableConfig from definition and data
func FromDefinitionAndData(def TableDefinition, data *TableData) *TableConfig {
	cfg := &TableConfig{
		Title:      def.Title(),
		DataURL:    def.DataURL(),
		Columns:    def.Columns(),
		Filters:    def.Filters(),
		Actions:    def.Actions(),
		SideFilter: def.SideFilter(),
		Rows:       data.Rows(),
		definition: &def,
	}

	if def.EnableInfiniteScroll() {
		pagination := data.Pagination()
		cfg.Infinite = &InfiniteScrollConfig{
			HasMore: pagination.HasMore,
			Page:    pagination.Page,
			PerPage: pagination.PerPage,
		}
	}

	return cfg
}
