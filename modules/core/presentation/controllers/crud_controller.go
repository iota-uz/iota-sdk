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
	"net/http"
	"net/url"
	"time"

	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/form"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
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
		enableEdit:   true,  // Enable by default
		enableDelete: true,  // Enable by default
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
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("/{id}", c.GetEdit).Methods(http.MethodGet)

	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id}", c.Delete).Methods(http.MethodDelete)
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
			if !f.Key() {
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

// getPrimaryKeyValue extracts primary key value from field values
func (c *CrudController[TEntity]) getPrimaryKeyValue(fieldValues []crud.FieldValue) (any, error) {
	for _, fv := range fieldValues {
		if fv.Field().Key() {
			return fv.Value(), nil
		}
	}
	return nil, fmt.Errorf("primary key not found")
}

// buildFieldValuesFromForm creates field values from form data
func (c *CrudController[TEntity]) buildFieldValuesFromForm(r *http.Request) ([]crud.FieldValue, error) {
	if err := r.ParseForm(); err != nil {
		return nil, fmt.Errorf("failed to parse form: %w", err)
	}

	fieldValues := make([]crud.FieldValue, 0, len(c.schema.Fields().Fields()))

	for _, f := range c.schema.Fields().Fields() {
		var value any

		if r.Form.Has(f.Name()) {
			formValue := r.Form.Get(f.Name())

			// Convert form value based on field type
			switch f.Type() {
			case crud.BoolFieldType:
				value = formValue == "on" || formValue == "true" || formValue == "1"
			case crud.IntFieldType:
				// Try to parse as int
				if formValue != "" {
					var intVal int64
					if _, err := fmt.Sscanf(formValue, "%d", &intVal); err == nil {
						value = int(intVal)
					} else {
						value = 0
					}
				} else {
					value = 0
				}
			case crud.FloatFieldType:
				// Try to parse as float
				if formValue != "" {
					var floatVal float64
					if _, err := fmt.Sscanf(formValue, "%f", &floatVal); err == nil {
						value = floatVal
					} else {
						value = 0.0
					}
				} else {
					value = 0.0
				}
			case crud.DateFieldType, crud.DateTimeFieldType, crud.TimeFieldType:
				// Parse time values
				if formValue != "" {
					parsedTime, err := time.Parse(time.RFC3339, formValue)
					if err != nil {
						// Try common HTML5 formats
						for _, format := range []string{"2006-01-02", "2006-01-02T15:04", "15:04"} {
							if parsedTime, err = time.Parse(format, formValue); err == nil {
								break
							}
						}
					}
					if err == nil {
						value = parsedTime
					} else {
						value = time.Time{}
					}
				} else {
					value = time.Time{}
				}
			case crud.UUIDFieldType:
				// Parse UUID
				if formValue != "" {
					if uid, err := uuid.Parse(formValue); err == nil {
						value = uid
					} else {
						value = uuid.Nil
					}
				} else {
					value = uuid.Nil
				}
			default:
				value = formValue
			}
		} else {
			// Special handling for checkboxes (bool fields)
			if f.Type() == crud.BoolFieldType {
				value = false
			} else {
				value = f.InitialValue()
			}
		}

		fieldValues = append(fieldValues, f.Value(value))
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

	// Fetch entities
	entities, err := c.service.List(ctx, params)
	if err != nil {
		log.Printf("[CrudController.List] Failed to list entities: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRetrieve, "Failed to retrieve data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Get total count for infinity scroll
	// We need to create a separate params for count to include the search query
	countParams := &crud.FindParams{
		Query: params.Query, // Include search query in count
	}
	totalCount, err := c.service.Count(ctx, countParams)
	if err != nil {
		log.Printf("[CrudController.List] Failed to count entities: %v", err)
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

		// Add create button
		createLabel, err := c.localize(ctx, fmt.Sprintf("%s.List.New", c.schema.Name()), "New")
		if err != nil {
			createLabel = "New"
		}

		// Create action configuration
		createAction := actions.CreateAction(createLabel, fmt.Sprintf("%s/new", c.basePath))
		cfg.AddActions(actions.RenderAction(createAction))
		
		// Optionally add export button
		// exportLabel, _ := c.localize(ctx, fmt.Sprintf("%s.List.Export", c.schema.Name()), "Export")
		// exportAction := actions.ExportAction(exportLabel, fmt.Sprintf("%s/export", c.basePath))
		// cfg.AddActions(actions.Action(exportAction))
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

	// Add row actions if enabled
	if c.enableEdit || c.enableDelete {
		var rowActions []actions.ActionProps
		
		if c.enableEdit {
			editAction := actions.EditAction(fmt.Sprintf("%s/%v", c.basePath, primaryKey))
			rowActions = append(rowActions, editAction)
		}
		
		if c.enableDelete {
			deleteAction := actions.DeleteAction(fmt.Sprintf("%s/%v", c.basePath, primaryKey))
			rowActions = append(rowActions, deleteAction)
		}
		
		components = append(components, actions.RenderRowActions(rowActions...))
	}

	fetchUrl := fmt.Sprintf("/%s/%v", c.basePath, primaryKey)
	return table.Row(components...).ApplyOpts(table.WithDrawer(fetchUrl)), nil
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

	// Build form fields using cached fields
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
		formField := c.fieldToFormField(ctx, f)
		if formField == nil {
			continue
		}

		// Set current value if available
		if fieldValueMap != nil {
			if fv, exists := fieldValueMap[f.Name()]; exists && !fv.IsZero() {
				// TODO: Implement setFormFieldValue properly
				formField = c.setFormFieldValue(formField, fv)
			}
		}

		formFields = append(formFields, formField)
	}

	return formFields
}

func (c *CrudController[TEntity]) GetEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Create field value for the ID
	idFieldValue := c.primaryKeyField.Value(id)

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

// setFormFieldValue sets the value of a form field based on field value
func (c *CrudController[TEntity]) setFormFieldValue(formField form.Field, fv crud.FieldValue) form.Field {
	// This is a simplified version - in real implementation,
	// you would need to recreate the field with the value
	// based on the specific field type and builder pattern
	return formField
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

	// Convert to entity
	entity, err := c.schema.Mapper().ToEntity(ctx, fieldValues)
	if err != nil {
		log.Printf("[CrudController.Create] Failed to map to entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid form data")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Save entity
	savedEntity, err := c.service.Save(ctx, entity)
	if err != nil {
		log.Printf("[CrudController.Create] Failed to save entity: %v", err)

		// TODO: Return form with validation errors
		errorMsg, _ := c.localize(ctx, errFailedToSave, "Failed to save data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Get primary key for redirect
	savedFieldValues, err := c.schema.Mapper().ToFieldValues(ctx, savedEntity)
	if err != nil {
		log.Printf("[CrudController.Create] Failed to map saved entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInternalServer, "Internal server error")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	primaryKey, err := c.getPrimaryKeyValue(savedFieldValues)
	if err != nil {
		log.Printf("[CrudController.Create] %v", err)
		errorMsg, _ := c.localize(ctx, errInternalServer, "Internal server error")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Handle redirect
	redirectUrl := fmt.Sprintf("%s/%v", c.basePath, primaryKey)
	if htmx.IsHxRequest(r) {
		w.Header().Set("HX-Redirect", redirectUrl)
	} else {
		http.Redirect(w, r, c.basePath, http.StatusSeeOther)
	}
}

func (c *CrudController[TEntity]) Update(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Build field values from form
	fieldValues, err := c.buildFieldValuesFromForm(r)
	if err != nil {
		log.Printf("[CrudController.Update] Failed to parse form: %v", err)
		errorMsg, _ := c.localize(ctx, errInvalidFormData, "Invalid form data")
		http.Error(w, errorMsg, http.StatusBadRequest)
		return
	}

	// Set the ID in field values
	for i, fv := range fieldValues {
		if fv.Field().Key() {
			fieldValues[i] = fv.Field().Value(id)
			break
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

	// Save the updated entity
	updatedEntity, err := c.service.Save(ctx, entity)
	if err != nil {
		log.Printf("[CrudController.Update] Failed to update entity %s: %v", id, err)

		// TODO: Return form with validation errors
		errorMsg, _ := c.localize(ctx, errFailedToUpdate, "Failed to update data")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Get primary key for redirect
	updatedFieldValues, err := c.schema.Mapper().ToFieldValues(ctx, updatedEntity)
	if err != nil {
		log.Printf("[CrudController.Update] Failed to map updated entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInternalServer, "Internal server error")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	primaryKey, err := c.getPrimaryKeyValue(updatedFieldValues)
	if err != nil {
		log.Printf("[CrudController.Update] %v", err)
		primaryKey = id // Fallback to original ID
	}

	// Handle redirect
	redirectUrl := fmt.Sprintf("%s/%v", c.basePath, primaryKey)
	if htmx.IsHxRequest(r) {
		w.Header().Set("HX-Redirect", redirectUrl)
	} else {
		http.Redirect(w, r, redirectUrl, http.StatusSeeOther)
	}
}

func (c *CrudController[TEntity]) Delete(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Create field value for the ID
	idFieldValue := c.primaryKeyField.Value(id)

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

func (c *CrudController[TEntity]) fieldToFormField(ctx context.Context, field crud.Field) form.Field {
	if field.Hidden() || field.Key() {
		return nil
	}

	// Localize field label
	fieldLabel, err := c.localize(ctx, fmt.Sprintf("%s.Fields.%s", c.schema.Name(), field.Name()), field.Name())
	if err != nil {
		fieldLabel = field.Name()
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

			// Check if field has any rules (likely means it's required)
			if len(field.Rules()) > 0 {
				textareaBuilder = textareaBuilder.Required()
			}

			if field.InitialValue() != nil {
				textareaBuilder = textareaBuilder.Default(field.InitialValue().(string))
			}

			return textareaBuilder.Build()
		}

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			builder = builder.Default(field.InitialValue().(string))
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

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			builder = builder.Default(float64(field.InitialValue().(int)))
		}

		return builder.Build()

	case crud.BoolFieldType:
		builder := form.Checkbox(field.Name(), fieldLabel)

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			builder = builder.Default(field.InitialValue().(bool))
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

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			builder = builder.Default(field.InitialValue().(float64))
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

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			builder = builder.Default(field.InitialValue().(time.Time))
		}

		return builder.Build()

	case crud.TimeFieldType:
		builder := form.Time(field.Name(), fieldLabel)

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			t := field.InitialValue().(time.Time)
			builder = builder.Default(t.Format("15:04"))
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

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			builder = builder.Default(field.InitialValue().(time.Time))
		}

		return builder.Build()

	case crud.UUIDFieldType:
		builder := form.Text(field.Name(), fieldLabel)

		if field.Readonly() {
			builder = builder.Attrs(templ.Attributes{"readonly": true})
		}

		// Check if field has any rules (likely means it's required)
		if len(field.Rules()) > 0 {
			builder = builder.Required()
		}

		if field.InitialValue() != nil {
			builder = builder.Default(field.InitialValue().(string))
		}

		return builder.Build()

	default:
		return form.Text(field.Name(), field.Name()).Build()
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
