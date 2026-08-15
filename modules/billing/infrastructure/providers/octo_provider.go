// Package providers provides this package.
package providers

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/pkg/middleware"
	octoapi "github.com/iota-uz/octo"
)

type OctoConfig struct {
	OctoShopID int32
	OctoSecret string
	NotifyURL  string
}

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

func (o *octoProvider) Gateway() billing.Gateway {
	return billing.Octo
}

func (o *octoProvider) Create(ctx context.Context, t billing.Transaction) (billing.Transaction, error) {
	octoDetails, err := toOctoDetails(t.Details())
	if err != nil {
		return nil, err
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
		return nil, err
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

func (o *octoProvider) Refund(ctx context.Context, t billing.Transaction, quantity float64) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

// CheckStatus checks the current status of a transaction via Octo's API.
// This is used after responding with capture to get the final transaction status.
// Implements the billing.StatusChecker interface.
func (o *octoProvider) CheckStatus(ctx context.Context, shopTransactionID string) (*billing.StatusCheckResult, error) {
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
		return nil, fmt.Errorf("failed to check status: %w", err)
	}

	if resp.GetError() != 0 {
		return nil, fmt.Errorf("octo check status error: %s", resp.GetErrMessage())
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
		return nil, fmt.Errorf("failed to cast details to OctoDetails: invalid type %T", detailsObj)
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
