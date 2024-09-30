package controllers

import (
	"net/http"
	"strconv"

	"github.com/a-h/templ"
	"github.com/gorilla/mux"
	"github.com/iota-agency/iota-erp/internal/app/services"
	"github.com/iota-agency/iota-erp/internal/domain/entities/payment"
	"github.com/iota-agency/iota-erp/internal/presentation/templates/pages/payments"
	"github.com/iota-agency/iota-erp/internal/presentation/types"
	"github.com/iota-agency/iota-erp/pkg/composables"
)

type PaymentsController struct {
	app *services.Application
}

func NewPaymentsController(app *services.Application) Controller {
	return &PaymentsController{
		app: app,
	}
}

func (c *PaymentsController) Register(r *mux.Router) {
	r.HandleFunc("/finance/payments", c.Payments).Methods(http.MethodGet)
	r.HandleFunc("/finance/payments", c.CreatePayment).Methods(http.MethodPost)
	r.HandleFunc("/finance/payments/{id:[0-9]+}", c.GetEdit).Methods(http.MethodGet)
	r.HandleFunc("/finance/payments/{id:[0-9]+}", c.PostEdit).Methods(http.MethodPost)
	r.HandleFunc("/finance/payments/{id:[0-9]+}", c.DeletePayment).Methods(http.MethodDelete)
	r.HandleFunc("/finance/payments/new", c.GetNew).Methods(http.MethodGet)
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
	redirect(w, r, "/payments")
}

func (c *PaymentsController) PostEdit(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(mux.Vars(r)["id"])
	if err != nil {
		http.Error(w, "Error parsing id", http.StatusInternalServerError)
		return
	}
	action := r.FormValue("_action")
	if action == "save" {
		amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
		if err != nil {
			http.Error(w, "Error parsing id", http.StatusInternalServerError)
			return
		}
		upd := &payment.PaymentUpdate{
			Amount: amount,
		}
		var pageCtx *types.PageContext
		pageCtx, err = composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.Edit.Title"})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if errors, ok := upd.Ok(pageCtx.Localizer); !ok {
			ps, err := c.app.PaymentService.GetByID(r.Context(), uint(id))
			if err != nil {
				http.Error(w, "Error retrieving payment", http.StatusInternalServerError)
				return
			}

			templ.Handler(payments.EditForm(pageCtx.Localizer, ps, errors), templ.WithStreaming()).ServeHTTP(w, r)
			return
		}
		err = c.app.PaymentService.Update(r.Context(), &payment.Payment{
			Id:     uint(id),
			Amount: upd.Amount,
		})
	} else if action == "delete" {
		_, err = c.app.PaymentService.Delete(r.Context(), uint(id))
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	redirect(w, r, "/payments")
}

func (c *PaymentsController) GetNew(w http.ResponseWriter, r *http.Request) {
	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	templ.Handler(payments.New(pageCtx, map[string]string{}), templ.WithStreaming()).ServeHTTP(w, r)
}

func (c *PaymentsController) CreatePayment(w http.ResponseWriter, r *http.Request) {
	amount, err := strconv.ParseFloat(r.FormValue("amount"), 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	paymentEntity := &payment.Payment{
		Amount: amount,
	}

	pageCtx, err := composables.UsePageCtx(r, &composables.PageData{Title: "Payments.Meta.New.Title"})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if errors, ok := paymentEntity.Ok(pageCtx.Localizer); !ok {
		templ.Handler(payments.CreateForm(pageCtx.Localizer, paymentEntity, errors), templ.WithStreaming()).ServeHTTP(w, r)
		return
	}

	if err := c.app.PaymentService.Create(r.Context(), paymentEntity); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	redirect(w, r, "/payments")
}
