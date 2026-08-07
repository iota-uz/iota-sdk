// Package providers provides this package.
package providers

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
)

// DefaultClickAPIURL is the Click Merchant API base used when ClickConfig.APIURL
// is empty. https://docs.click.uz/en/merchant-api-request/
const DefaultClickAPIURL = "https://api.click.uz/v2/merchant"

// ErrClickPartialRefund is returned when a refund asks for less than the full
// transaction amount. The Click reversal endpoint takes no amount: it reverses
// the whole payment or nothing.
var ErrClickPartialRefund = errors.New("click supports full reversal only, not partial refund")

// ErrClickPaymentIDMissing is returned when the transaction carries no Click
// payment identifier, which happens when the payment never reached the Complete
// callback. Nothing was captured, so there is nothing to reverse.
var ErrClickPaymentIDMissing = errors.New("click transaction has no payment id to reverse")

// ClickReversalError is a rejection reported by Click itself, carrying the
// gateway's own code so the caller can tell "already reversed" from "refused by
// the card processor" instead of collapsing both into a generic failure.
type ClickReversalError struct {
	Code int32
	Note string
}

func (e ClickReversalError) Error() string {
	return fmt.Sprintf("click reversal rejected: code=%d note=%s", e.Code, e.Note)
}

type ClickConfig struct {
	URL            string
	ServiceID      int64
	SecretKey      string
	MerchantID     int64
	MerchantUserID int64
	// APIURL overrides the Merchant API base (sandbox, or a recording proxy in
	// tests). Empty means DefaultClickAPIURL.
	APIURL string
	// HTTPClient performs Merchant API calls. Supply one wrapping
	// middleware.LogTransport to get the outbound request into the request log.
	HTTPClient *http.Client
}

func NewClickProvider(
	config ClickConfig,
) billing.Provider {
	return &clickProvider{
		config: config,
	}
}

type clickProvider struct {
	config ClickConfig
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
	//TODO implement me
	panic("implement me")
}

// Refund reverses a completed Click payment through the Merchant API.
//
// Click exposes reversal as DELETE payment/reversal/{service_id}/{payment_id};
// there is no amount in the request, so only a full reversal exists. Click also
// refuses a payment from the previous month on any day but the first, and the
// card processor can refuse independently — both come back as a
// ClickReversalError with the gateway's own code rather than as a transport
// failure, because they are business answers, not outages.
func (p *clickProvider) Refund(ctx context.Context, t billing.Transaction, quantity float64) (billing.Transaction, error) {
	clickDetails, err := toClickDetails(t.Details())
	if err != nil {
		return nil, err
	}

	if quantity < t.Amount().Quantity() {
		return nil, ErrClickPartialRefund
	}

	paymentID := clickDetails.PaymentID()
	if paymentID == 0 {
		return nil, ErrClickPaymentIDMissing
	}

	serviceID := clickDetails.ServiceID()
	if serviceID == 0 {
		serviceID = p.config.ServiceID
	}

	response, err := p.reverse(ctx, serviceID, paymentID)
	if err != nil {
		return nil, err
	}

	clickDetails = clickDetails.
		SetErrorCode(response.ErrorCode).
		SetErrorNote(response.ErrorNote)

	if response.ErrorCode < 0 {
		return nil, ClickReversalError{Code: response.ErrorCode, Note: response.ErrorNote}
	}

	return t.SetDetails(clickDetails).SetStatus(billing.Refunded), nil
}

type clickReversalResponse struct {
	ErrorCode int32  `json:"error_code"`
	ErrorNote string `json:"error_note"`
	PaymentID int64  `json:"payment_id"`
}

func (p *clickProvider) reverse(ctx context.Context, serviceID, paymentID int64) (clickReversalResponse, error) {
	var response clickReversalResponse

	base := p.config.APIURL
	if base == "" {
		base = DefaultClickAPIURL
	}
	endpoint := fmt.Sprintf("%s/payment/reversal/%d/%d", base, serviceID, paymentID)

	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, endpoint, nil)
	if err != nil {
		return response, err
	}
	request.Header.Set("Auth", p.authHeader(time.Now()))
	request.Header.Set("Accept", "application/json")

	client := p.config.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}

	httpResponse, err := client.Do(request)
	if err != nil {
		return response, err
	}
	defer func() {
		_ = httpResponse.Body.Close()
	}()

	body, err := io.ReadAll(httpResponse.Body)
	if err != nil {
		return response, err
	}
	if httpResponse.StatusCode != http.StatusOK {
		return response, fmt.Errorf("click reversal returned http %d: %s", httpResponse.StatusCode, string(body))
	}
	if err := json.Unmarshal(body, &response); err != nil {
		return response, fmt.Errorf("failed to decode click reversal response %q: %w", string(body), err)
	}

	return response, nil
}

// authHeader builds the Merchant API credential:
// merchant_user_id:sha1(timestamp + secret_key):timestamp.
func (p *clickProvider) authHeader(now time.Time) string {
	timestamp := strconv.FormatInt(now.Unix(), 10)
	// SHA-1 is what the Merchant API specifies; the algorithm is not ours to choose.
	digest := sha1.Sum([]byte(timestamp + p.config.SecretKey)) //nolint:gosec // required by the Click Merchant API
	return fmt.Sprintf("%d:%s:%s", p.config.MerchantUserID, hex.EncodeToString(digest[:]), timestamp)
}

func toClickDetails(detailsObj details.Details) (details.ClickDetails, error) {
	clickDetails, ok := detailsObj.(details.ClickDetails)
	if !ok {
		return nil, fmt.Errorf("failed to cast details to ClickDetails: invalid type %T", detailsObj)
	}
	return clickDetails, nil
}
