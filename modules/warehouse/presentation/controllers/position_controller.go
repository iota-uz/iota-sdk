package controllers

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/templates/layouts"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/entities/unit"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/warehouse/presentation/mappers"
	positions2 "github.com/iota-uz/iota-sdk/modules/warehouse/presentation/templates/pages/positions"
	viewmodels2 "github.com/iota-uz/iota-sdk/modules/warehouse/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services/positionservice"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/sirupsen/logrus"
	"github.com/xuri/excelize/v2"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	importcomponents "github.com/iota-uz/iota-sdk/components/import"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/warehouse/domain/aggregates/position"
	"github.com/iota-uz/iota-sdk/modules/warehouse/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	importpkg "github.com/iota-uz/iota-sdk/pkg/import"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type PositionsController struct {
	app      application.Application
	basePath string
}

type PositionPaginatedResponse struct {
	Positions       []*viewmodels2.Position
	PaginationState *pagination.State
}

func NewPositionsController(app application.Application) application.Controller {
	return &PositionsController{
		app:      app,
		basePath: "/warehouse/positions",
	}
}

func (c *PositionsController) Key() string {
	return c.basePath
}

func (c *PositionsController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(c.app),
		middleware.Tabs(),
		middleware.ProvideLocalizer(c.app.Bundle()),
		middleware.NavItems(),
		middleware.WithPageContext(),
	}
	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)

	getRouter.HandleFunc("", di.H(c.List)).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", di.H(c.GetEdit)).Methods(http.MethodGet)
	getRouter.HandleFunc("/search", di.H(c.Search)).Methods(http.MethodGet)
	getRouter.HandleFunc("/new", di.H(c.GetNew)).Methods(http.MethodGet)
	getRouter.HandleFunc("/import", di.H(c.GetUpload)).Methods(http.MethodGet)
	getRouter.HandleFunc("/template.xlsx", di.H(c.DownloadTemplate)).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())

	setRouter.HandleFunc("", di.H(c.Create)).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", di.H(c.Update)).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", di.H(c.Delete)).Methods(http.MethodDelete)
	setRouter.HandleFunc("/import", di.H(c.HandleUpload)).Methods(http.MethodPost)
}

func (c *PositionsController) GetUpload(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	localizer *i18n.Localizer,
) {
	config := NewPositionImportConfigWithLocalizer(localizer)
	props := &importcomponents.ImportPageProps{
		Config: config,
		Errors: map[string]string{},
	}

	// Create the content component that uses namespaced context
	contentComponent := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
		// Create namespaced context for import content
		pageCtx := composables.UsePageCtx(ctx)
		namespacedPageCtx := pageCtx.Namespace("WarehousePositions.Import")
		namespacedCtx := composables.WithPageCtx(ctx, namespacedPageCtx)

		return importcomponents.ImportPageContent(props).Render(namespacedCtx, w)
	})

	// Create layout with content as children
	layoutProps := layouts.AuthenticatedProps{
		BaseProps: layouts.BaseProps{Title: props.Config.GetTitle()},
	}

	// Add the content component as children to the request context
	ctx := templ.WithChildren(r.Context(), contentComponent)

	// Render the layout with the children
	templ.Handler(layouts.Authenticated(layoutProps), templ.WithStreaming()).ServeHTTP(w, r.WithContext(ctx))
}

