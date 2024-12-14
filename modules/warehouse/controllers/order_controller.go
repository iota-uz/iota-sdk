package controllers

import (
	"context"
	"fmt"
	"github.com/iota-agency/iota-sdk/modules/warehouse/controllers/dtos"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/position_service"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services/product_service"
	order_in "github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/orders/in"
	order_out "github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/orders/out"
	"github.com/iota-agency/iota-sdk/pkg/mapping"
	"github.com/iota-agency/iota-sdk/pkg/middleware"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-sdk/components/base/pagination"
	"github.com/iota-agency/iota-sdk/modules/warehouse/domain/aggregates/order"
	"github.com/iota-agency/iota-sdk/modules/warehouse/mappers"
	"github.com/iota-agency/iota-sdk/modules/warehouse/services"
	"github.com/iota-agency/iota-sdk/modules/warehouse/templates/pages/orders"
	"github.com/iota-agency/iota-sdk/modules/warehouse/viewmodels"
	"github.com/iota-agency/iota-sdk/pkg/application"
	"github.com/iota-agency/iota-sdk/pkg/composables"
	"github.com/iota-agency/iota-sdk/pkg/shared"
	"github.com/iota-agency/iota-sdk/pkg/types"
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
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.WithTransaction(),
		middleware.Authorize(),
		middleware.RequireAuthorization(),
		middleware.ProvideUser(),
		middleware.Tabs(),
		middleware.WithLocalizer(c.app.Bundle()),
		middleware.NavItems(c.app),
	)

	router.HandleFunc("", c.List).Methods(http.MethodGet)
	router.HandleFunc("/in", c.CreateInOrder).Methods(http.MethodPost)
	router.HandleFunc("/out", c.CreateOutOrder).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	//router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/in/new", c.NewInOrder).Methods(http.MethodGet)
	router.HandleFunc("/out/new", c.NewOutOrder).Methods(http.MethodGet)
	router.HandleFunc("/items", c.OrderItems).Methods(http.MethodPost)
}

func (c *OrdersController) viewModelOrders(r *http.Request) (*OrderPaginatedResponse, error) {
	params := composables.UsePaginated(r)
	entities, err := c.orderService.GetPaginated(r.Context(), &order.FindParams{
		Limit:  params.Limit,
		Offset: params.Offset,
		SortBy: []string{"created_at desc"},
	})
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving orders")
	}
	total, err := c.orderService.Count(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error counting orders")
	}
	return &OrderPaginatedResponse{
		PaginationState: pagination.New(c.basePath, params.Page, int(total), params.Limit),
		Orders:          mapping.MapViewModels(entities, mappers.OrderToViewModel),
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

func (c *OrdersController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseID(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r,
		types.NewPageData("WarehouseOrders.Edit.Meta.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	entity, err := c.orderService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving order", http.StatusInternalServerError)
		return
	}
	props := &orders.EditPageProps{
		PageContext: pageCtx,
		Order:       mappers.OrderToViewModel(entity),
		Errors:      map[string]string{},
		SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
		DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
	}
	templ.Handler(orders.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

//
//func (c *OrdersController) PostEdit(w http.ResponseWriter, r *http.Request) {
//	id, err := shared.ParseID(r)
//	if err != nil {
//		http.Error(w, "Error parsing id", http.StatusInternalServerError)
//		return
//	}
//	action := shared.FormAction(r.FormValue("_action"))
//	if !action.IsValid() {
//		http.Error(w, "Invalid action", http.StatusBadRequest)
//		return
//	}
//	r.Form.Del("_action")
//
//	switch action {
//	case shared.FormActionDelete:
//		if _, err := c.orderService.Delete(r.Context(), id); err != nil {
//			http.Error(w, err.Error(), http.StatusInternalServerError)
//			return
//		}
//	case shared.FormActionSave:
//		dto := dtos.UpdateOrderDTO{} //nolint:exhaustruct
//		var pageCtx *types.PageContext
//		pageCtx, err = composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.Edit.Meta.Title", ""))
//		if err != nil {
//			http.Error(w, err.Error(), http.StatusInternalServerError)
//			return
//		}
//		if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
//			http.Error(w, err.Error(), http.StatusBadRequest)
//			return
//		}
//		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
//			entity, err := c.orderService.GetByID(r.Context(), id)
//			if err != nil {
//				http.Error(w, "Error retrieving order", http.StatusInternalServerError)
//				return
//			}
//			props := &orders.EditPageProps{
//				PageContext: pageCtx,
//				Order:       mappers.OrderToViewModel(entity),
//				Errors:      errorsMap,
//				SaveURL:     fmt.Sprintf("%s/%d", c.basePath, id),
//				DeleteURL:   fmt.Sprintf("%s/%d", c.basePath, id),
//			}
//			templ.Handler(orders.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
//			return
//		}
//		if err := c.orderService.Update(r.Context(), id, &dto); err != nil {
//			http.Error(w, err.Error(), http.StatusInternalServerError)
//			return
//		}
//	}
//	shared.Redirect(w, r, c.basePath)
//}

func (c *OrdersController) NewInOrder(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &order_in.PageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		SaveURL:     fmt.Sprintf("%s/in", c.basePath),
		ItemsURL:    fmt.Sprintf("%s/items", c.basePath),
	}
	templ.Handler(order_in.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *OrdersController) NewOutOrder(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, types.NewPageData("WarehouseOrders.New.Meta.Title", ""))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &order_out.PageProps{
		PageContext: pageCtx,
		Errors:      map[string]string{},
		SaveURL:     fmt.Sprintf("%s/out", c.basePath),
		ItemsURL:    fmt.Sprintf("%s/items", c.basePath),
	}
	templ.Handler(order_out.New(props), templ.WithStreaming()).ServeHTTP(w, r)
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
		props := &order_in.PageProps{
			PageContext: pageCtx,
			Errors:      errorsMap,
		}
		templ.Handler(order_in.New(props), templ.WithStreaming()).ServeHTTP(w, r)
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
		props := &order_out.OderItemsProps{

			Items:  items,
			Errors: errorsMap,
		}
		fmt.Println(errorsMap)
		templ.Handler(order_out.OrderItems(props), templ.WithStreaming()).ServeHTTP(w, r)
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
			Quantity: strconv.Itoa(int(quantity)),
		}
	}
	return items, nil
}

func (c *OrdersController) OrderItems(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	dto := dtos.CreateOrderDTO{}

	if err := shared.Decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	items, err := c.orderItems(r.Context(), dto.PositionIDs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	props := &order_out.OderItemsProps{
		Items:  items,
		Errors: map[string]string{},
	}

	templ.Handler(order_out.OrderItems(props), templ.WithStreaming()).ServeHTTP(w, r)
}
