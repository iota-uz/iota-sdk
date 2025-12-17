package providers

import (
	"context"
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

	apiClient := newApiClient(o.logger)

	initTime := t.CreatedAt().Format("2006-01-02 15:04:05")

	req := octoapi.PreparePaymentRequest{
		OctoShopId:        o.config.OctoShopID,
		OctoSecret:        o.config.OctoSecret,
		ShopTransactionId: octoDetails.ShopTransactionId(),
		InitTime:          initTime,
		AutoCapture:       octoDetails.AutoCapture(),
		Test:              octoDetails.Test(),
		TotalSum:          t.Amount().Quantity(),
		Currency:          string(t.Amount().Currency()),
		Description:       octoDetails.Description(),
		ReturnUrl:         octoDetails.ReturnUrl(),
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
		SetOctoShopId(o.config.OctoShopID).
		SetNotifyUrl(o.config.NotifyURL)

	if resp.GetError() != 0 {
		octoDetails = octoDetails.
			SetError(resp.GetError()).
			SetErrMessage(resp.GetErrMessage())
	} else {
		octoDetails = octoDetails.
			SetOctoPaymentUUID(resp.Data.GetOctoPaymentUUID()).
			SetStatus(resp.Data.GetStatus()).
			SetOctoPayUrl(resp.Data.GetOctoPayUrl()).
			SetRefundedSum(resp.Data.GetRefundedSum())
	}

	t = t.SetDetails(octoDetails)

	return t, nil
}

func (o *octoProvider) Cancel(ctx context.Context, t billing.Transaction) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (o *octoProvider) Refund(ctx context.Context, t billing.Transaction, quantity float64) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

// CheckStatus checks the current status of a transaction via Octo's API.
// This is used after responding with capture to get the final transaction status.
// Implements the billing.StatusChecker interface.
func (o *octoProvider) CheckStatus(ctx context.Context, shopTransactionId string) (*billing.StatusCheckResult, error) {
	apiClient := newApiClient(o.logger)

	req := octoapi.NewCheckStatusRequest(o.config.OctoShopID, o.config.OctoSecret, shopTransactionId)

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
		OctoPaymentUUID:   resp.Data.GetOctoPaymentUUID(),
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

func newApiClient(logTransport *middleware.LogTransport) *octoapi.APIClient {
	configuration := octoapi.NewConfiguration()
	configuration.HTTPClient = &http.Client{
		Transport: logTransport,
	}

	apiClient := octoapi.NewAPIClient(configuration)

	return apiClient
}
