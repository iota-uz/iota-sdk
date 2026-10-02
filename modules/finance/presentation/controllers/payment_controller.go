// Package controllers provides this package.
package controllers

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/google/uuid"
	paymentcategory "github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment_category"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/controllers/dtos"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/mappers"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/templates/pages/payments"
	"github.com/iota-uz/iota-sdk/modules/finance/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/middleware"

	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/components/filters"
	"github.com/iota-uz/iota-sdk/components/scaffold/actions"
	"github.com/iota-uz/iota-sdk/components/scaffold/table"
	"github.com/iota-uz/iota-sdk/modules/finance/domain/aggregates/payment"
	"github.com/iota-uz/iota-sdk/modules/finance/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/htmx"
	"github.com/iota-uz/iota-sdk/pkg/mapping"
	"github.com/iota-uz/iota-sdk/pkg/serrors/serrorhttp"
	"github.com/iota-uz/iota-sdk/pkg/shared"
	"github.com/sirupsen/logrus"
)

type PaymentsController struct {
	paymentService         *services.PaymentService
	moneyAccountService    *services.MoneyAccountService
	counterpartyService    *services.CounterpartyService
	paymentCategoryService *services.PaymentCategoryService
	basePath               string
}

func NewPaymentsController(
	paymentService *services.PaymentService,
	moneyAccountService *services.MoneyAccountService,
	counterpartyService *services.CounterpartyService,
	paymentCategoryService *services.PaymentCategoryService,
) application.Controller {
	return &PaymentsController{
		paymentService:         paymentService,
		moneyAccountService:    moneyAccountService,
		counterpartyService:    counterpartyService,
		paymentCategoryService: paymentCategoryService,
		basePath:               "/finance/payments",
	}
}

func (c *PaymentsController) Descriptor() application.ControllerDescriptor {
	return application.Descriptor("finance.payment", 0, application.Route("", c.basePath))
}

func (c *PaymentsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(
		middleware.Authorize(),
		middleware.RedirectNotAuthenticated(),
		middleware.ProvideUser(),
		middleware.ProvideDynamicLogo(),
		middleware.NavItems(),
		middleware.WithPageContext(),
	)
	router.HandleFunc("", c.Payments).Methods(http.MethodGet)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.GetEdit).Methods(http.MethodGet)

	router.HandleFunc("", c.Create).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Update).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}", c.Delete).Methods(http.MethodDelete)

	// Attachment endpoints
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/attachments", di.H(c.AttachFile)).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9a-fA-F-]+}/attachments/{uploadId:[0-9]+}", di.H(c.DetachFile)).Methods(http.MethodDelete)
}

func (c *PaymentsController) viewModelPayment(r *http.Request) (*viewmodels.Payment, error) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing id")
	}
	p, err := c.paymentService.GetByID(r.Context(), id)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving payment")
	}
	return mappers.PaymentToViewModel(p), nil
}

func (c *PaymentsController) viewModelAccounts(r *http.Request) ([]*viewmodels.MoneyAccount, error) {
	accounts, err := c.moneyAccountService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving accounts")
	}
	return mapping.MapViewModels(accounts, mappers.MoneyAccountToViewModel), nil
}

func (c *PaymentsController) viewModelPaymentCategories(r *http.Request) ([]*viewmodels.PaymentCategory, error) {
	categories, err := c.paymentCategoryService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving payment categories")
	}
	return mapping.MapViewModels(categories, mappers.PaymentCategoryToViewModel), nil
}

