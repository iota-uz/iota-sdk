package controllers

import (
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	clickapi "github.com/iota-uz/click"
	clickauth "github.com/iota-uz/click/auth"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/composables"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

type ClickController struct {
	app            application.Application
	billingService *services.BillingService
	click          configuration.ClickOptions
	basePath       string
}

func NewClickController(app application.Application, click configuration.ClickOptions, basePath string) application.Controller {
	return &ClickController{
		app:            app,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
		click:          click,
		basePath:       basePath,
	}
}

func (c *ClickController) Key() string {
	return c.basePath
}

func (c *ClickController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.HandleFunc("/prepare", c.Prepare).Methods(http.MethodPost)
	router.HandleFunc("/complete", c.Complete).Methods(http.MethodPost)
}

func (c *ClickController) Prepare(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&clickapi.PrepareRequest{}, r)
	if err != nil {
		log.Printf("Form decode error (Pepare): %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	entities, err := c.billingService.GetByDetailsFields(
		r.Context(),
		billing.Click,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"merchant_trans_id"},
				Operator: billing.OpEqual,
				Value:    dto.MerchantTransId,
			},
		},
	)
	if err != nil {
		log.Printf("Failed to get transaction: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(entities) != 1 {
		log.Printf("Unexpected number of transactions found: %v", len(entities))
		http.Error(w, "Transaction not found or ambiguous", http.StatusBadRequest)
		return
	}

	entity := entities[0]

	if entity.Gateway() != billing.Click {
		log.Printf("Invalid gateway: %v", entity.Gateway())
		http.Error(w, "Invalid gateway", http.StatusBadRequest)
		return
	}

	clickDetails, ok := entity.Details().(details.ClickDetails)
	if !ok {
		log.Printf("Details is not of type ClickDetails")
		http.Error(w, "Invalid details type", http.StatusInternalServerError)
		return
	}

	if !clickauth.ValidatePrepareSignString(
		dto.SignString,
		dto.ClickTransId,
		c.click.ServiceID,
		c.click.SecretKey,
		clickDetails.MerchantTransID(),
		entity.Amount().Quantity(),
		dto.Action,
		dto.SignTime,
	) {
		log.Printf("Invalid signature in Prepare request")
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	entity = entity.
		SetStatus(billing.Pending).
		SetDetails(
			clickDetails.
				SetMerchantPrepareID(entity.CreatedAt().Unix()).
				SetPaymentID(dto.ClickTransId).
				SetPaymentStatus(dto.Action).
				SetPayDocId(dto.ClickPaydocId).
				SetErrorCode(dto.Error).
				SetErrorNote(dto.ErrorNote).
				SetSignTime(dto.SignTime).
				SetSignString(dto.SignString),
		)

	entity, err = c.billingService.Save(r.Context(), entity)
	if err != nil {
		log.Printf("Failed to update transaction: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clickDetails, ok = entity.Details().(details.ClickDetails)
	if !ok {
		log.Printf("Details is not of type ClickDetails")
		http.Error(w, "Invalid details type", http.StatusInternalServerError)
		return
	}

	response := clickapi.PrepareResponse{
		ClickTransId:      clickDetails.PaymentID(),
		MerchantTransId:   clickDetails.MerchantTransID(),
		MerchantPrepareId: clickDetails.MerchantPrepareID(),
		Error:             clickDetails.ErrorCode(),
		ErrorNote:         clickDetails.ErrorNote(),
	}

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		log.Printf("Failed to write JSON response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (c *ClickController) Complete(w http.ResponseWriter, r *http.Request) {
	dto, err := composables.UseForm(&clickapi.CompleteRequest{}, r)
	if err != nil {
		log.Printf("Form decode error (Complete): %v", err)
		http.Error(w, "Invalid form data", http.StatusBadRequest)
		return
	}

	entities, err := c.billingService.GetByDetailsFields(
		r.Context(),
		billing.Click,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"merchant_trans_id"},
				Operator: billing.OpEqual,
				Value:    dto.MerchantTransId,
			},
		},
	)
	if err != nil {
		log.Printf("Failed to get transaction: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(entities) != 1 {
		log.Printf("Unexpected number of transactions found: %v", len(entities))
		http.Error(w, "Transaction not found or ambiguous", http.StatusBadRequest)
		return
	}

	entity := entities[0]

	if entity.Gateway() != billing.Click {
		log.Printf("Invalid gateway: %v", entity.Gateway())
		http.Error(w, "Invalid gateway", http.StatusBadRequest)
		return
	}

	clickDetails, ok := entity.Details().(details.ClickDetails)
	if !ok {
		log.Printf("Details is not of type ClickDetails")
		http.Error(w, "Invalid details type", http.StatusInternalServerError)
		return
	}

	if !clickauth.ValidateCompleteSignString(
		dto.SignString,
		clickDetails.PaymentID(),
		c.click.ServiceID,
		c.click.SecretKey,
		clickDetails.MerchantTransID(),
		clickDetails.MerchantPrepareID(),
		entity.Amount().Quantity(),
		dto.Action,
		dto.SignTime,
	) {
		log.Printf("Invalid signature in Complete request")
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	entity = entity.
		SetStatus(billing.Completed).
		SetDetails(
			clickDetails.
				SetMerchantConfirmID(entity.UpdatedAt().Unix()).
				SetPaymentStatus(dto.Action).
				SetErrorCode(dto.Error).
				SetErrorNote(dto.ErrorNote).
				SetSignTime(dto.SignTime).
				SetSignString(dto.SignString),
		)

	entity, err = c.billingService.Save(r.Context(), entity)
	if err != nil {
		log.Printf("Failed to update transaction: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clickDetails, ok = entity.Details().(details.ClickDetails)
	if !ok {
		log.Printf("Details is not of type ClickDetails")
		http.Error(w, "Invalid details type", http.StatusInternalServerError)
		return
	}

	response := clickapi.CompleteResponse{
		ClickTransId:      clickDetails.PaymentID(),
		MerchantTransId:   clickDetails.MerchantTransID(),
		MerchantConfirmId: clickDetails.MerchantConfirmID(),
		Error:             clickDetails.ErrorCode(),
		ErrorNote:         clickDetails.ErrorNote(),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("Failed to write JSON response (Complete): %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
