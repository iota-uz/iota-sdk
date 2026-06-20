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
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/components/scaffold/form"
	"github.com/iota-uz/iota-sdk/pkg/crud"
)

// --- Interfaces ---

type RowOpt func(r *tableRowImpl)
type ColumnOpt func(c *tableColumnImpl)
type CellOpt func(c *tableCellImpl)

type TableColumn interface {
	Key() string
	Label() string
	Class() string
	Width() string
	Sortable() bool
	SortDir() SortDirection
	Editable() bool
	EditableField() crud.Field
	RendererRegistry() *crud.RendererRegistry
	SortURL() string
	StickyPos() StickyPosition
	AddonBottom() *Addon
	DefaultHidden() bool
	// Truncate reports whether cell content for this column is clipped to a
	// single line (with an ellipsis + overflow tooltip).
	Truncate() bool
	// TruncateWidth is the max content width in px when Truncate is true.
	TruncateWidth() int
	// Priority controls responsive auto-hiding. Lower = more important;
	// unset/0/1 always visible, 2 hides below tablet, >=3 hides below desktop.
	Priority() int
}

// DefaultTruncateWidth is the column max content width (px) applied when
// WithTruncate / WithTruncateDefault is used without an explicit width.
const DefaultTruncateWidth = 240

// priorityCellClass returns the responsive auto-hide utility for a body cell at
// the given column priority. Mirrors the header priorityClass in base/table.
// Lower priority = more important; 0/1 always visible.
func priorityCellClass(p int) string {
	switch {
	case p >= 3:
		return "max-lg:hidden"
	case p == 2:
		return "max-md:hidden"
	default:
		return ""
	}
}

type TableCell interface {
	Component(col TableColumn, editMode bool, withValue bool, fieldAttrs templ.Attributes) templ.Component
	Classes() templ.CSSClasses
	Attrs() templ.Attributes
}

type tableCellImpl struct {
	component templ.Component
	value     any
	classes   templ.CSSClasses
	attrs     templ.Attributes
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
	case crud.EntityFieldType:
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
	maps.Copy(fieldAttrs, selectField.Attrs())
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
		}

		if selectField.Readonly() {
			fieldAttrs["disabled"] = true
		}

		if len(selectField.Rules()) > 0 {
			builder = builder.Required()
		}
		builder = builder.Attrs(fieldAttrs)
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
		}

		if len(selectField.Rules()) > 0 {
			builder = builder.WithRequired(true)
		}

		if valueStr != "" {
			builder = builder.WithValue(valueStr)
		}

		return builder.Attrs(fieldAttrs).Build().Component()

	case crud.SelectTypeCombobox:
		builder := form.Combobox().
			Key(selectField.Name()).
			Label("").
			Endpoint(selectField.Endpoint()).
			Placeholder(selectField.Placeholder()).
			Multiple(selectField.Multiple())

		if selectField.Readonly() {
			fieldAttrs["disabled"] = true
		}

		if len(selectField.Rules()) > 0 {
			builder = builder.WithRequired(true)
		}

		if valueStr != "" {
			builder = builder.WithValue(valueStr)
		}
		return builder.Attrs(fieldAttrs).Build().Component()

	default:
		// Fallback to regular select
		return form.Select(selectField.Name(), "").Build().Component()
	}
}

func (c *tableCellImpl) Classes() templ.CSSClasses {
	return c.classes
}

func (c *tableCellImpl) Attrs() templ.Attributes {
	return c.attrs
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
			if c.value != nil && !reflect.ValueOf(c.value).IsZero() {
				currentValue = c.value
			} else if field.InitialValue(ctx) != nil {
				currentValue = field.InitialValue(ctx)
			}
		}

		if rendererType := field.RendererType(); rendererType != "" && col.RendererRegistry() != nil {
			if renderer, exists := col.RendererRegistry().Get(rendererType); exists {
				// Merge dynamic fieldAttrs into the field's attrs so the renderer can access them
				maps.Copy(field.Attrs(), fieldAttrs)
				return renderer.RenderFormControl(ctx, field, field.Value(currentValue))
			}
		}

		maps.Copy(fieldAttrs, field.Attrs())
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

			attrs := fieldAttrs
			if floatField.Step() != 0 {
				attrs["step"] = fmt.Sprintf("%f", floatField.Step())
			} else {
				attrs["step"] = "any"
			}

			if field.Readonly() {
				attrs["disabled"] = true
			}

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

			attrs := fieldAttrs
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

		case crud.EntityFieldType:
			// EntityField is readonly and hidden, should not appear in forms
			return nil

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
	key               string
	label             string
	class             string
	width             string
	sortable          bool
	sortDir           SortDirection
	sortURL           string
	editable          bool
	editableField     crud.Field
	rendererRegistery *crud.RendererRegistry
	stickyPos         StickyPosition
	addonBottom       *Addon
	defaultHidden     bool
	truncate          bool
	truncateWidth     int
	priority          int
}

