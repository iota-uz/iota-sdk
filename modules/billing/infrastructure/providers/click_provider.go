package providers

import (
	"context"
	"fmt"
	"net/url"

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
	return &ClickProvider{
		config: config,
	}
}

type ClickProvider struct {
	config ClickConfig
}

func (p *ClickProvider) Gateway() billing.Gateway {
	return billing.Click
}

func (p *ClickProvider) Create(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
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
	params["amount"] = t.Amount().Quantity()
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

func (p *ClickProvider) Cancel(ctx context.Context, t billing.Transaction) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (p *ClickProvider) Refund(ctx context.Context, t billing.Transaction, quantity float64) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func toClickDetails(detailsObj details.Details) (details.ClickDetails, error) {
	clickDetails, ok := detailsObj.(details.ClickDetails)
	if !ok {
		return nil, fmt.Errorf("failed to cast details to ClickDetails: invalid type %T", detailsObj)
	}
	return clickDetails, nil
}
