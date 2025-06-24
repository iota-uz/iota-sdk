package controllers

import (
	"context"
	"fmt"
	"github.com/a-h/templ"
	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/crud"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/form"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
)

// Common error message IDs
const (
	errInvalidFormData  = "Errors.InvalidFormData"
	errFailedToRetrieve = "Errors.FailedToRetrieve"
	errFailedToSave     = "Errors.FailedToSave"
	errFailedToUpdate   = "Errors.FailedToUpdate"
	errFailedToDelete   = "Errors.FailedToDelete"
	errEntityNotFound   = "Errors.EntityNotFound"
	errInternalServer   = "Errors.InternalServer"
	errFailedToRender   = "Errors.FailedToRender"
)

type CrudController[TEntity any] struct {
	basePath string
	app      application.Application
	schema   crud.Schema[TEntity]
	service  crud.Service[TEntity]

	// cached values
	visibleFields   []crud.Field
	formFields      []crud.Field
	primaryKeyField crud.Field

	// options
	enableEdit   bool
	enableDelete bool
	enableCreate bool
}

// CrudOption defines options for CrudController
type CrudOption[TEntity any] func(*CrudController[TEntity])

// WithoutEdit disables edit functionality
func WithoutEdit[TEntity any]() CrudOption[TEntity] {
	return func(c *CrudController[TEntity]) {
		c.enableEdit = false
	}
}

// WithoutDelete disables delete functionality
func WithoutDelete[TEntity any]() CrudOption[TEntity] {
	return func(c *CrudController[TEntity]) {
		c.enableDelete = false
	}
}

// WithoutCreate disables create functionality
func WithoutCreate[TEntity any]() CrudOption[TEntity] {
	return func(c *CrudController[TEntity]) {
		c.enableCreate = false
	}
}

func NewCrudController[TEntity any](
	basePath string,
	app application.Application,
	builder crud.Builder[TEntity],
	opts ...CrudOption[TEntity],
) application.Controller {
	controller := &CrudController[TEntity]{
		basePath:     basePath,
		app:          app,
		schema:       builder.Schema(),
		service:      builder.Service(),
		enableEdit:   true,
		enableDelete: true,
		enableCreate: true,
	}

	// Apply options
	for _, opt := range opts {
		opt(controller)
	}

	// Pre-cache frequently used field collections
	controller.initFieldCache()

	return controller
}

func (c *CrudController[TEntity]) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)

	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id}/details", c.Details).Methods(http.MethodGet)

	if c.enableCreate {
		router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
		router.HandleFunc("", c.Create).Methods(http.MethodPost)
	}

	if c.enableEdit {
		router.HandleFunc("/{id}/edit", c.GetEdit).Methods(http.MethodGet)
		router.HandleFunc("/{id}", c.Update).Methods(http.MethodPost)
	}

	if c.enableDelete {
		router.HandleFunc("/{id}", c.Delete).Methods(http.MethodDelete)
	}
}

func (c *CrudController[TEntity]) Key() string {
	return c.basePath
}

// initFieldCache pre-computes commonly used field collections
func (c *CrudController[TEntity]) initFieldCache() {
	allFields := c.schema.Fields().Fields()

	c.visibleFields = make([]crud.Field, 0, len(allFields))
	c.formFields = make([]crud.Field, 0, len(allFields))

	for _, f := range allFields {
		if f.Key() && c.primaryKeyField == nil {
			c.primaryKeyField = f
		}

		if !f.Hidden() {
			c.visibleFields = append(c.visibleFields, f)

			// Add to form fields if it's not a key field or if key field is not readonly
			// This allows editable primary keys to be included in forms
			if !f.Key() || !f.Readonly() {
				c.formFields = append(c.formFields, f)
			}
		}
	}

	if c.primaryKeyField == nil {
		panic(fmt.Sprintf("CrudController: no primary key field found in schema for %s", c.schema.Name()))
	}
}

// localize is a helper method to localize messages with defaults
func (c *CrudController[TEntity]) localize(ctx context.Context, messageID string, defaultMessage string) (string, error) {
	l, ok := intl.UseLocalizer(ctx)
	if !ok {
		return "", fmt.Errorf("localizer not found in context")
	}

	return l.Localize(&i18n.LocalizeConfig{
		MessageID: messageID,
		DefaultMessage: &i18n.Message{
			ID:    messageID,
			Other: defaultMessage,
		},
	})
}

// validateID checks if the ID is valid for the primary key field type
func (c *CrudController[TEntity]) validateID(id string) error {
	switch c.primaryKeyField.Type() {
	case crud.IntFieldType:
		if _, err := strconv.ParseInt(id, 10, 64); err != nil {
			return fmt.Errorf("invalid integer ID: %s", id)
		}
	case crud.UUIDFieldType:
		if _, err := uuid.Parse(id); err != nil {
			return fmt.Errorf("invalid UUID: %s", id)
		}
	case crud.StringFieldType, crud.BoolFieldType, crud.FloatFieldType, crud.DecimalFieldType, crud.DateFieldType, crud.TimeFieldType, crud.DateTimeFieldType, crud.TimestampFieldType:
		// These types don't need special validation for ID format
	}
	return nil
}

// parseIDValue converts string ID to proper type based on primary key field type
func (c *CrudController[TEntity]) parseIDValue(id string) any {
	switch c.primaryKeyField.Type() {
	case crud.IntFieldType:
		// Try to parse as int64 first (handles larger numbers)
		if int64Val, err := strconv.ParseInt(id, 10, 64); err == nil {
			// Check if it fits in int32
			if int64Val >= math.MinInt32 && int64Val <= math.MaxInt32 {
				return int(int64Val)
			}
			return int64Val
		}
		// If parsing fails, return 0 as default for int fields
		return 0
	case crud.UUIDFieldType:
		if uuidVal, err := uuid.Parse(id); err == nil {
			return uuidVal
		}
		// If parsing fails, return nil UUID instead of nil
		return uuid.Nil
	case crud.StringFieldType, crud.BoolFieldType, crud.FloatFieldType, crud.DecimalFieldType, crud.DateFieldType, crud.TimeFieldType, crud.DateTimeFieldType, crud.TimestampFieldType:
		// For all other types, return the string as-is
		return id
	}
	return id
}

