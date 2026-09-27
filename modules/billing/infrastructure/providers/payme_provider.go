// Package providers provides this package.
package providers

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"math"
	"time"

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

// ErrPaymeCancelAfterCompletion is returned when a merchant asks to cancel a
// transaction Payme has already completed. Sending that money back is a
// refund, and Payme models it as its own operation with its own reason code —
// treating it as a cancellation here would mark captured money as never taken.
var ErrPaymeCancelAfterCompletion = errors.New("payme transaction is completed; cancelling it would be a refund")

// Cancel voids a checkout the customer has not paid, and does so without
// calling Payme.
//
// That is not a shortcut: Payme's merchant protocol has no cancel a merchant
// can send. Every transaction operation is Payme calling us —
// CreateTransaction, PerformTransaction, CancelTransaction — so the only thing
// a merchant can do about a link nobody used is stop honouring it. Which is
// exactly what this does: the transaction leaves the active set, and
// PaymeController.create resolves an account through activePaymeTransaction, so
// a customer opening the old link is answered with InvalidAccount instead of
// being allowed to pay something the sale has moved on from.
//
// A transaction Payme already created and performed is deliberately refused
// rather than voided — see ErrPaymeCancelAfterCompletion.
//
// The customer who opened their transaction *before* the cancellation is the
// case this leaves to PaymeController.perform: a perform resolves by
// transaction id, so nothing here can stop the call arriving. What the state
// written below does is give that handler something to refuse on, and it
// refuses — an unconfirmed perform is money Payme returns, whereas accepting it
// is money taken for an order that has moved on.
func (p *paymeProvider) Cancel(_ context.Context, t billing.Transaction) (billing.Transaction, error) {
	paymeDetails, err := toPaymeDetails(t.Details())
	if err != nil {
		return nil, err
	}

	if paymeDetails.State() == paymeapi.TransactionStateCompleted || t.Status() == billing.Completed {
		return nil, ErrPaymeCancelAfterCompletion
	}

	if paymeDetails.CancelTime() == 0 {
		paymeDetails = paymeDetails.SetCancelTime(time.Now().UnixMilli())
	}

	// Reason is left as Payme set it. Its vocabulary describes why *Payme*
	// cancelled a transaction (recipient not found, debit error, timeout); a
	// merchant abandoning its own checkout is none of those, and picking the
	// nearest code would put a fact into the record that Payme never stated.
	return t.
		SetStatus(billing.Canceled).
		SetDetails(paymeDetails.SetState(paymeapi.TransactionStateCancelledBeforeCompletion)), nil
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
