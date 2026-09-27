// Package providers provides this package.
package providers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	octoapi "github.com/iota-uz/octo"
)

type OctoConfig struct {
	OctoShopID int32
	OctoSecret string
	NotifyURL  string
}

// NewOctoProvider creates a new Octo provider with the given configuration.
func NewOctoProvider(
	config OctoConfig,
	logTransport *middleware.LogTransport,
) billing.Provider {
	return &octoProvider{
		config: config,
		logger: logTransport,
	}
}

type octoProvider struct {
	config OctoConfig
	logger *middleware.LogTransport
}

// Gateway returns the Octo gateway.
func (o *octoProvider) Gateway() billing.Gateway {
	return billing.Octo
}

// Create prepares a payment with Octo.
func (o *octoProvider) Create(ctx context.Context, t billing.Transaction) (billing.Transaction, error) {
	const op serrors.Op = "octoProvider.Create"
	octoDetails, err := toOctoDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	apiClient := newAPIClient(o.logger)

	initTime := t.CreatedAt().Format("2006-01-02 15:04:05")

	req := octoapi.PreparePaymentRequest{
		OctoShopId:        o.config.OctoShopID,
		OctoSecret:        o.config.OctoSecret,
		ShopTransactionId: octoDetails.ShopTransactionID(),
		InitTime:          initTime,
		AutoCapture:       octoDetails.AutoCapture(),
		Test:              octoDetails.Test(),
		TotalSum:          t.Amount().Quantity(),
		Currency:          string(t.Amount().Currency()),
		Description:       octoDetails.Description(),
		ReturnUrl:         octoDetails.ReturnURL(),
		NotifyUrl:         o.config.NotifyURL,
	}

	resp, httpResp, err := apiClient.PaymentsAPI.
		PreparePaymentPost(ctx).
		PreparePaymentRequest(req).
		Execute()

	if httpResp != nil {
		if hErr := httpResp.Body.Close(); hErr != nil {
			log.Printf("failed to close http response body: %v", hErr)
		}
	}

	if err != nil {
		return nil, serrors.E(op, serrors.Internal, err)
	}

	if resp.ApiMessageForDevelopers != nil {
		log.Printf("Octo ApiMessageForDevelopers: %s", *resp.ApiMessageForDevelopers)
	}

	octoDetails = octoDetails.
		SetInitTime(initTime).
		SetOctoShopID(o.config.OctoShopID).
		SetNotifyURL(o.config.NotifyURL)

	if resp.GetError() != 0 {
		octoDetails = octoDetails.
			SetError(resp.GetError()).
			SetErrMessage(resp.GetErrMessage())
	} else {
		octoDetails = octoDetails.
			SetOctoPaymentUUID(resp.Data.GetOctoPaymentUUID()).
			SetStatus(resp.Data.GetStatus()).
			SetOctoPayURL(resp.Data.GetOctoPayUrl()).
			SetRefundedSum(resp.Data.GetRefundedSum())
	}

	t = t.SetDetails(octoDetails)

	return t, nil
}

// ErrOctoCancelAfterCapture is returned when a merchant asks to cancel a
// payment Octo has already captured. Sending that money back is a refund — the
// TransactionManagement API's RefundPost — not a cancellation.
var ErrOctoCancelAfterCapture = errors.New("octo payment is captured; cancelling it would be a refund")

// Cancel abandons a payment the merchant no longer intends to take, and does so
// without calling Octo.
//
// Octo has no cancel endpoint: PreparePayment opens the payment, CheckStatus
// reads it, RefundPost sends captured money back, and SetAccept answers a hold.
// The status written here is what the merchant withdraws, and under manual
// capture (auto_capture=false, which is how a hold arises at all) it also
// releases the customer's money: Octo asks for the verdict on its notification,
// and OctoController.determineFinalAcceptStatus answers `cancel` for exactly
// the statuses below. So a customer who pays a link the merchant has abandoned
// has their authorisation released rather than captured — the hold is not
// stranded by writing this locally, it is resolved by it.
//
// A captured payment is refused rather than voided (ErrOctoCancelAfterCapture):
// its money exists, and recording it as never taken is how a refund gets
// skipped.
func (o *octoProvider) Cancel(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	octoDetails, err := toOctoDetails(t.Details())
	if err != nil {
		return nil, err
	}

	if octoDetails.Status() == octoapi.SucceededStatus || t.Status() == billing.Completed {
		return nil, ErrOctoCancelAfterCapture
	}

	return t.SetStatus(billing.Canceled), nil
}