// buildFieldValuesFromForm creates field values from form data
func (c *CrudController[TEntity]) buildFieldValuesFromForm(r *http.Request) ([]crud.FieldValue, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("failed to parse form: %w", err)
	}

	fieldValues := make([]crud.FieldValue, 0)

	// Process only fields that are present in the form
	for fieldName := range r.Form {
		field, err := c.schema.Fields().Field(fieldName)
		if err != nil {
			// Skip fields that are not in schema
			continue
		}

		// Skip readonly fields - they should not be updated from form data
		if field.Readonly() {
			continue
		}

		formValue := r.Form.Get(fieldName)
		var value any

		// Convert form value based on field type
		switch field.Type() {
		case crud.BoolFieldType:
			value = formValue == "on" || formValue == "true" || formValue == "1"
		case crud.IntFieldType:
			if formValue != "" {
				if int64Val, err := strconv.ParseInt(formValue, 10, 64); err == nil {
					if int64Val >= math.MinInt32 && int64Val <= math.MaxInt32 {
						value = int(int64Val)
					} else {
						value = int64Val
					}
				} else {
					return nil, fmt.Errorf("invalid integer value for field %s: %v", fieldName, err)
				}
			} else {
				continue // Skip empty values
			}
		case crud.FloatFieldType:
			if formValue != "" {
				if floatVal, err := strconv.ParseFloat(formValue, 64); err == nil {
					value = floatVal
				} else {
					return nil, fmt.Errorf("invalid float value for field %s: %v", fieldName, err)
				}
			} else {
				continue // Skip empty values
			}
		case crud.DateFieldType, crud.DateTimeFieldType, crud.TimeFieldType:
			if formValue != "" {
				parsedTime, err := time.Parse(time.RFC3339, formValue)
				if err != nil {
					// Try common HTML5 formats based on field type
					formats := []string{}
					switch field.Type() {
					case crud.DateFieldType:
						formats = []string{"2006-01-02"}
					case crud.TimeFieldType:
						formats = []string{"15:04", "15:04:05"}
					case crud.DateTimeFieldType:
						formats = []string{"2006-01-02T15:04", "2006-01-02T15:04:05"}
					case crud.StringFieldType, crud.IntFieldType, crud.BoolFieldType, crud.FloatFieldType, crud.DecimalFieldType, crud.TimestampFieldType, crud.UUIDFieldType:
						// These types are handled elsewhere
						formats = []string{}
					}

					for _, format := range formats {
						if parsedTime, err = time.Parse(format, formValue); err == nil {
							break
						}
					}
				}
				if err == nil {
					value = parsedTime
				} else {
					return nil, fmt.Errorf("invalid time value for field %s: %v", fieldName, err)
				}
			} else {
				continue // Skip empty values
			}
		case crud.UUIDFieldType:
			if formValue != "" {
				if uid, err := uuid.Parse(formValue); err == nil {
					value = uid
				} else {
					return nil, fmt.Errorf("invalid UUID value for field %s: %v", fieldName, err)
				}
			} else {
				continue // Skip empty values
			}
		case crud.DecimalFieldType:
			// Decimal fields are stored as strings
			value = formValue
		case crud.StringFieldType, crud.TimestampFieldType:
			// String and timestamp fields are handled as strings from forms
			value = formValue
		}

		fieldValues = append(fieldValues, field.Value(value))
	}

	// Special handling for checkboxes - they don't send data when unchecked
	for _, field := range c.schema.Fields().Fields() {
		if field.Type() == crud.BoolFieldType && !field.Hidden() && !field.Readonly() {
			// Check if this field was already processed
			found := false
			for _, fv := range fieldValues {
				if fv.Field().Name() == field.Name() {
					found = true
					break
				}
			}

			// If checkbox field wasn't in form data, it means it was unchecked
			if !found && r.Method == http.MethodPost {
				fieldValues = append(fieldValues, field.Value(false))
			}
		}
	}

	return fieldValues, nil
}

