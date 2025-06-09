package providers

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	paymeapi "github.com/iota-uz/payme"
	"math"
)

type PaymeConfig struct {
	URL        string
	SecretKey  string
	MerchantID string
	User       string
}

func NewPaymeProvider(
	config PaymeConfig,
) billing.Provider {
	return &paymeProvider{
		config: config,
	}
}

type paymeProvider struct {
	config PaymeConfig
}

func (p *paymeProvider) Gateway() billing.Gateway {
	return billing.Payme
}

func (p *paymeProvider) Create(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	paymeDetails, err := toPaymeDetails(t.Details())
	if err != nil {
		return nil, err
	}

	params := paymeDetails.Params()
	params["m"] = p.config.MerchantID
	for k, v := range paymeDetails.Account() {
		params["ac."+k] = v
	}
	params["a"] = int64(math.Ceil(t.Amount().Quantity() * 100))
	params["cr"] = t.Amount().Currency()

	var linkData string
	for k, v := range params {
		linkData += fmt.Sprintf("%s=%v;", k, v)
	}
	linkData = linkData[:len(linkData)-1]
	encodedData := base64.StdEncoding.EncodeToString([]byte(linkData))

	link := fmt.Sprintf("%s/%s", p.config.URL, encodedData)

	t = t.SetDetails(
		paymeDetails.
			SetMerchantID(p.config.MerchantID).
			SetState(paymeapi.TransactionStateCreated).
			SetLink(link).
			SetParams(params),
	)

	return t, nil
}

func (p *paymeProvider) Cancel(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func (p *paymeProvider) Refund(_ context.Context, t billing.Transaction, quantity float64) (billing.Transaction, error) {
	//TODO implement me
	panic("implement me")
}

func toPaymeDetails(detailsObj details.Details) (details.PaymeDetails, error) {
	paymeDetails, ok := detailsObj.(details.PaymeDetails)
	if !ok {
		return nil, fmt.Errorf("failed to cast details to PaymeDetails: invalid type %T", detailsObj)
	}
	return paymeDetails, nil
}
