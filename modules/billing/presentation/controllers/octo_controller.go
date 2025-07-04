package controllers

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	octoapi "github.com/iota-uz/octo"
	octoauth "github.com/iota-uz/octo/auth"
)

type OctoController struct {
	app            application.Application
	billingService *services.BillingService
	octo           configuration.OctoOptions
	basePath       string
	logger         *middleware.LogTransport
}

func NewOctoController(
	app application.Application,
	octo configuration.OctoOptions,
	basePath string,
	logger *middleware.LogTransport,
) application.Controller {
	return &OctoController{
		app:            app,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
		octo:           octo,
		basePath:       basePath,
		logger:         logger,
	}
}

func (c *OctoController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.HandleFunc("", c.Handle).Methods(http.MethodPost)
}

func (c *OctoController) Key() string {
	return c.basePath
}

func (c *OctoController) Handle(w http.ResponseWriter, r *http.Request) {
	var notification octoapi.NotificationRequest

	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		http.Error(w, "invalid JSON", http.StatusBadRequest)
		return
	}

	if !octoauth.ValidateSignature(
		notification.Signature,
		c.octo.OctoSecretHash,
		notification.OctoPaymentUUID,
		notification.Status,
	) {
		log.Printf("Failed to validate signature: %v", notification)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	entities, err := c.billingService.GetByDetailsFields(
		r.Context(),
		billing.Octo,
		[]billing.DetailsFieldFilter{
			{
				Path:     []string{"shop_transaction_id"},
				Operator: billing.OpEqual,
				Value:    notification.ShopTransactionId,
			},
			{
				Path:     []string{"octo_payment_uuid"},
				Operator: billing.OpEqual,
				Value:    notification.OctoPaymentUUID,
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
	octoDetails, ok := entity.Details().(details.OctoDetails)
	if !ok {
		log.Printf("Details is not of type OctoDetails")
		http.Error(w, "Invalid details type", http.StatusInternalServerError)
		return
	}

	octoDetails = octoDetails.
		SetStatus(notification.Status).
		SetSignature(notification.Signature).
		SetHashKey(notification.HashKey).
		SetTransferSum(notification.GetTransferSum()).
		SetRefundedSum(notification.GetRefundedSum()).
		SetCardCountry(notification.GetCardCountry()).
		SetCardMaskedPan(notification.GetMaskedPan()).
		SetRrn(notification.GetRrn()).
		SetRiskLevel(notification.GetRiskLevel()).
		SetPayedTime(notification.GetPayedTime()).
		SetCardType(notification.GetCardType()).
		SetCardIsPhysical(notification.GetIsPhysicalCard())

	switch notification.Status {
	case octoapi.WaitingForCaptureStatus:
		entity = entity.SetStatus(billing.Pending)
	case octoapi.CancelledStatus:
		entity = entity.SetStatus(billing.Canceled)
	case octoapi.SucceededStatus:
		entity = entity.SetStatus(billing.Completed)
	}

	entity = entity.SetDetails(octoDetails)
	entity, err = c.billingService.Save(r.Context(), entity)
	if err != nil {
		log.Printf("Failed to update transaction: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	octoDetails, ok = entity.Details().(details.OctoDetails)
	if !ok {
		log.Printf("Details is not of type OctoDetails")
		http.Error(w, "Invalid details type", http.StatusInternalServerError)
		return
	}

	if !octoDetails.AutoCapture() {
		cfg := octoapi.NewConfiguration()
		cfg.HTTPClient = &http.Client{
			Transport: c.logger,
		}
		apiClient := octoapi.NewAPIClient(cfg)

		req := octoapi.SetAcceptRequest{
			OctoShopId:      octoDetails.OctoShopId(),
			OctoSecret:      c.octo.OctoSecret,
			OctoPaymentUUID: octoDetails.OctoPaymentUUID(),
			AcceptStatus:    octoapi.CaptureStatus,
			FinalAmount:     entity.Amount().Quantity(),
		}

		ctx := context.Background()
		resp, httpResp, err := apiClient.TransactionManagementAPI.
			SetAcceptPost(ctx).
			SetAcceptRequest(req).
			Execute()

		if httpResp != nil {
			if hErr := httpResp.Body.Close(); hErr != nil {
				log.Printf("failed to close http response body: %v", hErr)
			}
		}
		if err != nil {
			return
		}
		if resp.GetError() != 0 {
			octoDetails = octoDetails.
				SetError(resp.GetError()).
				SetErrMessage(resp.GetErrMessage())
		} else {
			octoDetails = octoDetails.
				SetStatus(resp.Data.GetStatus()).
				SetTransferSum(resp.Data.GetTransferSum()).
				SetRefundedSum(resp.Data.GetRefundedSum()).
				SetPayedTime(resp.Data.GetPayedTime())

			switch resp.Data.GetStatus() {
			case octoapi.CancelledStatus:
				entity = entity.SetStatus(billing.Canceled)
			case octoapi.SucceededStatus:
				entity = entity.SetStatus(billing.Completed)
			}
		}

		entity = entity.SetDetails(octoDetails)
		entity, err = c.billingService.Save(r.Context(), entity)
		if err != nil {
			log.Printf("Failed to update transaction: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	acceptStatus := octoDetails.Status()
	if !octoDetails.AutoCapture() {
		acceptStatus = octoapi.CaptureStatus
	}

	callbackResponse := octoapi.CallbackResponse{
		AcceptStatus: &acceptStatus,
		FinalAmount:  octoapi.PtrFloat64(entity.Amount().Quantity()),
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(callbackResponse); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}