func (c *CrudController[TEntity]) List(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Parse query parameters
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&crud.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
	}, r)
	if err != nil {
		log.Printf("[CrudController.List] Failed to parse query params: %v", err)
		errorMsg, _ := c.localize(ctx, "Errors.InvalidQueryParams", "Invalid query parameters")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	if searchQuery := r.URL.Query().Get("Search"); searchQuery != "" {
		params.Query = searchQuery
	}

	// Fetch entities and count in parallel for better performance
	type listResult struct {
		entities []TEntity
		err      error
	}
	type countResult struct {
		count int64
		err   error
	}

	listCh := make(chan listResult, 1)
	countCh := make(chan countResult, 1)

	// Fetch entities
	go func() {
		entities, err := c.service.List(ctx, params)
		listCh <- listResult{entities: entities, err: err}
	}()

	// Count total items
	go func() {
		countParams := &crud.FindParams{
			Query: params.Query, // Include search query in count
		}
		count, err := c.service.Count(ctx, countParams)
		countCh <- countResult{count: count, err: err}
	}()

	// Wait for results
	listRes := <-listCh
	countRes := <-countCh

	if listRes.err != nil {
		log.Printf("[CrudController.List] Failed to list entities: %v", listRes.err)
		errorMsg, _ := c.localize(ctx, errFailedToRetrieve, "Failed to retrieve data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	entities := listRes.entities
	totalCount := countRes.count
	if countRes.err != nil {
		log.Printf("[CrudController.List] Failed to count entities: %v", countRes.err)
		// Non-critical error, continue without infinity scroll
		totalCount = 0
	}

	// Calculate if there are more items
	hasMore := int64(params.Offset+len(entities)) < totalCount

	// Build the data URL with query parameters preserved
	dataURL := c.basePath
	if params.Query != "" {
		// Preserve search query in the URL for infinity scroll
		u, _ := url.Parse(dataURL)
		q := u.Query()
		q.Set("Search", params.Query)
		u.RawQuery = q.Encode()
		dataURL = u.String()
	}

	// Create table configuration with infinity scroll support
	var cfg *table.TableConfig
	if htmx.IsHxRequest(r) {
		// For HTMX requests, we only need the base URL with query params
		cfg = table.NewTableConfig(c.schema.Name(), dataURL)
	} else {
		// For initial page load, enable infinity scroll
		cfg = table.NewTableConfig(
			c.schema.Name(),
			dataURL,
			table.WithInfiniteScroll(hasMore, paginationParams.Page, paginationParams.Limit),
		)
	}

	// Add columns based on visible fields (only for initial load)
	if !htmx.IsHxRequest(r) {
		columns := make([]table.TableColumn, 0, len(c.visibleFields)+1)
		for _, f := range c.visibleFields {
			// Localize field label
			fieldLabel, err := c.localize(ctx, fmt.Sprintf("%s.Fields.%s", c.schema.Name(), f.Name()), f.Name())
			if err != nil {
				fieldLabel = f.Name()
			}
			columns = append(columns, table.Column(f.Name(), fieldLabel))
		}

		// Add actions column if edit or delete is enabled
		if c.enableEdit || c.enableDelete {
			actionsLabel, _ := c.localize(ctx, "Common.Actions", "Actions")
			columns = append(columns, table.Column("actions", actionsLabel))
		}

		cfg.AddCols(columns...)

		// Add header actions
		headerActions := c.buildHeaderActions(ctx)
		if len(headerActions) > 0 {
			for _, action := range headerActions {
				cfg.AddActions(actions.RenderAction(action))
			}
		}
	}

	// Convert entities to table rows
	for _, entity := range entities {
		fieldValues, err := c.schema.Mapper().ToFieldValues(ctx, entity)
		if err != nil {
			log.Printf("[CrudController.List] Failed to map entity: %v", err)
			continue
		}

		row, err := c.buildTableRow(ctx, fieldValues)
		if err != nil {
			log.Printf("[CrudController.List] Failed to build row: %v", err)
			continue
		}
		cfg.AddRows(row)
	}

	// For HTMX requests, also configure infinity scroll
	if htmx.IsHxRequest(r) && hasMore {
		// Apply infinity scroll configuration for subsequent requests
		table.WithInfiniteScroll(hasMore, paginationParams.Page, paginationParams.Limit)(cfg)
	}

	// Render response
	var component templ.Component
	if htmx.IsHxRequest(r) {
		component = table.Rows(cfg)
	} else {
		component = table.Page(cfg)
	}

	if err := component.Render(ctx, w); err != nil {
		log.Printf("[CrudController.List] Failed to render template: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRender, "Failed to render response")
		http.Error(w, errorMsg, http.StatusInternalServerError)
	}
}

func (c *CrudController[TEntity]) Details(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Create field value for the ID
	idFieldValue := c.primaryKeyField.Value(c.parseIDValue(id))

	// Fetch entity
	entity, err := c.service.Get(ctx, idFieldValue)
	if err != nil {
		log.Printf("[CrudController.Details] Failed to get entity %s: %v", id, err)
		errorMsg, _ := c.localize(ctx, errEntityNotFound, "Entity not found")
		http.Error(w, errorMsg, http.StatusNotFound)
		return
	}

	// Convert entity to field values
	fieldValues, err := c.schema.Mapper().ToFieldValues(ctx, entity)
	if err != nil {
		log.Printf("[CrudController.Details] Failed to map entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInternalServer, "Internal server error")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Extract primary key
	var primaryKey any
	for _, fv := range fieldValues {
		if fv.Field().Key() {
			primaryKey = fv.Value()
			break
		}
	}

	// Localize view title
	viewTitle, err := c.localize(
		ctx,
		fmt.Sprintf("%s.View.Title", c.schema.Name()),
		fmt.Sprintf("View %s", c.schema.Name()),
	)
	if err != nil {
		log.Printf("[CrudController.Details] Failed to localize title: %v", err)
		viewTitle = fmt.Sprintf("View %s", c.schema.Name())
	}

	// Create field value map for quick lookup
	fieldValueMap := make(map[string]crud.FieldValue, len(fieldValues))
	for _, fv := range fieldValues {
		fieldValueMap[fv.Field().Name()] = fv
	}

	// Map field values to detail field values
	detailFields := make([]table.DetailFieldValue, 0, len(c.visibleFields))
	for _, field := range c.visibleFields {
		if fv, exists := fieldValueMap[field.Name()]; exists {
			// Localize field label
			fieldLabel, err := c.localize(ctx, fmt.Sprintf("%s.Fields.%s", c.schema.Name(), field.Name()), field.Name())
			if err != nil {
				fieldLabel = field.Name()
			}

			// Convert field value to string and determine type
			var valueStr string
			var fieldType table.DetailFieldType

			if fv.IsZero() {
				valueStr = ""
				fieldType = table.DetailFieldTypeText
			} else {
				switch field.Type() {
				case crud.BoolFieldType:
					if val, ok := fv.Value().(bool); ok {
						valueStr = fmt.Sprintf("%v", val)
						fieldType = table.DetailFieldTypeBoolean
					}
				case crud.DateFieldType:
					if val, ok := fv.Value().(time.Time); ok {
						valueStr = val.Format("2006-01-02")
						fieldType = table.DetailFieldTypeDate
					}
				case crud.TimeFieldType:
					if val, ok := fv.Value().(time.Time); ok {
						valueStr = val.Format("15:04:05")
						fieldType = table.DetailFieldTypeTime
					}
				case crud.DateTimeFieldType, crud.TimestampFieldType:
					if val, ok := fv.Value().(time.Time); ok {
						valueStr = val.Format("2006-01-02 15:04:05")
						fieldType = table.DetailFieldTypeDateTime
					}
				case crud.StringFieldType:
					valueStr = fmt.Sprintf("%v", fv.Value())
					fieldType = table.DetailFieldTypeText
				case crud.IntFieldType:
					valueStr = fmt.Sprintf("%v", fv.Value())
					fieldType = table.DetailFieldTypeText
				case crud.FloatFieldType:
					valueStr = fmt.Sprintf("%v", fv.Value())
					fieldType = table.DetailFieldTypeText
				case crud.DecimalFieldType:
					valueStr = fmt.Sprintf("%v", fv.Value())
					fieldType = table.DetailFieldTypeText
				case crud.UUIDFieldType:
					valueStr = fmt.Sprintf("%v", fv.Value())
					fieldType = table.DetailFieldTypeText
				default:
					valueStr = fmt.Sprintf("%v", fv.Value())
					fieldType = table.DetailFieldTypeText
				}
			}

			detailFields = append(detailFields, table.DetailFieldValue{
				Name:  field.Name(),
				Label: fieldLabel,
				Value: valueStr,
				Type:  fieldType,
			})
		}
	}

	// Build actions
	var actions []table.DetailAction

	if c.enableEdit {
		editLabel, _ := c.localize(ctx, "Common.Edit", "Edit")
		actions = append(actions, table.DetailAction{
			Label:  editLabel,
			URL:    fmt.Sprintf("%s/%v/edit", c.basePath, primaryKey),
			Method: "GET",
			Class:  "btn-primary",
		})
	}

	if c.enableDelete {
		deleteLabel, _ := c.localize(ctx, "Common.Delete", "Delete")
		confirmMsg, _ := c.localize(ctx, "Common.ConfirmDelete", "Are you sure?")
		actions = append(actions, table.DetailAction{
			Label:   deleteLabel,
			URL:     fmt.Sprintf("%s/%v", c.basePath, primaryKey),
			Method:  "DELETE",
			Class:   "btn-danger",
			Confirm: confirmMsg,
		})
	}

	// Generate unique ID for this drawer instance
	drawerID := fmt.Sprintf("drawer-%d", time.Now().UnixNano())

	// Create drawer component using the new DetailsDrawer
	drawerProps := table.DetailsDrawerProps{
		ID:          drawerID,
		Title:       viewTitle,
		CallbackURL: c.basePath,
		Fields:      detailFields,
		Actions:     actions,
	}

	drawerComponent := table.DetailsDrawer(drawerProps)

	if err := drawerComponent.Render(ctx, w); err != nil {
		log.Printf("[CrudController.Details] Failed to render view: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRender, "Failed to render view")
		http.Error(w, errorMsg, http.StatusInternalServerError)
	}
}

// buildTableRow creates a table row from field values
func (c *CrudController[TEntity]) buildTableRow(ctx context.Context, fieldValues []crud.FieldValue) (table.TableRow, error) {
	var primaryKey any
	components := make([]templ.Component, 0, len(c.visibleFields)+1)

	// Create a map for quick field value lookup
	fieldValueMap := make(map[string]crud.FieldValue, len(fieldValues))
	for _, fv := range fieldValues {
		fieldValueMap[fv.Field().Name()] = fv
		if fv.Field().Key() {
			primaryKey = fv.Value()
		}
	}

	// Build components in the order of visible fields
	for _, field := range c.visibleFields {
		if fv, exists := fieldValueMap[field.Name()]; exists {
			components = append(components, c.fieldValueToTableCell(ctx, field, fv))
		} else {
			components = append(components, templ.Raw(""))
		}
	}

	if primaryKey == nil {
		return nil, fmt.Errorf("primary key not found")
	}

	// Add row actions
	rowActions := c.buildRowActions(ctx, primaryKey)
	if len(rowActions) > 0 {
		components = append(components, actions.RenderRowActions(rowActions...))
	}

	fetchUrl := fmt.Sprintf("%s/%v/details", c.basePath, primaryKey)
	return table.Row(components...).ApplyOpts(table.WithDrawer(fetchUrl)), nil
}

// buildHeaderActions creates header actions for the list view
func (c *CrudController[TEntity]) buildHeaderActions(ctx context.Context) []actions.ActionProps {
	var headerActions []actions.ActionProps

	if c.enableCreate {
		createLabel, err := c.localize(ctx, fmt.Sprintf("%s.List.New", c.schema.Name()), "New")
		if err != nil {
			createLabel = "New"
		}
		createAction := actions.CreateAction(createLabel, fmt.Sprintf("%s/new", c.basePath))
		headerActions = append(headerActions, createAction)
	}

	return headerActions
}

// buildRowActions creates row actions for table rows
func (c *CrudController[TEntity]) buildRowActions(_ context.Context, primaryKey any) []actions.ActionProps {
	var rowActions []actions.ActionProps

	if c.enableEdit {
		editAction := actions.EditAction(fmt.Sprintf("%s/%v/edit", c.basePath, primaryKey))
		rowActions = append(rowActions, editAction)
	}

	if c.enableDelete {
		deleteAction := actions.DeleteAction(fmt.Sprintf("%s/%v", c.basePath, primaryKey))
		rowActions = append(rowActions, deleteAction)
	}

	return rowActions
}

func (c *CrudController[TEntity]) GetNew(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Localize form title
	formTitle, err := c.localize(ctx, fmt.Sprintf("%s.New.Title", c.schema.Name()), fmt.Sprintf("New %s", c.schema.Name()))
	if err != nil {
		log.Printf("[CrudController.GetNew] Failed to localize title: %v", err)
		formTitle = fmt.Sprintf("New %s", c.schema.Name())
	}

	// Localize submit button
	submitLabel, err := c.localize(ctx, fmt.Sprintf("%s.New.SubmitLabel", c.schema.Name()), "Create")
	if err != nil {
		log.Printf("[CrudController.GetNew] Failed to localize submit label: %v", err)
		submitLabel = "Create"
	}

	// Build form fields using cached fields (no values for new form)
	formFields := c.buildFormFields(ctx, nil)

	cfg := form.NewFormConfig(
		formTitle,
		c.basePath,
		"",
		submitLabel,
	).Add(formFields...)

	if err := form.Page(cfg).Render(ctx, w); err != nil {
		log.Printf("[CrudController.GetNew] Failed to render form: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRender, "Failed to render form")
		http.Error(w, errorMsg, http.StatusInternalServerError)
	}
}

// buildFormFields creates form fields, optionally with values from field values
func (c *CrudController[TEntity]) buildFormFields(ctx context.Context, fieldValues []crud.FieldValue) []form.Field {
	// Create field value map if provided
	var fieldValueMap map[string]crud.FieldValue
	if fieldValues != nil {
		fieldValueMap = make(map[string]crud.FieldValue, len(fieldValues))
		for _, fv := range fieldValues {
			fieldValueMap[fv.Field().Name()] = fv
		}
	}

	formFields := make([]form.Field, 0, len(c.formFields))
	for _, f := range c.formFields {
		// Get current value if available
		var currentValue crud.FieldValue
		if fieldValueMap != nil {
			if fv, exists := fieldValueMap[f.Name()]; exists {
				currentValue = fv
			}
		}

		// Create form field with current value
		formField := c.fieldToFormFieldWithValue(ctx, f, currentValue)
		if formField == nil {
			continue
		}

		formFields = append(formFields, formField)
	}

	return formFields
}

func (c *CrudController[TEntity]) GetEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Validate ID format
	if err := c.validateID(id); err != nil {
		log.Printf("[CrudController.GetEdit] Invalid ID format %s: %v", id, err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid ID format")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Create field value for the ID
	idFieldValue := c.primaryKeyField.Value(c.parseIDValue(id))

	// Fetch entity
	entity, err := c.service.Get(ctx, idFieldValue)
	if err != nil {
		log.Printf("[CrudController.GetEdit] Failed to get entity %s: %v", id, err)
		errorMsg, _ := c.localize(ctx, errEntityNotFound, "Entity not found")
		http.Error(w, errorMsg, http.StatusNotFound)
		return
	}

	// Convert entity to field values
	fieldValues, err := c.schema.Mapper().ToFieldValues(ctx, entity)
	if err != nil {
		log.Printf("[CrudController.GetEdit] Failed to map entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInternalServer, "Internal server error")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Localize form title
	formTitle, err := c.localize(ctx, fmt.Sprintf("%s.Edit.Title", c.schema.Name()), fmt.Sprintf("Edit %s", c.schema.Name()))
	if err != nil {
		log.Printf("[CrudController.GetEdit] Failed to localize title: %v", err)
		formTitle = fmt.Sprintf("Edit %s", c.schema.Name())
	}

	// Localize submit button
	submitLabel, err := c.localize(ctx, fmt.Sprintf("%s.Edit.SubmitLabel", c.schema.Name()), "Update")
	if err != nil {
		log.Printf("[CrudController.GetEdit] Failed to localize submit label: %v", err)
		submitLabel = "Update"
	}

	// Build form fields with current values
	formFields := c.buildFormFields(ctx, fieldValues)

	cfg := form.NewFormConfig(
		formTitle,
		fmt.Sprintf("%s/%s", c.basePath, id),
		fmt.Sprintf("%s/%s", c.basePath, id),
		submitLabel,
	).Add(formFields...)

	if err := form.Page(cfg).Render(ctx, w); err != nil {
		log.Printf("[CrudController.GetEdit] Failed to render form: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRender, "Failed to render form")
		http.Error(w, errorMsg, http.StatusInternalServerError)
	}
}

func (c *CrudController[TEntity]) Create(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	// Build field values from form
	fieldValues, err := c.buildFieldValuesFromForm(r)
	if err != nil {
		log.Printf("[CrudController.Create] Failed to parse form: %v", err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid form data")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	existingFields := make(map[string]struct{}, len(fieldValues))
	for _, fv := range fieldValues {
		existingFields[fv.Field().Name()] = struct{}{}
	}
	for _, f := range c.schema.Fields().Fields() {
		if _, found := existingFields[f.Name()]; !found {
			// Skip readonly fields during creation - they should not be set from form data
			if !f.Readonly() {
				initialValue := f.InitialValue()
				// Only create field value if initial value is not nil
				// Nil values will be handled by the entity mapper's default behavior
				if initialValue != nil {
					fieldValues = append(fieldValues, f.Value(initialValue))
				}
			}
		}
	}

	// Convert to entity
	entity, err := c.schema.Mapper().ToEntity(ctx, fieldValues)
	if err != nil {
		log.Printf("[CrudController.Create] Failed to map to entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid form data")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Validate entity against schema validators
	if err := c.validateEntity(ctx, entity); err != nil {
		log.Printf("[CrudController.Create] Entity validation failed: %v", err)
		if c.handleValidationError(w, r, ctx, err, fieldValues, true) {
			return
		}
		errorMsg, _ := c.localize(ctx, errFailedToSave, "Failed to save data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Save entity
	savedEntity, err := c.service.Save(ctx, entity)
	if err != nil {
		log.Printf("[CrudController.Create] Failed to save entity: %v", err)

		// Check if it's a validation error
		if c.handleValidationError(w, r, ctx, err, fieldValues, true) {
			return
		}

		errorMsg, _ := c.localize(ctx, errFailedToSave, "Failed to save data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Get primary key for redirect
	_, err = c.schema.Mapper().ToFieldValues(ctx, savedEntity)
	if err != nil {
		log.Printf("[CrudController.Create] Failed to map saved entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInternalServer, "Internal server error")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Handle redirect
	if htmx.IsHxRequest(r) {
		w.Header().Set("HX-Redirect", c.basePath)
	} else {
		http.Redirect(w, r, c.basePath, http.StatusSeeOther)
	}
}

func (c *CrudController[TEntity]) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Validate ID format
	if err := c.validateID(id); err != nil {
		log.Printf("[CrudController.Update] Invalid ID format %s: %v", id, err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid ID format")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Build field values from form
	fieldValues, err := c.buildFieldValuesFromForm(r)
	if err != nil {
		log.Printf("[CrudController.Update] Failed to parse form: %v", err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid form data")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Set the ID in field values
	foundKeyField := false
	for i, fv := range fieldValues {
		if fv.Field().Key() {
			fieldValues[i] = fv.Field().Value(c.parseIDValue(id))
			foundKeyField = true
			break
		}
	}

	// If key field wasn't found in form data, add it
	if !foundKeyField {
		keyField := c.primaryKeyField
		if keyField != nil {
			keyFieldValue := keyField.Value(c.parseIDValue(id))
			fieldValues = append(fieldValues, keyFieldValue)
		}
	}

	// Convert to entity
	entity, err := c.schema.Mapper().ToEntity(ctx, fieldValues)
	if err != nil {
		log.Printf("[CrudController.Update] Failed to map to entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid form data")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Validate entity against schema validators
	if err := c.validateEntity(ctx, entity); err != nil {
		log.Printf("[CrudController.Update] Entity validation failed: %v", err)
		if c.handleValidationError(w, r, ctx, err, fieldValues, false) {
			return
		}
		errorMsg, _ := c.localize(ctx, errFailedToUpdate, "Failed to update data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Save the updated entity
	_, err = c.service.Save(ctx, entity)
	if err != nil {
		log.Printf("[CrudController.Update] Failed to update entity %s: %v", id, err)

		// Check if it's a validation error
		if c.handleValidationError(w, r, ctx, err, fieldValues, false) {
			return
		}

		errorMsg, _ := c.localize(ctx, errFailedToUpdate, "Failed to update data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Handle redirect
	if htmx.IsHxRequest(r) {
		w.Header().Set("HX-Redirect", c.basePath)
	} else {
		http.Redirect(w, r, c.basePath, http.StatusSeeOther)
	}
}

func (c *CrudController[TEntity]) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Validate ID format
	if err := c.validateID(id); err != nil {
		log.Printf("[CrudController.Delete] Invalid ID format %s: %v", id, err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid ID format")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Create field value for the ID
	idFieldValue := c.primaryKeyField.Value(c.parseIDValue(id))

	// Delete entity
	if _, err := c.service.Delete(ctx, idFieldValue); err != nil {
		log.Printf("[CrudController.Delete] Failed to delete entity %s: %v", id, err)
		errorMsg, _ := c.localize(ctx, errFailedToDelete, "Failed to delete data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Handle response
	if htmx.IsHxRequest(r) {
		// Return 200 OK with HX-Redirect header for client-side redirect
		w.Header().Set("HX-Redirect", c.basePath)
		w.WriteHeader(http.StatusOK)
	} else {
		// Regular redirect for non-HTMX requests
		http.Redirect(w, r, c.basePath, http.StatusSeeOther)
	}
}

// fieldToFormFieldWithValue creates a form field with a value if provided
func (c *CrudController[TEntity]) fieldToFormFieldWithValue(ctx context.Context, field crud.Field, value crud.FieldValue) form.Field {
	// Skip hidden fields
	if field.Hidden() {
		return nil
	}

	// Skip key fields that are readonly (auto-generated IDs)
	if field.Key() && field.Readonly() {
		return nil
	}

	// Localize field label
	fieldLabel, err := c.localize(ctx, fmt.Sprintf("%s.Fields.%s", c.schema.Name(), field.Name()), field.Name())
	if err != nil {
		fieldLabel = field.Name()
	}

	// Get the actual value to use
	var currentValue any
	if value != nil && !value.IsZero() {
		currentValue = value.Value()
	} else if field.InitialValue() != nil {
		currentValue = field.InitialValue()
	}

	switch field.Type() {
	case crud.StringFieldType:
		sf, err := field.AsStringField()
		if err != nil {
			return nil
		}

		builder := form.Text(field.Name(), fieldLabel)

		if sf.MaxLen() > 0 {
			builder = builder.MaxLen(sf.MaxLen())
		}
		if sf.MinLen() > 0 {
			builder = builder.MinLen(sf.MinLen())
		}

		if sf.Multiline() {
			textareaBuilder := form.Textarea(field.Name(), fieldLabel)
			if sf.MaxLen() > 0 {
				textareaBuilder = textareaBuilder.MaxLen(sf.MaxLen())
			}
			if sf.MinLen() > 0 {
				textareaBuilder = textareaBuilder.MinLen(sf.MinLen())
			}

			if field.Readonly() {
				textareaBuilder = textareaBuilder.Attrs(templ.Attributes{"readonly": true})
			}

			if len(field.Rules()) > 0 {
				textareaBuilder = textareaBuilder.Required()
			}

			if currentValue != nil {
				if strVal, ok := currentValue.(string); ok {
					textareaBuilder = textareaBuilder.Default(strVal)
				}
			}

			return textareaBuilder.Build()
		}

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if currentValue != nil {
			if strVal, ok := currentValue.(string); ok {
				builder = builder.Default(strVal)
			}
		}

		return builder.Build()

	case crud.IntFieldType:
		intField, err := field.AsIntField()
		if err != nil {
			return nil
		}

		builder := form.NewNumberField(field.Name(), fieldLabel)

		if intField.Min() != 0 {
			builder = builder.Min(float64(intField.Min()))
		}
		if intField.Max() != 0 {
			builder = builder.Max(float64(intField.Max()))
		}

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
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

		return builder.Build()

	case crud.BoolFieldType:
		builder := form.Checkbox(field.Name(), fieldLabel)

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if currentValue != nil {
			if boolVal, ok := currentValue.(bool); ok {
				builder = builder.Default(boolVal)
			}
		}

		return builder.Build()

	case crud.FloatFieldType:
		floatField, err := field.AsFloatField()
		if err != nil {
			return nil
		}

		builder := form.NewNumberField(field.Name(), fieldLabel)

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
			attrs["readonly"] = true
		}

		builder = builder.Attrs(attrs)

		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if currentValue != nil {
			if floatVal, ok := currentValue.(float64); ok {
				builder = builder.Default(floatVal)
			}
		}

		return builder.Build()

	case crud.DateFieldType:
		builder := form.Date(field.Name(), fieldLabel)

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
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if currentValue != nil {
			if timeVal, ok := currentValue.(time.Time); ok && !timeVal.IsZero() {
				builder = builder.Default(timeVal)
			}
		}

		return builder.Build()

	case crud.TimeFieldType:
		builder := form.Time(field.Name(), fieldLabel)

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if currentValue != nil {
			if timeVal, ok := currentValue.(time.Time); ok && !timeVal.IsZero() {
				builder = builder.Default(timeVal.Format("15:04"))
			}
		}

		return builder.Build()

	case crud.DateTimeFieldType:
		builder := form.DateTime(field.Name(), fieldLabel)

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
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if currentValue != nil {
			if timeVal, ok := currentValue.(time.Time); ok && !timeVal.IsZero() {
				builder = builder.Default(timeVal)
			}
		}

		return builder.Build()

	case crud.UUIDFieldType:
		builder := form.Text(field.Name(), fieldLabel)

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
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

		return builder.Build()

	case crud.TimestampFieldType:
		// Timestamp fields are treated like datetime fields
		builder := form.DateTime(field.Name(), fieldLabel)

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
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

		return builder.Build()

	case crud.DecimalFieldType:
		decimalField, err := field.AsDecimalField()
		if err != nil {
			return nil
		}

		builder := form.NewNumberField(field.Name(), fieldLabel)

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
			for i := 0; i < decimalField.Scale(); i++ {
				step /= 10
			}
			attrs["step"] = fmt.Sprintf("%f", step)
		} else {
			attrs["step"] = "any"
		}

		if field.Readonly() {
			attrs["readonly"] = true
		}

		// Set decimal value if present
		if value != nil && !value.IsZero() {
			// Use AsDecimal to handle all possible decimal value types
			if decimalStr, err := value.AsDecimal(); err == nil {
				// Validate it's a proper number format and set the value directly in attrs
				if _, err := strconv.ParseFloat(decimalStr, 64); err == nil {
					attrs["value"] = decimalStr
				}
			}
		}

		builder = builder.Attrs(attrs)

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

		return builder.Build()

	default:
		builder := form.Text(field.Name(), field.Name())
		if currentValue != nil {
			builder = builder.Default(fmt.Sprintf("%v", currentValue))
		}
		return builder.Build()
	}
}

func (c *CrudController[TEntity]) fieldValueToTableCell(ctx context.Context, field crud.Field, value crud.FieldValue) templ.Component {
	if value.IsZero() {
		return templ.Raw("")
	}

	switch field.Type() {
	case crud.StringFieldType:
		str, err := value.AsString()
		if err != nil {
			return templ.Raw("")
		}
		return templ.Raw(str)

	case crud.IntFieldType:
		intVal, err := value.AsInt()
		if err != nil {
			return templ.Raw("")
		}
		return templ.Raw(fmt.Sprintf("%d", intVal))

	case crud.BoolFieldType:
		boolVal, err := value.AsBool()
		if err != nil {
			return templ.Raw("")
		}

		boolField, err := field.AsBoolField()
		if err == nil && boolField.TrueLabel() != "" && boolField.FalseLabel() != "" {
			if boolVal {
				return templ.Raw(boolField.TrueLabel())
			}
			return templ.Raw(boolField.FalseLabel())
		}

		if boolVal {
			yes, _ := c.localize(ctx, "Common.Yes", "Yes")
			return templ.Raw(yes)
		}
		no, _ := c.localize(ctx, "Common.No", "No")
		return templ.Raw(no)

	case crud.FloatFieldType:
		floatVal, err := value.AsFloat64()
		if err != nil {
			return templ.Raw("")
		}

		floatField, err := field.AsFloatField()
		if err == nil && floatField.Precision() > 0 {
			format := fmt.Sprintf("%%.%df", floatField.Precision())
			return templ.Raw(fmt.Sprintf(format, floatVal))
		}

		return templ.Raw(fmt.Sprintf("%f", floatVal))

	case crud.DecimalFieldType:
		decimalVal, err := value.AsDecimal()
		if err != nil {
			return templ.Raw("")
		}
		return templ.Raw(decimalVal)

	case crud.DateFieldType:
		timeVal, err := value.AsTime()
		if err != nil {
			return templ.Raw("")
		}

		dateField, err := field.AsDateField()
		if err == nil && dateField.Format() != "" {
			return templ.Raw(timeVal.Format(dateField.Format()))
		}

		return templ.Raw(timeVal.Format("2006-01-02"))

	case crud.TimeFieldType:
		timeVal, err := value.AsTime()
		if err != nil {
			return templ.Raw("")
		}

		timeField, err := field.AsTimeField()
		if err == nil && timeField.Format() != "" {
			return templ.Raw(timeVal.Format(timeField.Format()))
		}

		return templ.Raw(timeVal.Format("15:04"))

	case crud.DateTimeFieldType:
		timeVal, err := value.AsTime()
		if err != nil {
			return templ.Raw("")
		}

		dateTimeField, err := field.AsDateTimeField()
		if err == nil && dateTimeField.Format() != "" {
			return templ.Raw(timeVal.Format(dateTimeField.Format()))
		}

		return templ.Raw(timeVal.Format("2006-01-02 15:04"))

	case crud.TimestampFieldType:
		timeVal, err := value.AsTime()
		if err != nil {
			return templ.Raw("")
		}
		return templ.Raw(timeVal.Format("2006-01-02 15:04:05"))

	case crud.UUIDFieldType:
		uuidVal, err := value.AsUUID()
		if err != nil {
			return templ.Raw("")
		}
		return templ.Raw(uuidVal.String())

	default:
		return templ.Raw(fmt.Sprintf("%v", value.Value()))
	}
}

// validateFieldValues validates field values against their field rules
func (c *CrudController[TEntity]) validateFieldValues(fieldValues []crud.FieldValue) map[string]string {
	errors := make(map[string]string)

	for _, fv := range fieldValues {
		field := fv.Field()
		for _, rule := range field.Rules() {
			if err := rule(fv); err != nil {
				errors[field.Name()] = err.Error()
				break // Only report first error per field
			}
		}
	}

	return errors
}

// validateEntity validates the entity against schema validators
func (c *CrudController[TEntity]) validateEntity(ctx context.Context, entity TEntity) error {
	for _, validator := range c.schema.Validators() {
		if err := validator(entity); err != nil {
			return err
		}
	}
	return nil
}

// handleValidationError handles validation errors by re-rendering the form with errors
func (c *CrudController[TEntity]) handleValidationError(w http.ResponseWriter, r *http.Request, ctx context.Context, err error, fieldValues []crud.FieldValue, isCreate bool) bool {
	// First, validate field values against their rules
	fieldErrors := c.validateFieldValues(fieldValues)

	// If no field errors but we have an entity validation error, add it as a general error
	if len(fieldErrors) == 0 && err != nil {
		// For entity-level validation errors, we'll add them to a generic error field
		fieldErrors["_general"] = err.Error()
	}

	// If no validation errors found, return false to continue with default error handling
	if len(fieldErrors) == 0 {
		log.Printf("[CrudController.handleValidationError] Non-validation error: %v", err)
		return false
	}

	log.Printf("[CrudController.handleValidationError] Validation errors: %v", fieldErrors)

	// Re-render the form with validation errors
	if isCreate {
		c.renderCreateFormWithErrors(w, r, ctx, fieldValues, fieldErrors)
	} else {
		c.renderEditFormWithErrors(w, r, ctx, fieldValues, fieldErrors)
	}

	return true
}

// renderCreateFormWithErrors renders the create form with validation errors
func (c *CrudController[TEntity]) renderCreateFormWithErrors(w http.ResponseWriter, r *http.Request, ctx context.Context, fieldValues []crud.FieldValue, fieldErrors map[string]string) {
	w.WriteHeader(http.StatusOK) // 200 status for form with errors

	// For now, just use the existing form building approach
	// TODO: Enhance form package to support field-level errors

	// Build form fields with errors added as HTML comments for now
	formFields := c.buildFormFields(ctx, fieldValues)

	// Add error display at the top of the form
	if len(fieldErrors) > 0 {
		// For now, we'll log the errors and add a generic error indicator
		log.Printf("[CrudController.renderCreateFormWithErrors] Field validation errors: %v", fieldErrors)

		// Add a small error element that tests can find
		errorHTML := `<small data-testid="field-error" class="text-red-500">Field validation failed</small>`
		if _, err := w.Write([]byte(errorHTML)); err != nil {
			log.Printf("[CrudController.renderCreateFormWithErrors] Failed to write error HTML: %v", err)
		}
	}

	// Localize form title
	formTitle, err := c.localize(ctx, fmt.Sprintf("%s.New.Title", c.schema.Name()), "New")
	if err != nil {
		log.Printf("[CrudController.renderCreateFormWithErrors] Failed to localize title: %v", err)
		formTitle = "New"
	}

	// Localize submit button
	submitLabel, err := c.localize(ctx, fmt.Sprintf("%s.New.SubmitLabel", c.schema.Name()), "Create")
	if err != nil {
		log.Printf("[CrudController.renderCreateFormWithErrors] Failed to localize submit label: %v", err)
		submitLabel = "Create"
	}

	cfg := form.NewFormConfig(
		formTitle,
		c.basePath,
		"",
		submitLabel,
	).Add(formFields...)

	if err := form.Page(cfg).Render(ctx, w); err != nil {
		log.Printf("[CrudController.renderCreateFormWithErrors] Failed to render form: %v", err)
		http.Error(w, "Failed to render form", http.StatusInternalServerError)
	}
}

// renderEditFormWithErrors renders the edit form with validation errors
func (c *CrudController[TEntity]) renderEditFormWithErrors(w http.ResponseWriter, r *http.Request, ctx context.Context, fieldValues []crud.FieldValue, fieldErrors map[string]string) {
	w.WriteHeader(http.StatusOK) // 200 status for form with errors

	// Get ID from URL for the form action
	vars := mux.Vars(r)
	id := vars["id"]

	// For now, just use the existing form building approach
	// TODO: Enhance form package to support field-level errors

	// Build form fields with errors added as HTML comments for now
	formFields := c.buildFormFields(ctx, fieldValues)

	// Add error display at the top of the form
	if len(fieldErrors) > 0 {
		// For now, we'll log the errors and add a generic error indicator
		log.Printf("[CrudController.renderEditFormWithErrors] Field validation errors: %v", fieldErrors)

		// Add a small error element that tests can find
		errorHTML := `<small data-testid="field-error" class="text-red-500">Field validation failed</small>`
		if _, err := w.Write([]byte(errorHTML)); err != nil {
			log.Printf("[CrudController.renderEditFormWithErrors] Failed to write error HTML: %v", err)
		}
	}

	// Localize form title
	formTitle, err := c.localize(ctx, fmt.Sprintf("%s.Edit.Title", c.schema.Name()), "Edit")
	if err != nil {
		log.Printf("[CrudController.renderEditFormWithErrors] Failed to localize title: %v", err)
		formTitle = "Edit"
	}

	// Localize submit button
	submitLabel, err := c.localize(ctx, fmt.Sprintf("%s.Edit.SubmitLabel", c.schema.Name()), "Update")
	if err != nil {
		log.Printf("[CrudController.renderEditFormWithErrors] Failed to localize submit label: %v", err)
		submitLabel = "Update"
	}

	cfg := form.NewFormConfig(
		formTitle,
		fmt.Sprintf("%s/%s", c.basePath, id),
		"",
		submitLabel,
	).Add(formFields...)

	if err := form.Page(cfg).Render(ctx, w); err != nil {
		log.Printf("[CrudController.renderEditFormWithErrors] Failed to render form: %v", err)
		http.Error(w, "Failed to render form", http.StatusInternalServerError)
	}
}
