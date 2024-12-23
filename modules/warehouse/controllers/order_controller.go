package controllers

import (
	"context"
	"fmt"
	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/components/base/pagination"
	"github.com/iota-agency/iota-sdk/modules/warehouse/controllers/dtos"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/product"
	"github.com/iota-agency/iota-sdk/modules/warehouse/presentation/mappers"
	"github.com/iota-agency/iota-sdk/modules/warehouse/presentation/templates/pages/orders"
	orderin "github.com/iota-agency/iota-sdk/modules/warehouse/presentation/templates/pages/orders/in"
	"github.com/iota-agency/iota-sdk/modules/warehouse/presentation/templates/pages/orders/out"
	"github.com/iota-agency/iota-sdk/modules/warehouse/presentation/viewmodels"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/order_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/fp"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"net/http"
	"strconv"
)

var (
	OrdersToViewModels = fp.Map[order.Order, *viewmodels.Order](func(o order.Order) *viewmodels.Order {
		return mappers.OrderToViewModel(o, map[uint]int{})
	})
)

type OrderItem struct {
	PositionID    uint
	PositionTitle string
	Barcode       string
	Unit          string
	InStock       uint
	Quantity      uint
	Error         string
}

func OrderOutItemToViewModel(item OrderItem) orderout.OrderItem {
	return orderout.OrderItem{
		PositionID:    strconv.FormatUint(uint64(item.PositionID), 10),
		PositionTitle: item.PositionTitle,
		Barcode:       item.Barcode,
		Unit:          item.Unit,
		InStock:       strconv.FormatUint(uint64(item.InStock), 10),
		Quantity:      strconv.FormatUint(uint64(item.Quantity), 10),
		Error:         item.Error,
	}
}

func OrderInItemToViewModel(item OrderItem) orderin.OrderItem {
	return orderin.OrderItem{
		PositionID:    strconv.FormatUint(uint64(item.PositionID), 10),
		PositionTitle: item.PositionTitle,
		Barcode:       item.Barcode,
		Unit:          item.Unit,
		InStock:       strconv.FormatUint(uint64(item.InStock), 10),
		Quantity:      strconv.FormatUint(uint64(item.Quantity), 10),
		Error:         item.Error,
	}
}

type OrdersController struct {
	app             application.Application
	orderService    *orderservice.OrderService
	positionService *positionservice.PositionService
	productService  *productservice.ProductService
	basePath        string
}

type OrderPaginatedResponse struct {
	Orders          []*viewmodels.Order
	PaginationState *pagination.State
}

func NewOrdersController(app application.Application) application.Controller {
	return &OrdersController{
		app:             app,
		orderService:    app.Service(orderservice.OrderService{}).(*orderservice.OrderService),
		positionService: app.Service(positionservice.PositionService{}).(*positionservice.PositionService),
		productService:  app.Service(productservice.ProductService{}).(*productservice.ProductService),
		basePath:        "/warehouse/orders",
	}
}

func (c *OrdersController) Key() string {
	return c.basePath
}

func (c *OrdersController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(),
	}

	getRouter := r.PathPrefix(c.basePath).Subrouter()
	getRouter.Use(commonMiddleware...)
	getRouter.HandleFunc("", c.List).Methods(http.MethodGet)
	getRouter.HandleFunc("/{id:[0-9]+}", c.ViewOrder).Methods(http.MethodGet)
	getRouter.HandleFunc("/in/new", c.NewInOrder).Methods(http.MethodGet)
	getRouter.HandleFunc("/out/new", c.NewOutOrder).Methods(http.MethodGet)

	setRouter := r.PathPrefix(c.basePath).Subrouter()
	setRouter.Use(commonMiddleware...)
	setRouter.Use(middleware.WithTransaction())
	setRouter.HandleFunc("/in", c.CreateInOrder).Methods(http.MethodPost)
	setRouter.HandleFunc("/out", c.CreateOutOrder).Methods(http.MethodPost)
	setRouter.HandleFunc("/items", c.OrderItems).Methods(http.MethodPost)
	setRouter.HandleFunc("/{id:[0-9]+}", c.Delete).Methods(http.MethodDelete)
}

func (c *OrdersController) viewModelOrders(r *http.Request) (*OrderPaginatedResponse, error) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&order.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
		SortBy: []string{"created_at desc"},
	}, r)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving orders")
	}
	entities, err := c.orderService.GetPaginated(r.Context(), params)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving orders")
	}
	total, err := c.orderService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting orders")
	}
	return &OrderPaginatedResponse{
		PaginationState: pagination.New(c.basePath, paginationParams.Page, int(total), params.Limit),
		Orders:          OrdersToViewModels(entities),
	}, nil
}

