package controllers

import (
	"github.com/a-h/templ"
	"github.com/go-faster/errors"
	"github.com/gorilla/mux"
	"net/http"

	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/aggregates/payment"
	"github.com/iota-agency/iota-erp/internal/presentation/mappers"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/payments"
	"github.com/iota-agency/iota-erp/internal/presentation/viewmodels"
	"github.com/iota-agency/iota-erp/pkg/composables"
	"github.com/iota-agency/iota-erp/pkg/middleware"
)

type PaymentsController struct {
	app      *services.Application
	basePath string
}

func NewPaymentsController(app *services.Application) Controller {
	return &PaymentsController{
		app:      app,
		basePath: "/finance/payments",
	}
}

func (c *PaymentsController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.Use(middleware.RequireAuthorization())
	router.HandleFunc("", c.Payments).Methods(http.MethodGet)
	router.HandleFunc("", c.CreatePayment).Methods(http.MethodPost)
	router.HandleFunc("/new", c.GetNew).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	router.HandleFunc("/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	router.HandleFunc("/{id:[0-9]+}", c.DeletePayment).Methods(http.MethodDelete)
}

func (c *PaymentsController) viewModelPayments(r *http.Request) ([]*viewmodels.Payment, error) {
	ps, err := c.app.PaymentService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving payments")
	}
	vms := make([]*viewmodels.Payment, len(ps))
	for i, p := range ps {
		vms[i] = mappers.PaymentToViewModel(p)
	}
	return vms, nil
}

func (c *PaymentsController) viewModelPayment(r *http.Request) (*viewmodels.Payment, error) {
	id, err := parseID(r)
	if err != nil {
		return nil, errors.Wrap(err, "Error parsing id")
	}
	p, err := c.app.PaymentService.GetByID(r.Context(), id)
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving payment")
	}
	return mappers.PaymentToViewModel(p), nil
}

func (c *PaymentsController) viewModelStages(r *http.Request) ([]*viewmodels.ProjectStage, error) {
	stages, err := c.app.ProjectStageService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving stages")
	}
	vms := make([]*viewmodels.ProjectStage, len(stages))
	for i, s := range stages {
		vms[i] = mappers.ProjectStageToViewModel(s)
	}
	return vms, nil
}

func (c *PaymentsController) viewModelAccounts(r *http.Request) ([]*viewmodels.MoneyAccount, error) {
	accounts, err := c.app.MoneyAccountService.GetAll(r.Context())
	if err != nil {
		return nil, errors.Wrap(err, "Error retrieving accounts")
	}
	vms := make([]*viewmodels.MoneyAccount, len(accounts))
	for i, a := range accounts {
		vms[i] = mappers.MoneyAccountToViewModel(a, "")
	}
	return vms, nil
}

func (c *PaymentsController) Payments(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		composables.NewPageData("Payments.Meta.List.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	paymentViewModels, err := c.viewModelPayments(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("Hx-Request")) > 0
	props := &payments.IndexPageProps{
		PageContext: pageCtx,
		Payments:    paymentViewModels,
	}
	if isHxRequest {
		templ.Handler(payments.PaymentsTable(props), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(payments.Index(props), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *PaymentsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r,
		composables.NewPageData("Payments.Meta.Edit.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	paymentViewModel, err := c.viewModelPayment(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stages, err := c.viewModelStages(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	accounts, err := c.viewModelAccounts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &payments.EditPageProps{
		PageContext: pageCtx,
		Payment:     paymentViewModel,
		Stages:      stages,
		Accounts:    accounts,
		Errors:      make(map[string]string),
	}
	templ.Handler(payments.Edit(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) DeletePayment(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.app.PaymentService.Delete(r.Context(), id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *PaymentsController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := parseID(r)
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	action := FormAction(r.FormValue("_action"))
	if !action.IsValid() {
		http.Error(w, "Invalid action", http.StatusBadRequest)
		return
	}
	r.Form.Del("_action")

	switch action {
	case FormActionDelete:
		_, err = c.app.PaymentService.Delete(r.Context(), id)
	case FormActionSave:
		dto := payment.UpdateDTO{} //nolint:exhaustruct
		if err := decoder.Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var pageCtx *composables.PageContext
		pageCtx, err = composables.UsePageCtx(
			r,
			composables.NewPageData("Payments.Meta.Edit.Title", ""),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
			paymentViewModel, err := c.viewModelPayment(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			stages, err := c.viewModelStages(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			accounts, err := c.viewModelAccounts(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			props := &payments.EditPageProps{
				PageContext: pageCtx,
				Payment:     paymentViewModel,
				Stages:      stages,
				Accounts:    accounts,
				Errors:      errorsMap,
			}
			templ.Handler(payments.EditForm(props), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		err = c.app.PaymentService.Update(r.Context(), id, &dto)
	}
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *PaymentsController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(
		r, composables.NewPageData("Payments.Meta.New.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stages, err := c.viewModelStages(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	accounts, err := c.viewModelAccounts(r)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	props := &payments.CreatePageProps{
		PageContext: pageCtx,
		Stages:      stages,
		Payment:     &viewmodels.Payment{}, //nolint:exhaustruct
		Accounts:    accounts,
		Errors:      make(map[string]string),
	}
	templ.Handler(payments.New(props), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) CreatePayment(w http.ResponseWriter, r *http.Request) {
	dto := payment.CreateDTO{} //nolint:exhaustruct
	if err := decoder.Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(
		r, composables.NewPageData("Payments.Meta.New.Title", ""),
	)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errorsMap, ok := dto.Ok(pageCtx.UniTranslator); !ok {
		stages, err := c.viewModelStages(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		accounts, err := c.viewModelAccounts(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		props := &payments.CreatePageProps{
			PageContext: pageCtx,
			Payment:     mappers.PaymentToViewModel(dto.ToEntity()),
			Accounts:    accounts,
			Errors:      errorsMap,
			Stages:      stages,
		}
		templ.Handler(
			payments.CreateForm(props),
			templ.WithStreaming(),
		).ServeHTTP(w, r)
		return
	}

	if err := c.app.PaymentService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, c.basePath)
}