func (c *PositionsController) HandleUpload(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	localizer *i18n.Localizer,
	coreUploadService *coreservices.UploadService,
	positionService *positionservice.PositionService,
) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	dto := dtos.PositionsUploadDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uniLocalizer, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	config := NewPositionImportConfigWithLocalizer(localizer)

	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		props := &importcomponents.ImportPageProps{
			Config: config,
			Errors: errorsMap,
		}

		// Create a namespaced component for the content
		contentComponent := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			pageCtx := composables.UsePageCtx(ctx)
			namespacedPageCtx := pageCtx.Namespace("WarehousePositions.Import")
			namespacedCtx := composables.WithPageCtx(ctx, namespacedPageCtx)

			return importcomponents.ImportContent(props).Render(namespacedCtx, w)
		})

		templ.Handler(contentComponent, templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	// Create Excel processor and row handler
	uploadServiceAdapter := NewUploadServiceAdapter(coreUploadService)
	excelProcessor := importpkg.NewMultiErrorProcessor(
		uploadServiceAdapter,
		importpkg.NewExcelFileReader(),
	)
	rowHandler := NewPositionRowHandler(
		positionService,
		importpkg.NewDefaultErrorFactory(),
	)

	if err := excelProcessor.ProcessFileWithAllErrors(r.Context(), dto.FileID, rowHandler); err != nil {
		// Check if it's a validation errors collection
		if validationErrors, ok := err.(*importpkg.ValidationErrors); ok {
			// Create namespaced context for error localization
			pageCtx := composables.UsePageCtx(r.Context())
			namespacedPageCtx := pageCtx.Namespace("WarehousePositions.Import")

			// Localize all errors
			var localizedErrors []string
			for _, vErr := range validationErrors.Errors {
				var localizedError string
				switch e := vErr.(type) {
				case *importpkg.InvalidCellError:
					localizedError = namespacedPageCtx.T("Error.ERR_INVALID_CELL", map[string]interface{}{
						"Row": e.Row,
						"Col": e.Col,
					})
				case *importpkg.ValidationError:
					localizedError = namespacedPageCtx.T("Error.ERR_VALIDATION", map[string]interface{}{
						"Col":     e.Col,
						"Value":   e.Value,
						"RowNum":  e.RowNum,
						"Message": e.Message,
					})
				default:
					if baseErr, ok := vErr.(serrors.Base); ok {
						localizedError = baseErr.Localize(localizer)
					} else {
						localizedError = vErr.Error()
					}
				}
				localizedErrors = append(localizedErrors, localizedError)
			}

			// Join all errors into a single string with line breaks
			errorsMap := make(map[string]string)
			for i, err := range localizedErrors {
				errorsMap[fmt.Sprintf("error_%d", i)] = err
			}

			props := &importcomponents.ImportPageProps{
				Config: config,
				Errors: errorsMap,
			}

			// Create a namespaced component for the content
			contentComponent := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				pageCtx := composables.UsePageCtx(ctx)
				namespacedPageCtx := pageCtx.Namespace("WarehousePositions.Import")
				namespacedCtx := composables.WithPageCtx(ctx, namespacedPageCtx)

				return importcomponents.ImportContent(props).Render(namespacedCtx, w)
			})

			templ.Handler(contentComponent, templ.WithStreaming()).ServeHTTP(w, r)
			return
		}

		// Handle single errors for backward compatibility
		var vErr serrors.Base
		if errors.As(err, &vErr) {
			// Create namespaced context for error localization
			pageCtx := composables.UsePageCtx(r.Context())
			namespacedPageCtx := pageCtx.Namespace("WarehousePositions.Import")

			// Localize the error with namespace prefix
			var localizedError string
			switch e := vErr.(type) {
			case *importpkg.InvalidCellError:
				localizedError = namespacedPageCtx.T("Error.ERR_INVALID_CELL", map[string]interface{}{
					"Row": e.Row,
					"Col": e.Col,
				})
			case *importpkg.ValidationError:
				localizedError = namespacedPageCtx.T("Error.ERR_VALIDATION", map[string]interface{}{
					"Col":     e.Col,
					"Value":   e.Value,
					"RowNum":  e.RowNum,
					"Message": e.Message,
				})
			default:
				localizedError = vErr.Localize(localizer)
			}

			props := &importcomponents.ImportPageProps{
				Config: config,
				Errors: map[string]string{
					"validation": localizedError,
				},
			}

			// Create a namespaced component for the content
			contentComponent := templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
				pageCtx := composables.UsePageCtx(ctx)
				namespacedPageCtx := pageCtx.Namespace("WarehousePositions.Import")
				namespacedCtx := composables.WithPageCtx(ctx, namespacedPageCtx)

				return importcomponents.ImportContent(props).Render(namespacedCtx, w)
			})

			templ.Handler(contentComponent, templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) viewModelPositions(
	r *http.Request,
	positionService *positionservice.PositionService,
) (*PositionPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&position.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving query")
	}
	entities, err := positionService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving positions")
	}
	total, err := positionService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting positions")
	}
	return &PositionPaginatedResponse{
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
		Positions:       mapping.MapViewModels(entities, mappers.PositionToViewModel),
	}, nil
}

func (c *PositionsController) viewModelUnits(
	r *http.Request,
	unitService *services.UnitService,
) ([]*viewmodels2.Unit, error) {
	entities, err := unitService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving units")
	}
	return mapping.MapViewModels(entities, mappers.UnitToViewModel), nil
}