func (c *PaymentsController) Payments(w http.ResponseWriter, r *http.Request) {
	paginationParams := composables.UsePaginated(r)
	params, err := composables.UseQuery(&payment.FindParams{
		Limit:  paginationParams.Limit,
		Offset: paginationParams.Offset,
	}, r)
	if err != nil {
		http.Error(w, "Error parsing query", http.StatusBadRequest)
		return
	}
	paymentEntities, err := c.paymentService.GetPaginated(r.Context(), params)
	if err != nil {
		http.Error(w, "Error retrieving payments", http.StatusInternalServerError)
		return
	}
	total, err := c.paymentService.Count(r.Context(), params)
	if err != nil {
		http.Error(w, "Error counting payments", http.StatusInternalServerError)
		return
	}

	cfg := c.tableConfig(r, paginationParams, int(total))
	for _, p := range paymentEntities {
		cfg.AddRows(table.Row(
			table.Cell(templ.Raw(p.Amount().Display()), p.Amount().Display()),
			table.Cell(table.DateTime(p.UpdatedAt()), p.UpdatedAt()),
			table.Cell(actions.RenderRowActions(actions.EditAction(fmt.Sprintf("%s/%s", c.basePath, p.ID()))), nil),
		))
	}

	// The overview tab loads the first chunk embedded; later chunks and
	// searches only need rows.
	if r.URL.Query().Get("embedded") == "true" && paginationParams.Page == 1 {
		templ.Handler(table.EmbeddedContent(cfg), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}
	templ.Handler(table.ContentHTMX(cfg), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) tableConfig(r *http.Request, params composables.PaginationParams, total int) *table.TableConfig {
	pageCtx := composables.UsePageCtx(r.Context())
	cfg := table.NewTableConfig(
		pageCtx.T("NavigationLinks.Payments"),
		c.basePath,
		// The overview shows several tables on one address, so each keeps
		// its column settings under its own key.
		table.WithID("finance-payments"),
		table.WithSearchValue(table.UseSearchQuery(r)),
		table.WithInfiniteScroll(total > params.Page*params.Limit, params.Page, params.Limit),
	)
	cfg.AddCols(
		table.Column("amount", pageCtx.T("Payments.List.Amount")),
		table.Column("updated_at", pageCtx.T("UpdatedAt")),
		table.Column("actions", pageCtx.T("Actions"), table.WithClass("w-16")),
	)
	cfg.AddFilters(filters.CreatedAt())
	cfg.AddActions(actions.RenderAction(actions.CreateAction(pageCtx.T("Payments.List.New"), c.basePath+"/new")))
	return cfg
}

func (c *PaymentsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	paymentViewModel, err := c.viewModelPayment(r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}
	accounts, err := c.viewModelAccounts(r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}
	categories, err := c.viewModelPaymentCategories(r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}

	props := &payments.EditPageProps{
		Payment:    paymentViewModel,
		Accounts:   accounts,
		Categories: categories,
		Errors:     make(map[string]string),
	}
	templ.Handler(payments.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) Delete(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.paymentService.Delete(r.Context(), id); err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}
	shared.Redirect(w, r, c.basePath)
}

// AttachFile attaches a file upload to a payment
func (c *PaymentsController) AttachFile(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	paymentService *services.PaymentService,
) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logger.Errorf("Error parsing payment ID: %v", err)
		http.Error(w, "Error parsing payment ID", http.StatusBadRequest)
		return
	}

	// Parse upload ID from form data
	uploadIDStr := r.FormValue("uploadId")
	if uploadIDStr == "" {
		logger.Error("Upload ID is required")
		http.Error(w, "Upload ID is required", http.StatusBadRequest)
		return
	}

	uploadID, err := strconv.ParseUint(uploadIDStr, 10, 32)
	if err != nil {
		logger.Errorf("Invalid upload ID: %v", err)
		http.Error(w, "Invalid upload ID", http.StatusBadRequest)
		return
	}

	err = paymentService.AttachFileToPayment(r.Context(), id, uint(uploadID))
	if err != nil {
		logger.Errorf("Error attaching file to payment: %v", err)
		http.Error(w, "Error attaching file", http.StatusInternalServerError)
		return
	}

	shared.Redirect(w, r, fmt.Sprintf("%s/%s", c.basePath, id.String()))
}

// DetachFile detaches a file upload from a payment
func (c *PaymentsController) DetachFile(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
	paymentService *services.PaymentService,
) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		logger.Errorf("Error parsing payment ID: %v", err)
		http.Error(w, "Error parsing payment ID", http.StatusBadRequest)
		return
	}

	// Parse upload ID from URL path
	uploadIDStr := mux.Vars(r)["uploadId"]
	uploadID, err := strconv.ParseUint(uploadIDStr, 10, 32)
	if err != nil {
		logger.Errorf("Invalid upload ID: %v", err)
		http.Error(w, "Invalid upload ID", http.StatusBadRequest)
		return
	}

	err = paymentService.DetachFileFromPayment(r.Context(), id, uint(uploadID))
	if err != nil {
		logger.Errorf("Error detaching file from payment: %v", err)
		http.Error(w, "Error detaching file", http.StatusInternalServerError)
		return
	}

	if htmx.IsHxRequest(r) {
		w.WriteHeader(http.StatusOK)
	} else {
		shared.Redirect(w, r, fmt.Sprintf("%s/%s", c.basePath, id.String()))
	}
}