func (c *tableColumnImpl) Key() string                              { return c.key }
func (c *tableColumnImpl) Label() string                            { return c.label }
func (c *tableColumnImpl) Class() string                            { return c.class }
func (c *tableColumnImpl) Width() string                            { return c.width }
func (c *tableColumnImpl) Sortable() bool                           { return c.sortable }
func (c *tableColumnImpl) SortDir() SortDirection                   { return c.sortDir }
func (c *tableColumnImpl) SortURL() string                          { return c.sortURL }
func (c *tableColumnImpl) Editable() bool                           { return c.editable }
func (c *tableColumnImpl) StickyPos() StickyPosition                { return c.stickyPos }
func (c *tableColumnImpl) AddonBottom() *Addon                      { return c.addonBottom }
func (c *tableColumnImpl) DefaultHidden() bool                      { return c.defaultHidden }
func (c *tableColumnImpl) Truncate() bool                           { return c.truncate }
func (c *tableColumnImpl) TruncateWidth() int                       { return c.truncateWidth }
func (c *tableColumnImpl) Priority() int                            { return c.priority }
func (c *tableColumnImpl) EditableField() crud.Field                { return c.editableField }
func (c *tableColumnImpl) RendererRegistry() *crud.RendererRegistry { return c.rendererRegistery }

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
		// Comma-ok guard: Row() initializes class as a string, but defend against
		// a nil/non-string class set by a prior opt instead of panicking.
		existing, _ := r.attrs["class"].(string)
		r.attrs["class"] = existing +
			" cursor-pointer hover:bg-surface-500 transition-colors" +
			" focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-primary"
		r.attrs["hx-get"] = fetchURL
		r.attrs["hx-target"] = "#view-drawer"
		r.attrs["hx-swap"] = "innerHTML"
		// Keyboard navigation hooks (see tableKeyboardNav in alpine.js):
		// the row becomes focusable and self-identifies as a drawer trigger so
		// arrow-key navigation and Enter-to-open work without scroll hijacking.
		r.attrs["tabindex"] = "0"
		r.attrs["data-row-drawer"] = "true"
		r.attrs["role"] = "button"
		r.attrs["aria-haspopup"] = "dialog"
	}
}

func WithRowAttrs(attrs templ.Attributes) RowOpt {
	return func(r *tableRowImpl) {
		maps.Copy(r.attrs, attrs)
	}
}

// --- Column Options ---

func WithRendererRegistry(registry *crud.RendererRegistry) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.rendererRegistery = registry
	}
}

func WithAddonBottom(addonBottom *Addon) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.addonBottom = addonBottom
	}
}

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

func WithSticky(pos StickyPosition) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.stickyPos = pos
	}
}

// WithDefaultHidden hides a column by default. Users can show it via table settings.
func WithDefaultHidden() ColumnOpt {
	return func(c *tableColumnImpl) {
		c.defaultHidden = true
	}
}

// WithTruncate clips this column's cell content to a single line within
// maxWidthPx (default DefaultTruncateWidth) and shows a tooltip with the full
// text only when the content actually overflows.
//
// Text-only scope: truncate (overflow:hidden) plus the overflow tooltip (which
// reads cell textContent) only behave correctly for plain inline text. Do not
// use it on block/flex component cells (badges, chips, action buttons) — those
// get visually clipped and surface meaningless or empty tooltips. Use it on
// text columns only.
func WithTruncate(maxWidthPx ...int) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.truncate = true
		c.truncateWidth = DefaultTruncateWidth
		if len(maxWidthPx) > 0 && maxWidthPx[0] > 0 {
			c.truncateWidth = maxWidthPx[0]
		}
	}
}

