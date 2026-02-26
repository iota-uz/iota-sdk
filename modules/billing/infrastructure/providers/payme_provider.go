package providers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math"
	"net/http"

	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	paymeapi "github.com/iota-uz/payme"
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
		config:     config,
		httpClient: &http.Client{},
	}
}

type paymeProvider struct {
	config     PaymeConfig
	httpClient *http.Client
}

func (p *paymeProvider) Gateway() billing.Gateway {
	return billing.Payme
}

func (p *paymeProvider) doRequest(ctx context.Context, method string, params any, result any) error {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.config.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}

	auth := base64.StdEncoding.EncodeToString([]byte(p.config.User + ":" + p.config.SecretKey))
	req.Header.Set("Authorization", "Basic "+auth)
	req.Header.Set("Content-Type", "application/json")

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var rpcResp struct {
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
		Result json.RawMessage `json:"result"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&rpcResp); err != nil {
		return err
	}

	if rpcResp.Error != nil {
		return fmt.Errorf("payme error %d: %s", rpcResp.Error.Code, rpcResp.Error.Message)
	}

	if result != nil {
		return json.Unmarshal(rpcResp.Result, result)
	}

	return nil
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
	params["cr"] = string(t.Amount().Currency())

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

func (p *paymeProvider) Cancel(ctx context.Context, t billing.Transaction) (billing.Transaction, error) {
	paymeDetails, err := toPaymeDetails(t.Details())
	if err != nil {
		return nil, err
	}

	if paymeDetails.ID() == "" {
		return nil, fmt.Errorf("cannot cancel: payme transaction id not found in details")
	}

	var result struct {
		CancelTime int64 `json:"cancel_time"`
		State      int32 `json:"state"`
	}

	params := map[string]any{
		"id":     paymeDetails.ID(),
		"reason": 2, // merchant cancel
	}

	if err := p.doRequest(ctx, "CancelTransaction", params, &result); err != nil {
		return nil, err
	}

	paymeDetails = paymeDetails.
		SetCancelTime(result.CancelTime).
		SetState(result.State)

	return t.SetDetails(paymeDetails).SetStatus(billing.Canceled), nil
}

func (p *paymeProvider) Refund(ctx context.Context, t billing.Transaction, _ float64) (billing.Transaction, error) {
	paymeDetails, err := toPaymeDetails(t.Details())
	if err != nil {
		return nil, err
	}

	if paymeDetails.ID() == "" {
		return nil, fmt.Errorf("cannot refund: payme transaction id not found in details")
	}

	var result struct {
		CancelTime int64 `json:"cancel_time"`
		State      int32 `json:"state"`
	}

	params := map[string]any{
		"id":     paymeDetails.ID(),
		"reason": 5, // return goods (refund)
	}

	if err := p.doRequest(ctx, "CancelTransaction", params, &result); err != nil {
		return nil, err
	}

	paymeDetails = paymeDetails.
		SetCancelTime(result.CancelTime).
		SetState(result.State)

	return t.SetDetails(paymeDetails).SetStatus(billing.Refunded), nil
}

func toPaymeDetails(detailsObj details.Details) (details.PaymeDetails, error) {
	paymeDetails, ok := detailsObj.(details.PaymeDetails)
	if !ok {
		return nil, fmt.Errorf("failed to cast details to PaymeDetails: invalid type %T", detailsObj)
	}
	return paymeDetails, nil
}