func (c *PaymentsController) Update(w http.ResponseWriter, r *http.Request) {
	id, err := shared.ParseUUID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	dto, err := composables.UseForm(&dtos.PaymentUpdateDTO{}, r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusBadRequest, nil)
		return
	}

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		paymentViewModel, err := c.viewModelPayment(r)
		if err != nil {
			serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
			return
		}
		accounts, err := c.viewModelAccounts(r)
		if err != nil {
			serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
			return
		}
		categories, err := c.viewModelPaymentCategories(r)
		if err != nil {
			serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
			return
		}
		props := &payments.EditPageProps{
			Payment:    paymentViewModel,
			Accounts:   accounts,
			Categories: categories,
			Errors:     errorsMap,
		}
		templ.Handler(payments.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	// Get payment category
	var categoryEntity paymentcategory.PaymentCategory
	if dto.PaymentCategoryID != "" {
		categoryID, err := uuid.Parse(dto.PaymentCategoryID)
		if err != nil {
			http.Error(w, "Invalid payment category ID", http.StatusBadRequest)
			return
		}
		categoryEntity, err = c.paymentCategoryService.GetByID(r.Context(), categoryID)
		if err != nil {
			serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
			return
		}
	} else {
		categoryEntity = paymentcategory.New("Uncategorized")
	}

	existing, err := c.paymentService.GetByID(r.Context(), id)
	if err != nil {
		http.Error(w, "Error retrieving payment", http.StatusInternalServerError)
		return
	}

	user, err := composables.UseUser(r.Context())
	if err != nil {
		http.Error(w, "Error getting user", http.StatusInternalServerError)
		return
	}

	entity, err := dto.Apply(existing, categoryEntity, user)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}
	if _, err := c.paymentService.Update(r.Context(), entity); err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}

	// Handle file attachments
	for _, uploadID := range dto.Attachments {
		if uploadID > 0 {
			if err := c.paymentService.AttachFileToPayment(r.Context(), id, uploadID); err != nil {
				http.Error(w, "Error attaching file to payment", http.StatusInternalServerError)
				return
			}
		}
	}

	shared.Redirect(w, r, c.basePath)
}

func (c *PaymentsController) GetNew(w http.ResponseWriter, r *http.Request) {
	accounts, err := c.viewModelAccounts(r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}
	categories, err := c.viewModelPaymentCategories(r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}

	props := &payments.CreatePageProps{
		Payment:    &viewmodels.Payment{},
		Accounts:   accounts,
		Categories: categories,
		Errors:     make(map[string]string),
	}
	templ.Handler(payments.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) Create(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&dtos.PaymentCreateDTO{}, r)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusBadRequest, nil)
		return
	}

	u, err := composables.UseUser(r.Context())
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}
	dto.UserID = u.ID()

	if errorsMap, ok := dto.Ok(r.Context()); !ok {
		accounts, err := c.viewModelAccounts(r)
		if err != nil {
			serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
			return
		}
		categories, err := c.viewModelPaymentCategories(r)
		if err != nil {
			serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
			return
		}
		// Create a default payment viewmodel for displaying errors
		payment := &viewmodels.Payment{
			Amount:           fmt.Sprintf("%.2f", dto.Amount),
			AccountID:        dto.AccountID,
			CounterpartyID:   dto.CounterpartyID,
			CategoryID:       dto.PaymentCategoryID,
			TransactionDate:  time.Time(dto.TransactionDate).Format(time.DateOnly),
			AccountingPeriod: time.Time(dto.AccountingPeriod).Format(time.DateOnly),
			Comment:          dto.Comment,
		}
		props := &payments.CreatePageProps{
			Payment:    payment,
			Accounts:   accounts,
			Categories: categories,
			Errors:     errorsMap,
		}
		templ.Handler(payments.CreateForm(props), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	// Get payment category
	categoryID, err := uuid.Parse(dto.PaymentCategoryID)
	if err != nil {
		http.Error(w, "Invalid payment category ID", http.StatusBadRequest)
		return
	}
	categoryEntity, err := c.paymentCategoryService.GetByID(r.Context(), categoryID)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}

	tenantID, err := composables.UseTenantID(r.Context())
	if err != nil {
		http.Error(w, "Error getting tenant ID", http.StatusInternalServerError)
		return
	}

	entity := dto.ToEntity(tenantID, categoryEntity)
	createdEntity, err := c.paymentService.Create(r.Context(), entity)
	if err != nil {
		serrorhttp.WriteText(w, err, http.StatusInternalServerError, nil)
		return
	}

	// Handle file attachments
	for _, uploadID := range dto.Attachments {
		if uploadID > 0 {
			if err := c.paymentService.AttachFileToPayment(r.Context(), createdEntity.ID(), uploadID); err != nil {
				http.Error(w, "Error attaching file to payment", http.StatusInternalServerError)
				return
			}
		}
	}

	shared.Redirect(w, r, c.basePath)
}