// WithPriority sets a column's responsive priority. Lower = more important.
// unset/0/1 → always visible; 2 → hidden below tablet (max-md); >=3 → hidden
// below desktop (max-lg). Sticky columns are exempt. Users may override the
// auto-hiding via table settings.
func WithPriority(n int) ColumnOpt {
	return func(c *tableColumnImpl) {
		c.priority = n
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

func WithID(id string) TableConfigOpt {
	return func(c *TableConfig) {
		c.ID = id
	}
}

func WithConfigurable(configurable bool) TableConfigOpt {
	return func(c *TableConfig) {
		c.Configurable = configurable
	}
}

func WithoutSearch() TableConfigOpt {
	return func(c *TableConfig) {
		c.WithoutSearch = true
	}
}

func WithEditable(config TableEditableConfig) TableConfigOpt {
	return func(c *TableConfig) {
		c.Editable = config
	}
}

func WithHead(config TableHeadConfig) TableConfigOpt {
	return func(c *TableConfig) {
		c.Head = config
	}
}

func WithScrollbarPosition(pos ScrollbarPosition) TableConfigOpt {
	return func(c *TableConfig) {
		c.ScrollbarPosition = pos
	}
}

func WithInfiniteScroll(hasMore bool, page, perPage int) TableConfigOpt {
	return func(c *TableConfig) {
		c.Infinite.HasMore = hasMore
		c.Infinite.Page = page
		c.Infinite.PerPage = perPage
	}
}

func WithSearchPlaceholder(placeholder string) TableConfigOpt {
	return func(c *TableConfig) {
		c.SearchPlaceholder = placeholder
	}
}

func WithFillerRows(enabled bool, rowHeight ...int) TableConfigOpt {
	return func(c *TableConfig) {
		c.FillerRows = enabled
		if len(rowHeight) > 0 {
			c.FillerRowHeight = rowHeight[0]
		}
	}
}

func WithNoWrap(nowrap bool) TableConfigOpt {
	return func(c *TableConfig) {
		c.NoWrap = nowrap
	}
}

// WithTruncateDefault enables truncation for ALL non-sticky columns that don't
// already declare their own WithTruncate. Sticky columns are skipped because
// they typically hold action buttons/dropdowns that overflow:hidden would clip.
// maxWidthPx defaults to DefaultTruncateWidth.
//
// Text-only scope: like WithTruncate, truncate + the overflow tooltip (reads
// cell textContent) only behave correctly for plain inline text, not for
// block/flex component cells (badges, chips, action buttons). Because this
// applies to every non-sticky column, callers that render component cells in
// non-sticky positions should opt those columns out / set per-column behavior
// accordingly (e.g. mark them sticky or render plain text).
func WithTruncateDefault(maxWidthPx ...int) TableConfigOpt {
	return func(c *TableConfig) {
		c.TruncateDefault = true
		c.TruncateDefaultWidth = DefaultTruncateWidth
		if len(maxWidthPx) > 0 && maxWidthPx[0] > 0 {
			c.TruncateDefaultWidth = maxWidthPx[0]
		}
	}
}

func WithScrollbarGutter(enabled bool) TableConfigOpt {
	return func(c *TableConfig) {
		c.ScrollbarGutter = enabled
	}
}

func WithSearchClearable(clearable bool) TableConfigOpt {
	return func(c *TableConfig) {
		c.SearchClearable = clearable
	}
}

func WithHxTrigger(trigger string) TableConfigOpt {
	return func(c *TableConfig) {
		c.HxTrigger = trigger
	}
}

func WithFullHeight(enabled bool) TableConfigOpt {
	return func(c *TableConfig) {
		c.FullHeight = enabled
	}
}

func WithContentID(id string) TableConfigOpt {
	return func(c *TableConfig) {
		c.ContentID = id
	}
}

func WithHxTarget(target string) TableConfigOpt {
	return func(c *TableConfig) {
		c.HxTarget = target
	}
}

func WithHxSwap(swap string) TableConfigOpt {
	return func(c *TableConfig) {
		c.HxSwap = swap
	}
}

func WithHxIndicator(indicator string) TableConfigOpt {
	return func(c *TableConfig) {
		c.HxIndicator = indicator
	}
}

func WithSearchValue(value string) TableConfigOpt {
	return func(c *TableConfig) {
		c.SearchValue = value
	}
}

func WithSearchParamName(name string) TableConfigOpt {
	return func(c *TableConfig) {
		c.SearchParamName = strings.TrimSpace(name)
	}
}

// WithDeferredPanels registers one or more deferred panels (summary/aggregate
// regions) rendered inside the table form but outside the swap target, so they
// persist across row/content swaps and reload on filter/search changes.
func WithDeferredPanels(panels ...DeferredPanel) TableConfigOpt {
	return func(c *TableConfig) {
		c.DeferredPanels = append(c.DeferredPanels, panels...)
	}
}

// WithFooter sets the sticky footer row cells (one per column, index-matched to
// Columns). See TableConfig.Footer.
func WithFooter(cells ...TableCell) TableConfigOpt {
	return func(c *TableConfig) {
		c.Footer = cells
	}
}

// resolvedTruncate returns the effective truncation (on, widthPx) for a column,
// folding in the table-wide WithTruncateDefault. Sticky columns never truncate.
func (c *TableConfig) resolvedTruncate(col TableColumn) (bool, int) {
	if !col.StickyPos().Unknown() && !col.StickyPos().None() {
		return false, 0
	}
	if col.Truncate() {
		w := col.TruncateWidth()
		if w <= 0 {
			w = DefaultTruncateWidth
		}
		return true, w
	}
	if c.TruncateDefault {
		w := c.TruncateDefaultWidth
		if w <= 0 {
			w = DefaultTruncateWidth
		}
		return true, w
	}
	return false, 0
}

// resolvedPriority returns the effective responsive priority for a column.
// Sticky columns are exempt (always 0).
func (c *TableConfig) resolvedPriority(col TableColumn) int {
	if !col.StickyPos().Unknown() && !col.StickyPos().None() {
		return 0
	}
	return col.Priority()
}

func (c *TableConfig) ResolvedHxTarget() string {
	if c.HxTarget != "" {
		return c.HxTarget
	}
	if c.ContentID != "" {
		return "#" + c.ContentID
	}
	return "#table-body"
}

func (c *TableConfig) ResolvedHxSwap() string {
	if c.HxSwap != "" {
		return c.HxSwap
	}
	return "innerHTML"
}

func (c *TableConfig) ResolvedHxIndicator() string {
	if c.HxIndicator != "" {
		return c.HxIndicator
	}
	return "#table-body"
}

// RefreshEvent is the canonical client event the form re-broadcasts after its
// own (filter/search) request completes; deferred panels listen for it to
// reload. Namespaced by table ID so multiple tables on a page don't cross-talk.
func (c *TableConfig) RefreshEvent() string {
	id := c.ID
	if id == "" {
		id = "default"
	}
	return "tbl:" + id + ":refresh"
}

// FormHxAttrs returns extra <form> attributes that re-broadcast RefreshEvent
// after the form's OWN successful request. The event.detail.elt===this guard
// fires only for the form's request (not bubbled panel/infinite-scroll/sort
// requests), so it is loop-free and excludes infinite scroll & sort from panel
// reloads. Empty when no deferred panels are configured.
func (c *TableConfig) FormHxAttrs() templ.Attributes {
	if len(c.DeferredPanels) == 0 {
		return templ.Attributes{}
	}
	return templ.Attributes{
		"hx-on::after-request": fmt.Sprintf(
			"if(event.detail.elt===this && event.detail.successful){htmx.trigger(this,'%s')}",
			c.RefreshEvent(),
		),
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

type TableHeadConfig struct {
	Sticky               bool
	ScrollbarUnderHeader bool
	Attrs                templ.Attributes
}

// DeferredPanel declares a region (e.g. a summary/aggregates bar or a total
// count badge) that paints a skeleton immediately, self-loads its real content
// via hx-get on `load`, and reloads whenever the table's filter/search state
// changes — without being destroyed by row/content swaps.
//
// Panels render inside the table <form> (so they inherit the current filter
// querystring via hx-include) but outside the swap target (so row/content swaps
// never wipe them). Reload is driven by RefreshEvent, which the form
// re-broadcasts after its own successful request (see FormHxAttrs); infinite
// scroll and sort therefore do NOT reload panels.
//
// The endpoint at URL MUST return only the panel's inner content — never the
// wrapper div — otherwise the swap nests a self-loading element.
type DeferredPanel struct {
	// ID is the stable DOM id of the panel wrapper. Required when panels are
	// used; must be unique on the page.
	ID string
	// URL is the hx-get endpoint returning the rendered inner fragment. It
	// receives the table's current filter/search querystring via hx-include.
	URL string
	// Skeleton is the placeholder rendered on first paint. If nil,
	// DefaultPanelSkeleton() is used.
	Skeleton templ.Component
	// Class is appended to the wrapper div's class list (layout/placement).
	Class string
}

type TableConfig struct {
	ID                string
	Configurable      bool
	Title             string
	DataURL           string
	Filters           []templ.Component
	Actions           []templ.Component // Actions like Create button
	Columns           []TableColumn
	Rows              []TableRow
	Head              TableHeadConfig
	Infinite          *InfiniteScrollConfig
	SideFilter        templ.Component
	Editable          TableEditableConfig
	ScrollbarPosition ScrollbarPosition
	WithoutSearch     bool
	SearchPlaceholder string // Custom placeholder for search input

	// Sorting configuration
	CurrentSort      string // Current sort field
	CurrentSortOrder string // Current sort order (asc/desc)

	// Table display customizations
	FillerRows      bool // Enable filler rows to fill vertical space
	FillerRowHeight int  // Filler row height in px (default: 49)
	NoWrap          bool // white-space: nowrap on all td/th

	// TruncateDefault clips all non-sticky columns lacking their own
	// WithTruncate to TruncateDefaultWidth px (single line + overflow tooltip).
	TruncateDefault      bool
	TruncateDefaultWidth int
	ScrollbarGutter      bool   // scrollbar-gutter: stable + hide SDK cover div
	SearchClearable      bool   // Show clear (X) button on search input
	HxTrigger            string // Custom hx-trigger (overrides default)
	FullHeight           bool   // Full-height flex layout (h-full min-h-0 overflow-hidden)
	ContentID            string // ID for content wrapper div (HTMX swap target)
	HxTarget             string // Custom hx-target; defaults to "#"+ContentID if set, else "#table-body"
	HxSwap               string // Custom hx-swap; defaults to "innerHTML"
	HxIndicator          string // Custom hx-indicator; defaults to "#table-body"
	SearchValue          string // Current search input value (for HTMX re-render)
	SearchParamName      string // Query/form field name used for the search value

	// Deferred panels rendered inside the form but outside the swap target;
	// they skeleton-load and reload on filter/search change. See DeferredPanel.
	DeferredPanels []DeferredPanel

	// Footer, when non-empty, renders a sticky <tfoot> row (e.g. column totals)
	// aligned with the columns. One cell per column, index-matched to Columns.
	// Because <tfoot> sits outside <tbody>, infinite-scroll rows never displace
	// it, and it stays pinned to the bottom when the head is sticky.
	Footer []TableCell

	// Optional: reference to definition for advanced usage
	definition *TableDefinition
}

func NewTableConfig(title, dataURL string, opts ...TableConfigOpt) *TableConfig {
	t := &TableConfig{
		Title:           title,
		DataURL:         dataURL,
		Infinite:        &InfiniteScrollConfig{},
		Columns:         []TableColumn{},
		Filters:         []templ.Component{},
		Actions:         []templ.Component{},
		Rows:            []TableRow{},
		Configurable:    true,
		FillerRowHeight: 49, // Default filler row height
		SearchParamName: QueryParamSearch,
	}
	for _, o := range opts {
		o(t)
	}
	if strings.TrimSpace(t.SearchParamName) == "" {
		t.SearchParamName = QueryParamSearch
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

func WithCellClasses(classes templ.CSSClasses) CellOpt {
	return func(c *tableCellImpl) {
		c.classes = classes
	}
}

func WithCellAttrs(attrs templ.Attributes) CellOpt {
	return func(c *tableCellImpl) {
		c.attrs = attrs
	}
}

func Cell(component templ.Component, value any, opts ...CellOpt) TableCell {
	cell := &tableCellImpl{
		component: component,
		value:     value,
	}
	for _, opt := range opts {
		opt(cell)
	}
	return cell
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

// SetFooter sets the sticky footer row cells (one per column, index-matched to
// Columns). Use when totals are computed after NewTableConfig. See TableConfig.Footer.
func (c *TableConfig) SetFooter(cells ...TableCell) *TableConfig {
	c.Footer = cells
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
	values := r.URL.Query()[QueryParamSearch]
	if len(values) > 0 {
		return values[len(values)-1]
	}
	return ""
}

// UsePageQuery gets the "page" query parameter from the request and converts it to int
func UsePageQuery(r *http.Request) int {
	values := r.URL.Query()[QueryParamPage]
	pageStr := ""
	if len(values) > 0 {
		pageStr = values[len(values)-1]
	}
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
	values := r.URL.Query()[QueryParamSort]
	if len(values) > 0 {
		return values[len(values)-1]
	}
	return ""
}

// UseOrderQuery gets the "order" query parameter from the request (asc/desc)
func UseOrderQuery(r *http.Request) string {
	values := r.URL.Query()[QueryParamOrder]
	order := ""
	if len(values) > 0 {
		order = values[len(values)-1]
	}
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
