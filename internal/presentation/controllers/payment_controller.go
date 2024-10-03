package controllers

import (
	"github.com/gorilla/schema"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/payments"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
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
	r.HandleFunc(c.basePath, c.Payments).Methods(http.MethodGet)
	r.HandleFunc(c.basePath, c.CreatePayment).Methods(http.MethodPost)
	r.HandleFunc(c.basePath+"/new", c.GetNew).Methods(http.MethodGet)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	r.HandleFunc(c.basePath+"/{id:[0-9]+}", c.DeletePayment).Methods(http.MethodDelete)
}

func (c *PaymentsController) Payments(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.List.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	params := composables.UsePaginated(r)
	ps, err := c.app.PaymentService.GetPaginated(r.Context(), params.Limit, params.Offset, []string{})
	if err != nil {
		http.Error(w, "Error retrieving payments", http.StatusInternalServerError)
		return
	}
	isHxRequest := len(r.Header.Get("HX-Request")) > 0
	if isHxRequest {
		templ.Handler(payments.PaymentsTable(pageCtx.Localizer, ps), templ.WithStreaming()).ServeHTTP(w, r)
	} else {
		templ.Handler(payments.Index(pageCtx, ps), templ.WithStreaming()).ServeHTTP(w, r)
	}
}

func (c *PaymentsController) GetEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.Edit.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	ps, err := c.app.PaymentService.GetByID(r.Context(), uint(id))
	if err != nil {
		http.Error(w, "Error retrieving payment", http.StatusInternalServerError)
		return
	}
	templ.Handler(payments.Edit(pageCtx, ps, map[string]string{}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) DeletePayment(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}

	if _, err := c.app.PaymentService.Delete(r.Context(), uint(id)); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *PaymentsController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	action := r.FormValue("_action")
	r.Form.Del("_action")

	if action == "save" {
		dto := payment.UpdateDTO{}
		if err := schema.NewDecoder().Decode(&dto, r.Form); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := validate.Struct(&dto); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.Edit.Title"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if errors, ok := dto.Ok(pageCtx.Localizer); !ok {
			ps, err := c.app.PaymentService.GetByID(r.Context(), uint(id))
			if err != nil {
				http.Error(w, "Error retrieving payment", http.StatusInternalServerError)
				return
			}

			templ.Handler(payments.EditForm(pageCtx.Localizer, ps, errors), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		err = c.app.PaymentService.Update(r.Context(), uint(id), &dto)
	} else if action == "delete" {
		_, err = c.app.PaymentService.Delete(r.Context(), uint(id))
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, c.basePath)
}

func (c *PaymentsController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stages, err := c.app.ProjectStageService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	templ.Handler(payments.New(pageCtx, stages, map[string]string{}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) CreatePayment(w http.ResponseWriter, r *http.Request) {
	dto := payment.CreateDTO{}
	if err := schema.NewDecoder().Decode(&dto, r.Form); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := validate.Struct(&dto); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	stages, err := c.app.ProjectStageService.GetAll(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errors, ok := dto.Ok(pageCtx.Localizer); !ok {
		templ.Handler(payments.CreateForm(pageCtx.Localizer, dto.ToEntity(), stages, errors), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.app.PaymentService.Create(r.Context(), &dto); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, c.basePath)
}