func (c *OrdersController) List(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehouseOrders.List.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paginated, err := c.viewModelOrders(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &orders.IndexPageProps{
		PageContext:     pageCtx,
		Orders:          paginated.Orders,
		PaginationState: paginated.PaginationState,
	}
	if isHxRequest {
		templ.Handler(orders.OrdersTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(orders.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *OrdersController) ViewOrder(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	entity, err := c.orderService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	var status product.Status
	switch entity.Type() {
	case order.TypeIn:
		status = product.InDevelopment
	case order.TypeOut:
		status = product.InStock

	}
	countByPositionID := make(map[uint]int)
	for _, item := range entity.Items() {
		count, err := c.productService.CountInStock(r.Context(), &product.CountParams{
			PositionID: item.Position().ID,
			Status:     status,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		countByPositionID[item.Position().ID] = int(count)
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.View.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	viewModel := mappers.OrderToViewModel(entity, countByPositionID)
	props := &orders.ViewPageProps{
		PageContext: pageCtx,
		Order:       viewModel,
		DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(orders.View(props), templ.WithStreaming()).ServeHTTP(w, r)

}

func (c *OrdersController) NewInOrder(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &orderin.PageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		Items:       []orderin.OrderItem{},
		SaveURL:     fmt.Sprintf("%s/in", c.basePath),
		ItemsURL:    fmt.Sprintf("%s/items?status=%s", c.basePath, product.InDevelopment),
	}
	templ.Handler(orderin.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *OrdersController) NewOutOrder(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &orderout.PageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		SaveURL:     fmt.Sprintf("%s/out", c.basePath),
		ItemsURL:    fmt.Sprintf("%s/items?status=%s", c.basePath, product.InStock),
	}
	templ.Handler(orderout.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *OrdersController) CreateInOrder(w http.ResponseWriter, r *http.Request) {
	formDTO, err := composables.UseForm(&dtos.CreateOrderDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items, err := c.orderItems(r.Context(), formDTO, product.InDevelopment)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := formDTO.Ok(r.Context()); !ok {
		props := &orderin.FormProps{
			Errors: errorsMap,
			Items:  mapping.MapViewModels(items, OrderInItemToViewModel),
		}
		templ.Handler(orderin.Form(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	dto := order.CreateDTO{
		Type:       string(order.TypeIn),
		Status:     string(order.Pending),
		ProductIDs: []uint{},
	}
	var hasErrors bool
	for i, item := range items {
		quantity := int(formDTO.Quantity[item.PositionID])
		products, err := c.orderService.FindByPositionID(r.Context(), &product.FindByPositionParams{
			PositionID: item.PositionID,
			SortBy:     []string{"created_at asc"},
			Limit:      quantity,
			Status:     product.InDevelopment,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(products) < quantity {
			hasErrors = true
			items[i].Error = pageCtx.T("Errors.ERR_NOT_ENOUGH_PRODUCTS_IN_DEVELOPMENT")
		}
		for _, p := range products {
			dto.ProductIDs = append(dto.ProductIDs, p.ID)
		}
	}

	if hasErrors {
		props := &orderin.FormProps{
			Errors: map[string]string{},
			Items:  mapping.MapViewModels(items, OrderInItemToViewModel),
		}
		templ.Handler(orderin.Form(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.orderService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *OrdersController) CreateOutOrder(w http.ResponseWriter, r *http.Request) {
	formDTO, err := composables.UseForm(&dtos.CreateOrderDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	items, err := c.orderItems(r.Context(), formDTO, product.InStock)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if errorsMap, ok := formDTO.Ok(r.Context()); !ok {
		props := &orderout.FormProps{
			Errors: errorsMap,
			Items:  mapping.MapViewModels(items, OrderOutItemToViewModel),
		}
		templ.Handler(orderout.Form(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	dto := order.CreateDTO{
		Type:       string(order.TypeOut),
		Status:     string(order.Pending),
		ProductIDs: []uint{},
	}
	var hasErrors bool
	for i, item := range items {
		quantity := int(formDTO.Quantity[item.PositionID])
		products, err := c.orderService.FindByPositionID(r.Context(), &product.FindByPositionParams{
			PositionID: item.PositionID,
			SortBy:     []string{"created_at asc"},
			Limit:      quantity,
			Status:     product.InStock,
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(products) < quantity {
			hasErrors = true
			items[i].Error = pageCtx.T("Errors.ERR_NOT_ENOUGH_PRODUCTS_IN_STOCK")
		}
		for _, p := range products {
			dto.ProductIDs = append(dto.ProductIDs, p.ID)
		}
	}
	if hasErrors {
		props := &orderout.FormProps{
			Errors: map[string]string{},
			Items:  mapping.MapViewModels(items, OrderOutItemToViewModel),
		}
		templ.Handler(orderout.Form(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.orderService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *OrdersController) orderItems(ctx context.Context, dto *dtos.CreateOrderDTO, status product.Status) ([]OrderItem, error) {
	positionEntities, err := c.positionService.GetByIDs(ctx, dto.PositionIDs)
	if err != nil {
		return nil, err
	}

	items := make([]OrderItem, len(positionEntities))
	for i, position := range positionEntities {
		inStock, err := c.productService.CountInStock(ctx, &product.CountParams{
			PositionID: position.ID,
			Status:     status,
		})
		if err != nil {
			return nil, err
		}
		quantity, ok := dto.Quantity[position.ID]
		if !ok {
			quantity = 1
		}
		items[i] = OrderItem{
			PositionID:    position.ID,
			PositionTitle: position.Title,
			Barcode:       position.Barcode,
			Quantity:      quantity,
			Unit:          position.Unit.Title,
			InStock:       uint(inStock),
			Error:         "",
		}
	}
	return items, nil
}

func (c *OrdersController) OrderItems(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.CreateOrderDTO{}, r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	status, err := product.NewStatus(r.URL.Query().Get("status"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := c.orderItems(r.Context(), dto, status)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	viewModelItems := mapping.MapViewModels(items, OrderOutItemToViewModel)
	templ.Handler(orderout.OrderItemsTable(viewModelItems), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *OrdersController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if _, err := c.orderService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}
