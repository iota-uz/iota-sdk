package providers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"github.com/iota-uz/click"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
)

type ClickConfig struct {
	URL            string
	ServiceID      int64
	SecretKey      string
	MerchantID     int64
	MerchantUserID int64
}

func NewClickProvider(
	config ClickConfig,
) billing.Provider {
	apiCfg := clickapi.NewConfiguration()
	apiCfg.HTTPClient = &http.Client{}
	apiClient := clickapi.NewAPIClient(apiCfg)
	return &clickProvider{
		config:    config,
		apiClient: apiClient,
	}
}

type clickProvider struct {
	config    ClickConfig
	apiClient *clickapi.APIClient
}

func (p *clickProvider) Gateway() billing.Gateway {
	return billing.Click
}

func (p *clickProvider) Create(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	if t.Amount().Currency() != billing.UZS {
		return nil, fmt.Errorf("click can work only with UZS currency, provided: %s", t.Amount().Currency())
	}
	if t.Status() != billing.Created {
		return nil, fmt.Errorf("transaction status must be 'created', provided: %s", t.Status())
	}

	clickDetails, err := toClickDetails(t.Details())
	if err != nil {
		return nil, err
	}

	params := clickDetails.Params()
	params["service_id"] = p.config.ServiceID
	params["merchant_id"] = p.config.MerchantID
	params["merchant_user_id"] = p.config.MerchantUserID
	// https://docs.click.uz/click-button/ (format N.NN)
	params["amount"] = fmt.Sprintf("%.2f", t.Amount().Quantity())
	params["transaction_param"] = clickDetails.MerchantTransID()

	values := url.Values{}
	for key, value := range params {
		values.Set(key, fmt.Sprintf("%v", value))
	}

	link := fmt.Sprintf("%s/services/pay?%s", p.config.URL, values.Encode())

	t = t.SetDetails(
		clickDetails.
			SetServiceID(p.config.ServiceID).
			SetMerchantID(p.config.MerchantID).
			SetMerchantUserID(p.config.MerchantUserID).
			SetLink(link).
			SetParams(params),
	)

	return t, nil
}

func (p *clickProvider) Cancel(ctx context.Context, t billing.Transaction) (billing.Transaction, error) {
	clickDetails, err := toClickDetails(t.Details())
	if err != nil {
		return nil, err
	}

	if clickDetails.PaymentID() == 0 {
		return nil, fmt.Errorf("cannot cancel: click payment_id not found in details")
	}

	// Manual Basic Auth as ContextBasicAuth is missing from the SDK
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", p.config.ServiceID, p.config.SecretKey)))
	ctx = context.WithValue(ctx, clickapi.ContextAPIKeys, map[string]clickapi.APIKey{
		"Authorization": {
			Key:    "Basic " + auth,
			Prefix: "",
		},
	})

	resp, _, err := p.apiClient.PaymentAPI.ReversePayment(ctx, p.config.ServiceID, clickDetails.PaymentID()).Execute()
	if err != nil {
		return nil, err
	}

	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("click error: %d - %s", resp.GetErrorCode(), resp.GetErrorNote())
	}

	return t.SetStatus(billing.Canceled), nil
}

func (p *clickProvider) Refund(ctx context.Context, t billing.Transaction, amount float64) (billing.Transaction, error) {
	clickDetails, err := toClickDetails(t.Details())
	if err != nil {
		return nil, err
	}

	if clickDetails.PaymentID() == 0 {
		return nil, fmt.Errorf("cannot refund: click payment_id not found in details")
	}

	// Manual Basic Auth
	auth := base64.StdEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%s", p.config.ServiceID, p.config.SecretKey)))
	ctx = context.WithValue(ctx, clickapi.ContextAPIKeys, map[string]clickapi.APIKey{
		"Authorization": {
			Key:    "Basic " + auth,
			Prefix: "",
		},
	})

	resp, _, err := p.apiClient.PaymentAPI.PartialRefund(ctx, p.config.ServiceID, clickDetails.PaymentID(), amount).Execute()
	if err != nil {
		return nil, err
	}

	if resp.GetErrorCode() != 0 {
		return nil, fmt.Errorf("click error: %d - %s", resp.GetErrorCode(), resp.GetErrorNote())
	}

	newStatus := billing.PartiallyRefunded
	if amount >= t.Amount().Quantity() {
		newStatus = billing.Refunded
	}

	return t.SetStatus(newStatus), nil
}

func toClickDetails(detailsObj details.Details) (details.ClickDetails, error) {
	clickDetails, ok := detailsObj.(details.ClickDetails)
	if !ok {
		return nil, fmt.Errorf("failed to cast details to ClickDetails: invalid type %T", detailsObj)
	}
	return clickDetails, nil
}