// Refund processes a partial or full refund for Octo.
func (o *octoProvider) Refund(ctx context.Context, t billing.Transaction, amount float64) (billing.Transaction, error) {
	const op serrors.Op = "octoProvider.Refund"
	octoDetails, err := toOctoDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if octoDetails.OctoPaymentUUID() == "" {
		return nil, serrors.E(op, serrors.Invalid, "cannot refund: octo_payment_uuid not found in details")
	}

	apiClient := newAPIClient(o.logger)

	shopRefundID := fmt.Sprintf("ref_%s_%d", t.ID().String(), time.Now().UnixNano())

	req := octoapi.RefundRequest{
		OctoShopId:      o.config.OctoShopID,
		OctoSecret:      o.config.OctoSecret,
		OctoPaymentUUID: octoDetails.OctoPaymentUUID(),
		ShopRefundId:    shopRefundID,
		Amount:          amount,
	}

	resp, httpResp, err := apiClient.TransactionManagementAPI.
		RefundPost(ctx).
		RefundRequest(req).
		Execute()

	if httpResp != nil {
		if hErr := httpResp.Body.Close(); hErr != nil {
			log.Printf("failed to close http response body: %v", hErr)
		}
	}

	if err != nil {
		return nil, serrors.E(op, serrors.Internal, err)
	}

	if resp.GetError() != 0 {
		return nil, serrors.E(op, serrors.Internal, fmt.Sprintf("octo refund error: %s", resp.GetErrMessage()))
	}

	totalRefunded := octoDetails.RefundedSum() + amount
	if resp.Data != nil {
		octoDetails = octoDetails.SetRefundedSum(totalRefunded)
	}

	newStatus := billing.PartiallyRefunded
	if totalRefunded >= t.Amount().Quantity()-0.001 {
		newStatus = billing.Refunded
	}

	return t.SetDetails(octoDetails).SetStatus(newStatus), nil
}

// CheckStatus checks the current status of a transaction via Octo's API.
// This is used after responding with capture to get the final transaction status.
// Implements the billing.StatusChecker interface.
func (o *octoProvider) CheckStatus(ctx context.Context, shopTransactionID string) (*billing.StatusCheckResult, error) {
	const op serrors.Op = "octoProvider.CheckStatus"
	apiClient := newAPIClient(o.logger)

	req := octoapi.NewCheckStatusRequest(o.config.OctoShopID, o.config.OctoSecret, shopTransactionID)

	resp, httpResp, err := apiClient.StatusAPI.
		CheckStatus(ctx).
		CheckStatusRequest(*req).
		Execute()

	if httpResp != nil {
		if hErr := httpResp.Body.Close(); hErr != nil {
			log.Printf("failed to close http response body: %v", hErr)
		}
	}

	if err != nil {
		return nil, serrors.E(op, serrors.Internal, err)
	}

	if resp.GetError() != 0 {
		return nil, serrors.E(op, serrors.Internal, fmt.Sprintf("octo check status error: %s", resp.GetErrMessage()))
	}

	result := &billing.StatusCheckResult{
		Status:            resp.Data.GetStatus(),
		ShopTransactionID: resp.Data.GetShopTransactionId(),
		ProviderPaymentID: resp.Data.GetOctoPaymentUUID(),
	}

	return result, nil
}

func toOctoDetails(detailsObj details.Details) (details.OctoDetails, error) {
	octoDetails, ok := detailsObj.(details.OctoDetails)
	if !ok {
		return nil, serrors.E(serrors.Invalid, fmt.Sprintf("failed to cast details to OctoDetails: invalid type %T", detailsObj))
	}
	return octoDetails, nil
}

func newAPIClient(logTransport *middleware.LogTransport) *octoapi.APIClient {
	configuration := octoapi.NewConfiguration()
	configuration.HTTPClient = &http.Client{
		Transport: logTransport,
	}

	apiClient := octoapi.NewAPIClient(configuration)

	return apiClient
}
