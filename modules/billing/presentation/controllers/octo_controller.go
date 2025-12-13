package controllers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/services"
	"github.com/iota-uz/iota-sdk/pkg/application"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
	"github.com/iota-uz/iota-sdk/pkg/di"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	octoapi "github.com/iota-uz/octo"
	octoauth "github.com/iota-uz/octo/auth"
	"github.com/sirupsen/logrus"
)

const (
	// HTTP error messages
	errMsgInvalidJSON         = "invalid JSON"
	errMsgInternalServerError = "Internal Server Error"
	errMsgInvalidDetailsType  = "invalid details type"
	errMsgTransactionNotFound = "transaction not found or ambiguous"
)

type OctoController struct {
	app            application.Application
	billingService *services.BillingService
	octo           configuration.OctoOptions
	basePath       string
	logTransport   *middleware.LogTransport
}

func NewOctoController(
	app application.Application,
	octo configuration.OctoOptions,
	basePath string,
	logTransport *middleware.LogTransport,
) application.Controller {
	return &OctoController{
		app:            app,
		billingService: app.Service(services.BillingService{}).(*services.BillingService),
		octo:           octo,
		basePath:       basePath,
		logTransport:   logTransport,
	}
}

func (c *OctoController) Register(r *mux.Router) {
	router := r.PathPrefix(c.basePath).Subrouter()
	router.HandleFunc("", di.H(c.Handle)).Methods(http.MethodPost)
}

func (c *OctoController) Key() string {
	return c.basePath
}

func (c *OctoController) Handle(
	r *http.Request,
	w http.ResponseWriter,
	logger *logrus.Entry,
) {
	logger.Info("Octo notification received")

	// Parse and validate notification
	notification, err := c.parseNotification(r, logger)
	if err != nil {
		http.Error(w, errMsgInvalidJSON, http.StatusBadRequest)
		return
	}

	if err := c.validateSignature(&notification, logger); err != nil {
		http.Error(w, errMsgInternalServerError, http.StatusInternalServerError)
		return
	}

	// Find transaction
	entity, err := c.findTransaction(r.Context(), &notification, logger)
	if err != nil {
		statusCode := http.StatusInternalServerError
		errMsg := errMsgInternalServerError
		if err.Error() == errMsgTransactionNotFound {
			statusCode = http.StatusBadRequest
			errMsg = errMsgTransactionNotFound
		}
		http.Error(w, errMsg, statusCode)
		return
	}

	// Update transaction from notification
	entity, err = c.updateTransactionFromNotification(r.Context(), entity, &notification, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to update transaction from notification")

		// Mark transaction as failed
		entity = entity.SetStatus(billing.Failed)
		if _, saveErr := c.billingService.Save(r.Context(), entity); saveErr != nil {
			logger.WithError(saveErr).Error("Failed to save failed transaction status")
		}

		// Send cancel response to Octo
		if respErr := c.sendCallbackResponse(w, entity, logger); respErr != nil {
			logger.WithError(respErr).Error("Failed to send cancel response to Octo")
		}
		return
	}

	// Handle callback for pending transactions
	entity, err = c.handleCallback(r.Context(), entity, &notification, logger)
	if err != nil {
		logger.WithError(err).Error("Failed to handle callback")

		// Mark transaction as failed
		entity = entity.SetStatus(billing.Failed)
		if _, saveErr := c.billingService.Save(r.Context(), entity); saveErr != nil {
			logger.WithError(saveErr).Error("Failed to save failed transaction status")
		}

		// Send cancel response to Octo
		if respErr := c.sendCallbackResponse(w, entity, logger); respErr != nil {
			logger.WithError(respErr).Error("Failed to send cancel response to Octo")
		}
		return
	}

	// Send response to Octo
	if err := c.sendCallbackResponse(w, entity, logger); err != nil {
		logger.WithError(err).Error("Failed to write Octo callback response")
	}
}

// parseNotification decodes and validates the incoming JSON notification
func (c *OctoController) parseNotification(r *http.Request, logger *logrus.Entry) (octoapi.NotificationRequest, error) {
	var notification octoapi.NotificationRequest
	if err := json.NewDecoder(r.Body).Decode(&notification); err != nil {
		logger.WithError(err).Error("Failed to decode Octo notification JSON")
		return notification, err
	}
	return notification, nil
}

// validateSignature verifies the webhook signature
func (c *OctoController) validateSignature(notification *octoapi.NotificationRequest, logger *logrus.Entry) error {
	if !octoauth.ValidateSignature(
		notification.Signature,
		c.octo.OctoSecretHash,
		notification.OctoPaymentUUID,
		notification.Status,
	) {
		logger.WithFields(logrus.Fields{
			"octo_payment_uuid": notification.OctoPaymentUUID,
			"status":            notification.Status,
		}).Error("Failed to validate Octo signature")
		return fmt.Errorf("invalid signature")
	}
	return nil
}

// findTransaction locates the transaction by notification details
func (c *OctoController) findTransaction(
	ctx context.Context,
	notification *octoapi.NotificationRequest,
	logger *logrus.Entry,
) (billing.Transaction, error) {
	entities, err := c.billingService.GetByDetailsFields(
		ctx,
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
		logger.WithError(err).WithFields(logrus.Fields{
			"shop_transaction_id": notification.ShopTransactionId,
			"octo_payment_uuid":   notification.OctoPaymentUUID,
		}).Error("Failed to get transaction")
		return nil, err
	}

	if len(entities) != 1 {
		logger.WithFields(logrus.Fields{
			"shop_transaction_id": notification.ShopTransactionId,
			"octo_payment_uuid":   notification.OctoPaymentUUID,
			"count":               len(entities),
		}).Error("Unexpected number of transactions found")
		return nil, errors.New(errMsgTransactionNotFound)
	}

	return entities[0], nil
}

