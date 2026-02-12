package controllers

import (
	"encoding/json"
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
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/sirupsen/logrus"
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
	router.HandleFunc("/prepare", di.H(c.Prepare)).Methods(http.MethodPost)
	router.HandleFunc("/complete", di.H(c.Complete)).Methods(http.MethodPost)
}

func (c *ClickController) Prepare(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	logger.Info("Click Prepare request received")

	dto, err := composables.UseForm(&clickapi.PrepareRequest{}, r)
	if err != nil {
		logger.WithError(err).Error("Form decode error in Prepare")
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
		logger.WithError(err).WithField("merchant_trans_id", dto.MerchantTransId).Error("Failed to get transaction")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(entities) != 1 {
		logger.WithFields(logrus.Fields{
			"merchant_trans_id": dto.MerchantTransId,
			"count":             len(entities),
		}).Error("Unexpected number of transactions found")
		http.Error(w, "Transaction not found or ambiguous", http.StatusBadRequest)
		return
	}

	entity := entities[0]

	if entity.Gateway() != billing.Click {
		logger.WithFields(logrus.Fields{
			"expected_gateway": billing.Click,
			"actual_gateway":   entity.Gateway(),
		}).Error("Invalid gateway")
		http.Error(w, "Invalid gateway", http.StatusBadRequest)
		return
	}

	clickDetails, ok := entity.Details().(details.ClickDetails)
	if !ok {
		logger.Error("Details is not of type ClickDetails")
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
		logger.WithField("click_trans_id", dto.ClickTransId).Error("Invalid signature in Prepare request")
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
		logger.WithError(err).WithField("merchant_trans_id", dto.MerchantTransId).Error("Failed to update transaction")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	clickDetails, ok = entity.Details().(details.ClickDetails)
	if !ok {
		logger.Error("Details is not of type ClickDetails after save")
		http.Error(w, "Invalid details type", http.StatusInternalServerError)
		return
	}

	// Invoke callback to allow external processing
	if err := c.billingService.InvokeCallback(r.Context(), entity); err != nil {
		logger.WithError(err).WithField("merchant_trans_id", dto.MerchantTransId).Error("Callback error in Prepare")

		// Update entity with error details
		// https://docs.click.uz/click-api-error/
		entity = entity.SetDetails(
			clickDetails.
				SetErrorCode(-9).
				SetErrorNote(err.Error()),
		)
		entity := entity.SetStatus(billing.Failed)

		entity, saveErr := c.billingService.Save(r.Context(), entity)
		if saveErr != nil {
			logger.WithError(saveErr).Error("Failed to save callback error")
		}

		clickDetails, ok = entity.Details().(details.ClickDetails)
		if !ok {
			logger.Error("Details is not of type ClickDetails after callback error")
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
	}

	response := clickapi.PrepareResponse{
		ClickTransId:      clickDetails.PaymentID(),
		MerchantTransId:   clickDetails.MerchantTransID(),
		MerchantPrepareId: clickDetails.MerchantPrepareID(),
		Error:             clickDetails.ErrorCode(),
		ErrorNote:         clickDetails.ErrorNote(),
	}

	logger.WithFields(logrus.Fields{
		"click_trans_id":    clickDetails.PaymentID(),
		"merchant_trans_id": clickDetails.MerchantTransID(),
		"status":            entity.Status(),
	}).Info("Click Prepare completed successfully")

	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(response)
	if err != nil {
		logger.WithError(err).Error("Failed to write JSON response")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}

func (c *ClickController) Complete(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	logger.Info("Click Complete request received")

	dto, err := composables.UseForm(&clickapi.CompleteRequest{}, r)
	if err != nil {
		logger.WithError(err).Error("Form decode error in Complete")
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
		logger.WithError(err).WithField("merchant_trans_id", dto.MerchantTransId).Error("Failed to get transaction")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if len(entities) != 1 {
		logger.WithFields(logrus.Fields{
			"merchant_trans_id": dto.MerchantTransId,
			"count":             len(entities),
		}).Error("Unexpected number of transactions found")
		http.Error(w, "Transaction not found or ambiguous", http.StatusBadRequest)
		return
	}

	entity := entities[0]

	if entity.Gateway() != billing.Click {
		logger.WithFields(logrus.Fields{
			"expected_gateway": billing.Click,
			"actual_gateway":   entity.Gateway(),
		}).Error("Invalid gateway")
		http.Error(w, "Invalid gateway", http.StatusBadRequest)
		return
	}

	clickDetails, ok := entity.Details().(details.ClickDetails)
	if !ok {
		logger.Error("Details is not of type ClickDetails")
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
		logger.WithField("click_trans_id", clickDetails.PaymentID()).Error("Invalid signature in Complete request")
		http.Error(w, "Invalid signature", http.StatusBadRequest)
		return
	}

	oldStatus := entity.Status()

	// Determine status based on error code: 0 = success, any other value = error
	// https://docs.click.uz/click-api-error/
	newStatus := billing.Completed
	if dto.Error != 0 {
		newStatus = billing.Failed
		logger.WithFields(logrus.Fields{
			"error_code":        dto.Error,
			"error_note":        dto.ErrorNote,
			"merchant_trans_id": dto.MerchantTransId,
		}).Warn("Click Complete received error from gateway")
	}

	entity = entity.
		SetStatus(newStatus).
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
		logger.WithError(err).WithField("merchant_trans_id", dto.MerchantTransId).Error("Failed to update transaction")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	// Invoke callback for notification (non-blocking)
	if err := c.billingService.InvokeCallback(r.Context(), entity); err != nil {
		logger.WithError(err).WithField("merchant_trans_id", dto.MerchantTransId).Warn("Callback error on status change")
	}

	clickDetails, ok = entity.Details().(details.ClickDetails)
	if !ok {
		logger.Error("Details is not of type ClickDetails after save")
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

	logger.WithFields(logrus.Fields{
		"click_trans_id":    clickDetails.PaymentID(),
		"merchant_trans_id": clickDetails.MerchantTransID(),
		"old_status":        oldStatus,
		"new_status":        newStatus,
	}).Info("Click Complete transaction processed")

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		logger.WithError(err).Error("Failed to write JSON response in Complete")
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
}
