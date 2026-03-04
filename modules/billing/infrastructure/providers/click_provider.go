package providers

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"
	"time"

	clickapi "github.com/iota-uz/click"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
)

type ClickConfig struct {
	URL            string
	ServiceID      int64
	SecretKey      string
	MerchantID     int64
	MerchantUserID int64
}

// NewClickProvider creates a new Click provider with the given configuration.
func NewClickProvider(
	config ClickConfig,
) billing.Provider {
	apiCfg := clickapi.NewConfiguration()
	apiCfg.HTTPClient = &http.Client{
		Timeout: 30 * time.Second,
	}
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

// Gateway returns the Click gateway.
func (p *clickProvider) Gateway() billing.Gateway {
	return billing.Click
}

// Create generates a payment link for Click.
func (p *clickProvider) Create(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	const op serrors.Op = "clickProvider.Create"
	if t.Amount().Currency() != billing.UZS {
		return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("click can work only with UZS currency, provided: %s", t.Amount().Currency()))
	}
	if t.Status() != billing.Created {
		return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("transaction status must be 'created', provided: %s", t.Status()))
	}

	clickDetails, err := toClickDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
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

// Cancel reverses a Click payment.
func (p *clickProvider) Cancel(ctx context.Context, t billing.Transaction) (billing.Transaction, error) {
	const op serrors.Op = "clickProvider.Cancel"
	clickDetails, err := toClickDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if clickDetails.PaymentID() == 0 {
		return nil, serrors.E(op, serrors.Invalid, "cannot cancel: click payment_id not found in details")
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
		return nil, serrors.E(op, err)
	}

	if resp.GetErrorCode() != 0 {
		return nil, serrors.E(op, serrors.Internal, fmt.Sprintf("click error: %d - %s", resp.GetErrorCode(), resp.GetErrorNote()))
	}

	return t.SetStatus(billing.Canceled), nil
}

// Refund processes a partial or full refund for Click.
func (p *clickProvider) Refund(ctx context.Context, t billing.Transaction, amount float64) (billing.Transaction, error) {
	const op serrors.Op = "clickProvider.Refund"
	clickDetails, err := toClickDetails(t.Details())
	if err != nil {
		return nil, serrors.E(op, err)
	}

	if amount <= 0 {
		return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid refund amount: %f. Amount must be positive", amount))
	}

	if amount > t.Amount().Quantity()+0.001 {
		return nil, serrors.E(op, serrors.Invalid, fmt.Sprintf("invalid refund amount: %f. Amount exceeds transaction total: %f", amount, t.Amount().Quantity()))
	}

	if clickDetails.PaymentID() == 0 {
		return nil, serrors.E(op, serrors.Invalid, "cannot refund: click payment_id not found in details")
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
		return nil, serrors.E(op, err)
	}

	if resp.GetErrorCode() != 0 {
		return nil, serrors.E(op, serrors.Internal, fmt.Sprintf("click error: %d - %s", resp.GetErrorCode(), resp.GetErrorNote()))
	}

	totalRefunded := clickDetails.RefundedSum() + amount
	clickDetails = clickDetails.SetRefundedSum(totalRefunded)

	newStatus := billing.PartiallyRefunded
	// Using a small epsilon for floating point comparison of currency values
	if totalRefunded >= t.Amount().Quantity()-0.001 {
		newStatus = billing.Refunded
	}

	return t.SetDetails(clickDetails).SetStatus(newStatus), nil
}

func toClickDetails(detailsObj details.Details) (details.ClickDetails, error) {
	clickDetails, ok := detailsObj.(details.ClickDetails)
	if !ok {
		return nil, fmt.Errorf("failed to cast details to ClickDetails: invalid type %T", detailsObj)
	}
	return clickDetails, nil
}