// updateTransactionFromNotification updates transaction details and status based on notification
func (c *OctoController) updateTransactionFromNotification(
	ctx context.Context,
	entity billing.Transaction,
	notification *octoapi.NotificationRequest,
	logger *logrus.Entry,
) (billing.Transaction, error) {
	octoDetails, ok := entity.Details().(details.OctoDetails)
	if !ok {
		logger.Error("Details is not of type OctoDetails")
		return nil, errors.New(errMsgInvalidDetailsType)
	}

	// Update details with notification data
	octoDetails = c.updateOctoDetailsFromNotification(octoDetails, notification)

	// Map Octo status to billing status
	oldStatus := entity.Status()
	entity = c.mapNotificationStatusToEntity(entity, notification.Status)

	entity = entity.SetDetails(octoDetails)
	updatedEntity, err := c.billingService.Save(ctx, entity)
	if err != nil {
		logger.WithError(err).WithField("octo_payment_uuid", notification.OctoPaymentUUID).
			Error("Failed to update transaction")
		return nil, err
	}

	logger.WithFields(logrus.Fields{
		"octo_payment_uuid":   notification.OctoPaymentUUID,
		"old_status":          oldStatus,
		"new_status":          updatedEntity.Status(),
		"notification_status": notification.Status,
	}).Info("Transaction status updated from Octo notification")

	return updatedEntity, nil
}

// updateOctoDetailsFromNotification updates OctoDetails with notification data
func (c *OctoController) updateOctoDetailsFromNotification(
	octoDetails details.OctoDetails,
	notification *octoapi.NotificationRequest,
) details.OctoDetails {
	return octoDetails.
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
}

// mapNotificationStatusToEntity maps Octo notification status to billing status
func (c *OctoController) mapNotificationStatusToEntity(
	entity billing.Transaction,
	octoStatus string,
) billing.Transaction {
	switch octoStatus {
	case octoapi.WaitingForCaptureStatus:
		return entity.SetStatus(billing.Pending)
	case octoapi.CancelledStatus:
		return entity.SetStatus(billing.Canceled)
	case octoapi.SucceededStatus:
		return entity.SetStatus(billing.Completed)
	default:
		return entity
	}
}

// handleCallback invokes the registered callback for pending transactions
func (c *OctoController) handleCallback(
	ctx context.Context,
	entity billing.Transaction,
	notification *octoapi.NotificationRequest,
	logger *logrus.Entry,
) (billing.Transaction, error) {
	// For WaitingForCapture status - blocking callback
	if notification.Status == octoapi.WaitingForCaptureStatus {
		if err := c.billingService.InvokeCallback(ctx, entity); err != nil {
			logger.WithError(err).WithField("octo_payment_uuid", notification.OctoPaymentUUID).
				Error("Callback error in Octo Handle")

			// Mark transaction as failed on callback error
			entity = entity.SetStatus(billing.Failed)
			updatedEntity, saveErr := c.billingService.Save(ctx, entity)
			if saveErr != nil {
				logger.WithError(saveErr).Error("Failed to save transaction after callback failure")
				return nil, saveErr
			}
			return updatedEntity, nil
		}
		return entity, nil
	}

	// For Succeeded and Cancelled statuses - non-blocking callback
	if notification.Status == octoapi.SucceededStatus || notification.Status == octoapi.CancelledStatus {
		if err := c.billingService.InvokeCallback(ctx, entity); err != nil {
			logger.WithError(err).WithField("octo_payment_uuid", notification.OctoPaymentUUID).
				Warn("Callback error on status change")
		}
	}

	return entity, nil
}

// sendCallbackResponse sends the final callback response to Octo
func (c *OctoController) sendCallbackResponse(
	w http.ResponseWriter,
	entity billing.Transaction,
	logger *logrus.Entry,
) error {
	octoDetails, ok := entity.Details().(details.OctoDetails)
	if !ok {
		logger.Error("Details is not of type OctoDetails in final response")
		http.Error(w, errMsgInvalidDetailsType, http.StatusInternalServerError)
		return errors.New(errMsgInvalidDetailsType)
	}

	// Determine final accept status for response
	acceptStatus := c.determineFinalAcceptStatus(octoDetails, entity.Status())

	callbackResponse := octoapi.CallbackResponse{
		AcceptStatus: &acceptStatus,
		FinalAmount:  octoapi.PtrFloat64(entity.Amount().Quantity()),
	}

	logger.WithFields(logrus.Fields{
		"octo_payment_uuid": octoDetails.OctoPaymentUUID(),
		"accept_status":     acceptStatus,
		"final_status":      entity.Status(),
	}).Info("Octo notification processed successfully")

	w.Header().Set("Content-Type", "application/json")
	return json.NewEncoder(w).Encode(callbackResponse)
}

// determineFinalAcceptStatus determines the final accept status for the callback response
func (c *OctoController) determineFinalAcceptStatus(
	_ details.OctoDetails,
	status billing.Status,
) string {
	// For failed or canceled transactions, return cancel status
	if status == billing.Failed || status == billing.Canceled {
		return octoapi.CancelStatus
	}

	// Default to capture status
	return octoapi.CaptureStatus
}
