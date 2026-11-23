// Package controllers provides this package.
package controllers

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/base/pagination"
	"github.com/iota-uz/iota-sdk/components/scaffold/filterbuilder"
	coremappers "github.com/iota-uz/iota-sdk/modules/core/presentation/mappers"
	coreviewmodels "github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	coreservices "github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/entities/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	inventoryTemplates "github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/inventory"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/filterq"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/money"
	"github.com/iota-uz/iota-sdk/pkg/repo"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
	"github.com/iota-uz/iota-sdk/pkg/shared"
)

type InventoryController struct {
	inventoryService *services.InventoryService
	currencyService  *coreservices.CurrencyService
	basePath         string
}

type InventoryPaginatedResponse struct {
	Items           []*viewmodels.Inventory
	PaginationState *pagination.State
	Filters         filterbuilder.Props
}

func NewInventoryController(inventoryService *services.InventoryService, currencyService *coreservices.CurrencyService) application.Controller {
	return &InventoryController{
		inventoryService: inventoryService,
		currencyService:  currencyService,
		basePath:         "/finance/inventory",
	}
}

func (c *InventoryController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("finance.inventory", 0, application.Route("", c.basePath)).
		WithNav(application.NavNode{
			ID:       "finance.inventory",
			Parent:   "finance",
			TitleKey: "NavigationLinks.Inventory",
			Path:     c.basePath,
			Order:    60,
			Actions: []application.NavAction{{
				ID:       "finance.inventory.new",
				TitleKey: "Inventory.List.New",
				Path:     c.basePath + "/new",
			}},
		})
}

func (c *InventoryController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)

	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *InventoryController) viewModelCurrencies(r *http.Request) ([]*coreviewmodels.Currency, error) {
	currencies, err := c.currencyService.GetAll(r.Context())
	if err != nil {
		return nil, err
	}
	return mapping.MapViewModels(currencies, coremappers.CurrencyToViewModel), nil
}

// Filter chip field keys of the inventory list.
const (
	inventoryFilterCurrency  = "currency"
	inventoryFilterPrice     = "price"
	inventoryFilterQuantity  = "quantity"
	inventoryFilterCreatedAt = "created_at"
)

// filterRegistry describes the chips of the inventory list: the currency of
// the price, the price in major units, the quantity and the creation date.
func (c *InventoryController) filterRegistry(r *http.Request) *filterbuilder.Registry {
	pageCtx := composables.UsePageCtx(r.Context())
	var currencies []filterbuilder.Option
	if list, err := c.viewModelCurrencies(r); err == nil {
		for _, item := range list {
			currencies = append(currencies, filterbuilder.Opt(item.Code, item.Code))
		}
	}
	return filterbuilder.NewRegistry(
		filterbuilder.FieldDef{
			Key: inventoryFilterCurrency, Type: filterq.FieldTypeReference,
			Label: pageCtx.T("Inventory.List.Filters.Currency"), Options: currencies,
		},
		filterbuilder.FieldDef{
			Key: inventoryFilterPrice, Type: filterq.FieldTypeNumber,
			Label: pageCtx.T("Inventory.List.Price"),
		},
		filterbuilder.FieldDef{
			Key: inventoryFilterQuantity, Type: filterq.FieldTypeNumber,
			Label: pageCtx.T("Inventory.List.Quantity"),
		},
		filterbuilder.FieldDef{
			Key: inventoryFilterCreatedAt, Type: filterq.FieldTypeDate,
			Label: pageCtx.T("CreatedAt"),
		},
	)
}

func (c *InventoryController) viewModelInventory(r *http.Request) (*InventoryPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	registry := c.filterRegistry(r)
	// Links made by the former created-at select still work.
	conditions := registry.Decode(filterq.LegacyDateRange(r.URL.Query(), "CreatedAt.From", "CreatedAt.To", inventoryFilterCreatedAt))
	params := &inventory.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: inventory.SortBy{
			Fields: []repo.SortByField[inventory.Field]{
				{
					Field:     inventory.CreatedAtField,
					Ascending: false,
				},
			},
		},
		Search: strings.TrimSpace(r.URL.Query().Get("Search")),
		Filters: filterq.RepoFilters(conditions, registry.Schema(), map[string]filterq.RepoField[inventory.Field]{
			inventoryFilterCurrency: {Column: inventory.CurrencyField},
			// Prices are stored in minor units of the row's currency; the chip
			// takes major units.
			inventoryFilterPrice:     {Column: inventory.PriceField, Expr: func(col string) string { return majorUnits(col, "i.currency_id") }},
			inventoryFilterQuantity:  {Column: inventory.QuantityField},
			inventoryFilterCreatedAt: {Column: inventory.CreatedAtField},
		}, time.Now()),
	}

	items, err := c.inventoryService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, err
	}

	total, err := c.inventoryService.Count(r.Context(), params)
	if err != nil {
		return nil, err
	}

	viewItems := mapping.MapViewModels(items, mappers.InventoryToViewModel)

	return &InventoryPaginatedResponse{
		Items: viewItems,
		// Page links keep the search and chips, or page 2 would drop them.
		PaginationState: pagination.NewWithQuery(c.basePath, r.URL.Query(), paginationParams.Page, int(total), params.Limit),
		Filters: filterbuilder.Props{
			Registry: registry, Filters: conditions, ID: "inventory-filters", ShowResetAlways: true,
		},
	}, nil
}

