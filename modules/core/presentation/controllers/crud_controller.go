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
	"html"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/components/base/dialog"
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
	router.HandleFunc("/{id}/view", c.GetView).Methods(http.MethodGet)

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
			c.formFields = append(c.formFields, f)
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
	case crud.UUIDFieldType:
		if uuidVal, err := uuid.Parse(id); err == nil {
			return uuidVal
		}
	}
	return id
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

	fetchUrl := fmt.Sprintf("%s/%v/view", c.basePath, primaryKey)
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
func (c *CrudController[TEntity]) buildRowActions(ctx context.Context, primaryKey any) []actions.ActionProps {
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

func (c *CrudController[TEntity]) GetView(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

	// Create field value for the ID
	idFieldValue := c.primaryKeyField.Value(c.parseIDValue(id))

	// Fetch entity
	entity, err := c.service.Get(ctx, idFieldValue)
	if err != nil {
		log.Printf("[CrudController.GetView] Failed to get entity %s: %v", id, err)
		errorMsg, _ := c.localize(ctx, errEntityNotFound, "Entity not found")
		http.Error(w, errorMsg, http.StatusNotFound)
		return
	}

	// Convert entity to field values
	fieldValues, err := c.schema.Mapper().ToFieldValues(ctx, entity)
	if err != nil {
		log.Printf("[CrudController.GetView] Failed to map entity: %v", err)
		errorMsg, _ := c.localize(ctx, errInternalServer, "Internal server error")
		http.Error(w, errorMsg, http.StatusInternalServerError)
		return
	}

	// Get entity title
	titleText, _ := c.localize(ctx, fmt.Sprintf("%s.View.Title", c.schema.Name()), c.schema.Name())

	// Create view content
	viewContent := c.buildViewContent(ctx, fieldValues)

	// Create wrapper component that renders drawer with content
	drawerComponent := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		// Generate unique ID for this drawer instance
		drawerID := fmt.Sprintf("drawer-%d", time.Now().UnixNano())

		// Write wrapper div that will be removed when drawer closes
		fmt.Fprintf(w, `<div id="%s">`, drawerID)

		// Create drawer component
		component := dialog.StdViewDrawer(dialog.StdDrawerProps{
			ID:     drawerID + "-dialog",
			Title:  titleText,
			Action: "open-view-drawer",
			Open:   true,
			Attrs: templ.Attributes{
				"@closing": fmt.Sprintf("window.history.pushState({}, '', '%s')", c.basePath),
				"@closed":  fmt.Sprintf("document.getElementById('%s').remove()", drawerID),
			},
		})

		// Render drawer with content
		if err := component.Render(templ.WithChildren(ctx, viewContent), w); err != nil {
			return err
		}

		// Close wrapper div
		fmt.Fprintf(w, `</div>`)

		return nil
	})

	if err := drawerComponent.Render(ctx, w); err != nil {
		log.Printf("[CrudController.GetView] Failed to render view: %v", err)
		errorMsg, _ := c.localize(ctx, errFailedToRender, "Failed to render view")
		http.Error(w, errorMsg, http.StatusInternalServerError)
	}
}

// buildViewContent creates the content for displaying entity details
func (c *CrudController[TEntity]) buildViewContent(ctx context.Context, fieldValues []crud.FieldValue) templ.Component {
	return templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		// Create field value map
		fieldValueMap := make(map[string]crud.FieldValue, len(fieldValues))
		var primaryKey any
		for _, fv := range fieldValues {
			fieldValueMap[fv.Field().Name()] = fv
			if fv.Field().Key() {
				primaryKey = fv.Value()
			}
		}

		// Start the content container
		fmt.Fprintf(w, `<div class="p-6 space-y-4">`)

		// Add field details
		fmt.Fprintf(w, `<dl class="divide-y divide-gray-100">`)

		for _, field := range c.visibleFields {
			if fv, exists := fieldValueMap[field.Name()]; exists {
				// Localize field label
				fieldLabel, err := c.localize(ctx, fmt.Sprintf("%s.Fields.%s", c.schema.Name(), field.Name()), field.Name())
				if err != nil {
					fieldLabel = field.Name()
				}

				fmt.Fprintf(w, `<div class="py-3 sm:grid sm:grid-cols-3 sm:gap-4">`)
				fmt.Fprintf(w, `<dt class="text-sm font-medium text-gray-900">%s</dt>`, html.EscapeString(fieldLabel))
				fmt.Fprintf(w, `<dd class="mt-1 text-sm text-gray-700 sm:col-span-2 sm:mt-0">`)

				// Render field value
				if err := c.fieldValueToTableCell(ctx, field, fv).Render(ctx, w); err != nil {
					return err
				}

				fmt.Fprintf(w, `</dd></div>`)
			}
		}

		fmt.Fprintf(w, `</dl>`)

		// Add action buttons
		if c.enableEdit || c.enableDelete {
			fmt.Fprintf(w, `<div class="mt-6 flex gap-3">`)

			if c.enableEdit {
				editLabel, _ := c.localize(ctx, "Common.Edit", "Edit")
				fmt.Fprintf(w, `<a href="%s/%v/edit" class="btn btn-primary">%s</a>`,
					html.EscapeString(c.basePath), primaryKey, html.EscapeString(editLabel))
			}

			if c.enableDelete {
				deleteLabel, _ := c.localize(ctx, "Common.Delete", "Delete")
				fmt.Fprintf(w, `<button hx-delete="%s/%v" hx-confirm="%s" hx-target="closest dialog" hx-swap="outerHTML" class="btn btn-danger">%s</button>`,
					html.EscapeString(c.basePath), primaryKey,
					html.EscapeString("Are you sure you want to delete this item?"),
					html.EscapeString(deleteLabel))
			}

			fmt.Fprintf(w, `</div>`)
		}

		fmt.Fprintf(w, `</div>`)

		return nil
	})
}

func (c *CrudController[TEntity]) GetEdit(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	vars := mux.Vars(r)
	id := vars["id"]

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

		// Check if it's a validation error
		if c.handleValidationError(w, r, ctx, err, fieldValues, true) {
			return
		}

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
			fieldValues[i] = fv.Field().Value(c.parseIDValue(id))
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

		// Check if it's a validation error
		if c.handleValidationError(w, r, ctx, err, fieldValues, false) {
			return
		}

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
	if field.Hidden() || field.Key() {
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

// handleValidationError handles validation errors by re-rendering the form with errors
func (c *CrudController[TEntity]) handleValidationError(w http.ResponseWriter, r *http.Request, ctx context.Context, err error, fieldValues []crud.FieldValue, isCreate bool) bool {
	// For now, we'll just log the error and return false
	// In a real implementation, you would need to enhance the form package
	// to support error handling at the field level
	log.Printf("[CrudController.handleValidationError] Validation error: %v", err)

	// You could potentially enhance this by:
	// 1. Parsing the error to extract field-specific errors
	// 2. Creating a custom form renderer that includes errors
	// 3. Using HTMX to return partial form updates with errors

	return false
}