func (c *PositionsController) List(
	r *http.Request,
	w http.ResponseWriter,
	positionService *positionservice.PositionService,
	unitService *services.UnitService,
) {
	paginated, err := c.viewModelPositions(r, positionService)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	unitViewModels, err := c.viewModelUnits(r, unitService)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &positions2.IndexPageProps{
		Positions:       paginated.Positions,
		Units:           unitViewModels,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(positions2.PositionsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(positions2.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *PositionsController) GetEdit(
	r *http.Request,
	w http.ResponseWriter,
	positionService *positionservice.PositionService,
	unitService *services.UnitService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := positionService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving position", http.StatusInternalServerError)
		return
	}
	unitViewModels, err := c.viewModelUnits(r, unitService)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &positions2.EditPageProps{
		Position:  mappers.PositionToViewModel(entity),
		Units:     unitViewModels,
		Errors:    map[string]string{},
		SaveURL:   fmt.Sprintf("%s/%d", c.basePath, id),
		DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(positions2.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Search(
	r *http.Request,
	w http.ResponseWriter,
	positionService *positionservice.PositionService,
) {
	search := r.URL.Query().Get("q")
	entities, err := positionService.GetPaginated(r.Context(), &position.FindParams{
		Query: search,
		Field: "title",
		Limit: 10,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := mapping.MapViewModels(entities, func(pos *position.Position) *base.ComboboxOption {
		return &base.ComboboxOption{
			Value: strconv.FormatUint(uint64(pos.ID), 10),
			Label: pos.Title,
		}
	})
	templ.Handler(base.ComboboxOptions(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Update(
	r *http.Request,
	w http.ResponseWriter,
	positionService *positionservice.PositionService,
	unitService *services.UnitService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	dto := position.UpdateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	uniLocalizer, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		entity, err := positionService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving position", http.StatusInternalServerError)
			return
		}
		unitViewModels, err := c.viewModelUnits(r, unitService)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &positions2.EditPageProps{
			Position:  mappers.PositionToViewModel(entity),
			Units:     unitViewModels,
			Errors:    errorsMap,
			SaveURL:   fmt.Sprintf("%s/%d", c.basePath, id),
			DeleteURL: fmt.Sprintf("%s/%d", c.basePath, id),
		}
		templ.Handler(positions2.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	if err := positionService.Update(r.Context(), id, &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) GetNew(
	r *http.Request,
	w http.ResponseWriter,
	unitService *services.UnitService,
) {
	unitViewModels, err := c.viewModelUnits(r, unitService)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &positions2.CreatePageProps{
		Errors: map[string]string{},
		Position: mappers.PositionToViewModel(&position.Position{
			Unit: &unit.Unit{},
		}),
		SaveURL: c.basePath,
		Units:   unitViewModels,
	}
	templ.Handler(positions2.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PositionsController) Create(
	r *http.Request,
	w http.ResponseWriter,
	positionService *positionservice.PositionService,
) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := position.CreateDTO{}
	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	uniLocalizer, err := intl.UseUniLocalizer(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := dto.Ok(uniLocalizer); !ok {
		entity, err := dto.ToEntity()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &positions2.CreatePageProps{
			Errors:   errorsMap,
			Position: mappers.PositionToViewModel(entity),
		}
		templ.Handler(positions2.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if _, err := positionService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) Delete(
	r *http.Request,
	w http.ResponseWriter,
	positionService *positionservice.PositionService,
) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if _, err := positionService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *PositionsController) DownloadTemplate(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	localizer *i18n.Localizer,
) {

	// Set response headers for Excel file download
	filename := fmt.Sprintf("warehouse_positions_template_%s.xlsx", time.Now().Format("20060102_150405"))
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=\"%s\"", filename))

	// Create Excel file using excelize
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			log.Printf("Error closing Excel file: %v", err)
		}
	}()

	// Helper function to translate
	t := func(key string) string {
		translated, err := localizer.Localize(&i18n.LocalizeConfig{MessageID: key})
		if err == nil {
			return translated
		}
		return key
	}

	// Set headers based on the import configuration
	headers := []string{
		t("WarehousePositions.Import.Example.ItemName"),
		t("WarehousePositions.Import.Example.ItemCode"),
		t("WarehousePositions.Import.Example.Unit"),
		t("WarehousePositions.Import.Example.Quantity"),
	}

	// Set header row
	for i, header := range headers {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue("Sheet1", cell, header)
	}

	// Add sample data (matching the config's ExampleRows)
	sampleData := [][]interface{}{
		{"Дрель Молоток N.C.V (900W)", "30232478", "шт", "1"},
		{"Дрель Ударная (650W)", "30232477", "шт", "1"},
		{"Комплект плакатов по предмету \"Математика\", 40 листов", "00017492", "компл", "7"},
		{"Комплект плакатов цветных по \"Технике безопасности\" (500x700мм, 5 листов) на туркменском", "00028544", "компл", "127"},
	}

	// Add sample data to the Excel file
	for rowIdx, row := range sampleData {
		for colIdx, value := range row {
			cell, _ := excelize.CoordinatesToCellName(colIdx+1, rowIdx+2)
			f.SetCellValue("Sheet1", cell, value)
		}
	}

	// Style the header row
	headerStyle, err := f.NewStyle(&excelize.Style{
		Fill: excelize.Fill{
			Type:    "pattern",
			Color:   []string{"#E5E7EB"},
			Pattern: 1,
		},
		Font: &excelize.Font{
			Bold: true,
		},
		Border: []excelize.Border{
			{Type: "left", Color: "000000", Style: 1},
			{Type: "top", Color: "000000", Style: 1},
			{Type: "bottom", Color: "000000", Style: 1},
			{Type: "right", Color: "000000", Style: 1},
		},
	})
	if err == nil {
		for i := range headers {
			cell, _ := excelize.CoordinatesToCellName(i+1, 1)
			f.SetCellStyle("Sheet1", cell, cell, headerStyle)
		}
	}

	// Auto-fit columns
	for i := range headers {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth("Sheet1", col, col, 20)
	}

	// Write to response
	if err := f.Write(w); err != nil {
		http.Error(w, "Failed to generate Excel file", http.StatusInternalServerError)
		return
	}
}