func (c *InventoryController) List(w http.ResponseWriter, r *http.Request) {
	paginated, err := c.viewModelInventory(r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	props := &inventoryTemplates.IndexPageProps{
		Inventory:       paginated.Items,
		PaginationState: paginated.PaginationState,
		Filters:         paginated.Filters,
	}
	// A history restore (cache miss on Back) needs the whole document.
	if !htmx.WantsFullPage(r) {
		templ.Handler(inventoryTemplates.InventoryTableWithChips(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(inventoryTemplates.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *InventoryController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	entity, err := c.inventoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving inventory item", http.StatusInternalServerError)
		return
	}
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	props := &inventoryTemplates.EditPageProps{
		Inventory:  mappers.InventoryToViewModel(entity),
		Currencies: currencies,
		Errors:     map[string]string{},
		PostPath:   fmt.Sprintf("%s/%s", c.basePath, id.String()),
		DeletePath: fmt.Sprintf("%s/%s", c.basePath, id.String()),
	}
	templ.Handler(inventoryTemplates.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	// Check if item exists first
	_, err = c.inventoryService.GetByID(r.Context(), id)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	if err := c.inventoryService.Delete(r.Context(), id); err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	dto, err := composables.UseForm(&dtos.InventoryUpdateDTO{}, r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		entity, err := c.inventoryService.GetByID(r.Context(), id)
		if err != nil {
			http.Error(w, "Error retrieving inventory item", http.StatusInternalServerError)
			return
		}
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
			return
		}
		props := &inventoryTemplates.EditPageProps{
			Inventory:  mappers.InventoryToViewModel(entity),
			Currencies: currencies,
			Errors:     errorsMap,
			PostPath:   fmt.Sprintf("%s/%s", c.basePath, id.String()),
			DeletePath: fmt.Sprintf("%s/%s", c.basePath, id.String()),
		}
		templ.Handler(inventoryTemplates.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	existing, err := c.inventoryService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving inventory item", http.StatusInternalServerError)
		return
	}

	entity, err := dto.Apply(existing)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	if _, err := c.inventoryService.Update(r.Context(), entity); err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *InventoryController) GetNew(w http.ResponseWriter, r *http.Request) {
	currencies, err := c.viewModelCurrencies(r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}
	props := &inventoryTemplates.CreatePageProps{
		Currencies: currencies,
		Errors:     map[string]string{},
		Inventory:  &viewmodels.Inventory{},
		PostPath:   c.basePath,
	}
	templ.Handler(inventoryTemplates.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *InventoryController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.InventoryCreateDTO{}, r)
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusBadRequest, nil)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		currencies, err := c.viewModelCurrencies(r)
		if err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
			return
		}
		entity, err := dto.ToEntity()
		if err != nil {
			serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
			return
		}
		props := &inventoryTemplates.CreatePageProps{
			Currencies: currencies,
			Errors:     errorsMap,
			Inventory:  mappers.InventoryToViewModel(entity),
			PostPath:   c.basePath,
		}
		templ.Handler(inventoryTemplates.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	entity, err := dto.ToEntity()
	if err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	if _, err := c.inventoryService.Create(r.Context(), entity); err != nil {
		serrorhttp.WriteTextContext(r.Context(), w, err, http.StatusInternalServerError, nil)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

// majorUnits renders amount (minor units) in major units of the currency in
// currency: a yen price has no minor digits, a dinar has three. Only the
// registered currencies whose fraction is not 2 get a case; codes come from
// the money registry, never from the request.
func majorUnits(amount, currency string) string {
	var b strings.Builder
	b.WriteString("(" + amount + ")::numeric / CASE " + currency)
	for _, c := range money.RegisteredCurrencies() {
		if c.Fraction == 2 || !isCurrencyCode(c.Code) {
			continue
		}
		b.WriteString(" WHEN '" + c.Code + "' THEN " + strconv.FormatFloat(math.Pow10(c.Fraction), 'f', 0, 64))
	}
	b.WriteString(" ELSE 100 END")
	return b.String()
}

func isCurrencyCode(code string) bool {
	if code == "" {
		return false
	}
	for _, r := range code {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return false
		}
	}
	return true
}
