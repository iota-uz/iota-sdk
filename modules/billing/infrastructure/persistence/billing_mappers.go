package persistence

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/billing"
	"github.com/iota-uz/iota-sdk/modules/billing/domain/aggregates/details"
	"github.com/iota-uz/iota-sdk/modules/billing/infrastructure/persistence/models"
)

func ToDomainTransaction(dbRow *models.Transaction) (billing.Transaction, error) {
	transactionID, err := uuid.Parse(dbRow.ID)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	gateway := billing.Gateway(dbRow.Gateway)

	d, err := fromDbDetails(gateway, dbRow.Details)
	if err != nil {
		return nil, fmt.Errorf("failed to parse details: %w", err)
	}

	return billing.New(
		dbRow.Quantity,
		billing.Currency(dbRow.Currency),
		billing.Gateway(dbRow.Gateway),
		d,
		billing.WithID(transactionID),
		billing.WithStatus(billing.Status(dbRow.Status)),
		billing.WithCreatedAt(dbRow.CreatedAt),
		billing.WithUpdatedAt(dbRow.UpdatedAt),
	), nil
}

func ToDBTransaction(entity billing.Transaction) (*models.Transaction, error) {
	d, err := toDbDetails(entity.Details())
	if err != nil {
		return nil, fmt.Errorf("failed to serialize details: %w", err)
	}

	return &models.Transaction{
		ID:        entity.ID().String(),
		Status:    string(entity.Status()),
		Quantity:  entity.Amount().Quantity(),
		Currency:  string(entity.Amount().Currency()),
		Gateway:   string(entity.Gateway()),
		Details:   d,
		CreatedAt: entity.CreatedAt(),
		UpdatedAt: entity.UpdatedAt(),
	}, nil
}

func fromDbDetails(gateway billing.Gateway, data json.RawMessage) (details.Details, error) {
	switch gateway {
	case billing.Click:
		var d models.ClickDetails
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		clickDetails := details.NewClickDetails(
			d.MerchantTransID,
			details.ClickWithServiceID(d.ServiceID),
			details.ClickWithMerchantID(d.MerchantID),
			details.ClickWithMerchantUserID(d.MerchantUserID),
			details.ClickWithMerchantPrepareID(d.MerchantPrepareID),
			details.ClickWithMerchantConfirmID(d.MerchantConfirmID),
			details.ClickWithPayDocId(d.PayDocId),
			details.ClickWithPaymentID(d.PaymentID),
			details.ClickWithPaymentStatus(d.PaymentStatus),
			details.ClickWithSignTime(d.SignTime),
			details.ClickWithSignString(d.SignString),
			details.ClickWithErrorCode(d.ErrorCode),
			details.ClickWithErrorNote(d.ErrorNote),
			details.ClickWithLink(d.Link),
			details.ClickWithParams(d.Params),
		)
		return clickDetails, nil

	case billing.Payme:
		var d models.PaymeDetails
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		receivers := make([]details.PaymeReceiver, len(d.Receivers))
		for i, r := range d.Receivers {
			receivers[i] = details.NewPaymeReceiver(
				r.ID,
				r.Amount,
			)
		}
		return details.NewPaymeDetails(
			d.Transaction,
			details.PaymeWithMerchantID(d.MerchantID),
			details.PaymeWithID(d.ID),
			details.PaymeWithState(d.State),
			details.PaymeWithTime(d.Time),
			details.PaymeWithCreatedTime(d.CreatedTime),
			details.PaymeWithPerformTime(d.PerformTime),
			details.PaymeWithCancelTime(d.CancelTime),
			details.PaymeWithAccount(d.Account),
			details.PaymeWithReceivers(receivers),
			details.PaymeWithAdditional(d.Additional),
			details.PaymeWithReason(d.Reason),
			details.PaymeWithErrorCode(d.ErrorCode),
			details.PaymeWithLink(d.Link),
			details.PaymeWithParams(d.Params),
		), nil

	case billing.Octo:
		var d models.OctoDetails
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		return details.NewOctoDetails(), nil

	case billing.Stripe:
		var d models.StripeDetails
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}
		return details.NewStripeDetails(), nil

	default:
		return nil, fmt.Errorf("unsupported gateway: %s", gateway)
	}
}

func toDbDetails(data details.Details) (json.RawMessage, error) {
	switch d := data.(type) {
	case details.ClickDetails:
		return json.Marshal(&models.ClickDetails{
			ServiceID:         d.ServiceID(),
			MerchantID:        d.MerchantID(),
			MerchantUserID:    d.MerchantUserID(),
			MerchantTransID:   d.MerchantTransID(),
			MerchantPrepareID: d.MerchantPrepareID(),
			MerchantConfirmID: d.MerchantConfirmID(),
			PayDocId:          d.PayDocId(),
			PaymentID:         d.PaymentID(),
			PaymentStatus:     d.PaymentStatus(),
			SignTime:          d.SignTime(),
			SignString:        d.SignString(),
			ErrorCode:         d.ErrorCode(),
			ErrorNote:         d.ErrorNote(),
			Link:              d.Link(),
			Params:            d.Params(),
		})

	case details.PaymeDetails:
		receivers := make([]models.PaymeReceiver, len(d.Receivers()))
		for i, r := range d.Receivers() {
			receivers[i] = models.PaymeReceiver{
				ID:     r.ID(),
				Amount: r.Amount(),
			}
		}
		return json.Marshal(&models.PaymeDetails{
			MerchantID:  d.MerchantID(),
			ID:          d.ID(),
			Transaction: d.Transaction(),
			State:       d.State(),
			Time:        d.Time(),
			CreatedTime: d.CreatedTime(),
			PerformTime: d.PerformTime(),
			CancelTime:  d.CancelTime(),
			Account:     d.Account(),
			Receivers:   receivers,
			Additional:  d.Additional(),
			Reason:      d.Reason(),
			ErrorCode:   d.ErrorCode(),
			Link:        d.Link(),
			Params:      d.Params(),
		})
	//
	//case details.OctoDetails:
	//	return json.Marshal(&models.OctoDetails{})
	//
	//case details.StripeDetails:
	//	return json.Marshal(&models.StripeDetails{})

	default:
		return nil, fmt.Errorf("unsupported details type: %T", d)
	}
}
