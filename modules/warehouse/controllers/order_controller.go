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
	"github.com/iota-agency/iota-sdk/modules/warehouse/mappers"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/orders"
	orderin "github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/orders/in"
	orderout "github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/orders/out"
	"github.com/iota-agency/iota-sdk/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
	"net/http"
	"strconv"
)

type OrdersController struct {
	app             application.Application
	orderService    *services.OrderService
	positionService *position_service.PositionService
	productService  *product_service.ProductService
	basePath        string
}

type OrderPaginatedResponse struct {
	Orders          []*viewmodels.Order
	PaginationState *pagination.State
}

func NewOrdersController(app application.Application) application.Controller {
	return &OrdersController{
		app:             app,
		orderService:    app.Service(services.OrderService{}).(*services.OrderService),
		positionService: app.Service(position_service.PositionService{}).(*position_service.PositionService),
		productService:  app.Service(product_service.ProductService{}).(*product_service.ProductService),
		basePath:        "/warehouse/orders",
	}
}

func (c *OrdersController) Register(r *mux.Router) {
	commonMiddleware := []mux.MiddlewareFunc{
		middleware.Authorize(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(c.app),
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
		Orders: mapping.MapViewModels(entities, func(o *order.Order) *viewmodels.Order {
			return mappers.OrderToViewModel(o, map[uint]int{})
		}),
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

	countByPositionID := make(map[uint]int)
	for _, item := range entity.Items {
		count, err := c.productService.CountByPositionID(r.Context(), item.Position.ID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		countByPositionID[item.Position.ID] = int(count)
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
		SaveURL:     fmt.Sprintf("%s/in", c.basePath),
		ItemsURL:    fmt.Sprintf("%s/items", c.basePath),
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
		ItemsURL:    fmt.Sprintf("%s/items", c.basePath),
	}
	templ.Handler(orderout.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *OrdersController) CreateInOrder(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	formDTO := dtos.CreateOrderDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&formDTO, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := formDTO.Ok(r.Context()); !ok {
		props := &orderin.PageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
		}
		templ.Handler(orderin.New(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	dto := order.CreateDTO{
		Type:       string(order.TypeIn),
		Status:     string(order.Pending),
		ProductIDs: []uint{},
	}
	for _, positionID := range formDTO.PositionIDs {
		quantity := formDTO.Quantity[positionID]
		products, err := c.orderService.GetOldestProducts(r.Context(), positionID, quantity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(products) < int(quantity) {
			// TODO: Localize this message
			http.Error(w, "Not enough products", http.StatusBadRequest)
			return
		}
		for _, product := range products {
			dto.ProductIDs = append(dto.ProductIDs, product.ID)
		}
	}

	if err := c.orderService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *OrdersController) CreateOutOrder(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	formDTO := dtos.CreateOrderDTO{} //nolint:exhaustruct
	if err := shared.Decoder.Decode(&formDTO, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if errorsMap, ok := formDTO.Ok(r.Context()); !ok {
		items, err := c.orderItems(r.Context(), formDTO.PositionIDs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &orderout.OderItemsProps{

			Items:  items,
			Errors: errorsMap,
		}
		fmt.Println(errorsMap)
		templ.Handler(orderout.OrderItems(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	dto := order.CreateDTO{
		Type:       string(order.TypeOut),
		Status:     string(order.Pending),
		ProductIDs: []uint{},
	}
	for _, positionID := range formDTO.PositionIDs {
		quantity := formDTO.Quantity[positionID]
		products, err := c.orderService.GetOldestProducts(r.Context(), positionID, quantity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if len(products) < int(quantity) {
			// TODO: Localize this message
			http.Error(w, "Not enough products", http.StatusBadRequest)
			return
		}
		for _, product := range products {
			dto.ProductIDs = append(dto.ProductIDs, product.ID)
		}
	}

	if err := c.orderService.Create(r.Context(), dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

func (c *OrdersController) orderItems(ctx context.Context, positionIDs []uint) ([]*viewmodels.OrderItem, error) {
	positionEntities, err := c.positionService.GetByIDs(ctx, positionIDs)
	if err != nil {
		return nil, err
	}

	items := make([]*viewmodels.OrderItem, len(positionEntities))
	for i, position := range positionEntities {
		quantity, err := c.productService.CountByPositionID(ctx, position.ID)
		if err != nil {
			return nil, err
		}
		items[i] = &viewmodels.OrderItem{
			Position: *mappers.PositionToViewModel(position),
			InStock:  strconv.FormatUint(uint64(quantity), 10),
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

	items, err := c.orderItems(r.Context(), dto.PositionIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &orderout.OderItemsProps{
		Items:  items,
		Errors: map[string]string{},
	}

	templ.Handler(orderout.OrderItems(props), templ.WithStreaming()).ServeHTTP(w, r)
}
