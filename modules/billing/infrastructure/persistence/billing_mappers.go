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

	tenantID, err := uuid.Parse(dbRow.TenantID)
	if err != nil {
		return nil, fmt.Errorf("invalid UUID: %w", err)
	}

	gateway := billing.Gateway(dbRow.Gateway)

	d, err := ToDomainDetails(gateway, dbRow.Details)
	if err != nil {
		return nil, fmt.Errorf("failed to parse details: %w", err)
	}

	return billing.New(
		dbRow.Quantity,
		billing.Currency(dbRow.Currency),
		billing.Gateway(dbRow.Gateway),
		d,
		billing.WithTenantID(tenantID),
		billing.WithID(transactionID),
		billing.WithStatus(billing.Status(dbRow.Status)),
		billing.WithCreatedAt(dbRow.CreatedAt),
		billing.WithUpdatedAt(dbRow.UpdatedAt),
	), nil
}

func ToDBTransaction(entity billing.Transaction) (*models.Transaction, error) {
	d, err := ToDbDetails(entity.Details())
	if err != nil {
		return nil, fmt.Errorf("failed to serialize details: %w", err)
	}

	return &models.Transaction{
		ID:        entity.ID().String(),
		TenantID:  entity.TenantID().String(),
		Status:    string(entity.Status()),
		Quantity:  entity.Amount().Quantity(),
		Currency:  string(entity.Amount().Currency()),
		Gateway:   string(entity.Gateway()),
		Details:   d,
		CreatedAt: entity.CreatedAt(),
		UpdatedAt: entity.UpdatedAt(),
	}, nil
}

func ToDomainDetails(gateway billing.Gateway, data json.RawMessage) (details.Details, error) {
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
		return details.NewOctoDetails(
			d.ShopTransactionId,
			details.OctoWithOctoShopId(d.OctoShopID),
			details.OctoWithShopTransactionId(d.ShopTransactionId),
			details.OctoWithOctoPaymentUUID(d.OctoPaymentUUID),
			details.OctoWithInitTime(d.InitTime),
			details.OctoWithAutoCapture(d.AutoCapture),
			details.OctoWithTest(d.Test),
			details.OctoWithStatus(d.Status),
			details.OctoWithDescription(d.Description),
			details.OctoWithCardType(d.CardType),
			details.OctoWithCardCountry(d.CardCountry),
			details.OctoWithCardIsPhysical(d.CardIsPhysical),
			details.OctoWithCardMaskedPan(d.CardMaskedPan),
			details.OctoWithRrn(d.Rrn),
			details.OctoWithRiskLevel(d.RiskLevel),
			details.OctoWithRefundedSum(d.RefundedSum),
			details.OctoWithTransferSum(d.TransferSum),
			details.OctoWithReturnUrl(d.ReturnUrl),
			details.OctoWithNotifyUrl(d.NotifyUrl),
			details.OctoWithOctoPayUrl(d.OctoPayUrl),
			details.OctoWithSignature(d.Signature),
			details.OctoWithHashKey(d.HashKey),
			details.OctoWithPayedTime(d.PayedTime),
			details.OctoWithError(d.Error),
			details.OctoWithErrMessage(d.ErrMessage),
		), nil

	case billing.Stripe:
		var d models.StripeDetails
		if err := json.Unmarshal(data, &d); err != nil {
			return nil, err
		}

		items := make([]details.StripeItem, len(d.Items))
		for i, item := range d.Items {
			itemOpts := make([]details.StripeItemOption, 0)

			if item.AdjustableQuantity != nil {
				itemOpts = append(itemOpts, details.StripeItemWithAdjustableQuantity(
					item.AdjustableQuantity.Enabled,
					item.AdjustableQuantity.Minimum,
					item.AdjustableQuantity.Maximum,
				))
			}
			items[i] = details.NewStripeItem(item.PriceID, item.Quantity, itemOpts...)
		}

		opts := []details.StripeOption{
			details.StripeWithMode(d.Mode),
			details.StripeWithBillingReason(d.BillingReason),
			details.StripeWithSessionID(d.SessionID),
			details.StripeWithInvoiceID(d.InvoiceID),
			details.StripeWithSubscriptionID(d.SubscriptionID),
			details.StripeWithCustomerID(d.CustomerID),
			details.StripeWithItems(items),
			details.StripeWithSuccessURL(d.SuccessURL),
			details.StripeWithCancelURL(d.CancelURL),
			details.StripeWithURL(d.URL),
		}

		if d.SubscriptionData != nil {
			opts = append(opts, details.StripeWithSubscription(
				details.NewStripeSubscriptionData(
					details.StripeSubscriptionDataWithDescription(d.SubscriptionData.Description),
					details.StripeSubscriptionDataWithTrialPeriodDays(d.SubscriptionData.TrialPeriodDays),
				),
			))
		}

		return details.NewStripeDetails(
			d.ClientReferenceID,
			opts...,
		), nil
	default:
		return nil, fmt.Errorf("unsupported gateway: %s", gateway)
	}
}

func ToDbDetails(data details.Details) (json.RawMessage, error) {
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

	case details.OctoDetails:
		return json.Marshal(&models.OctoDetails{
			OctoShopID:        d.OctoShopId(),
			ShopTransactionId: d.ShopTransactionId(),
			OctoPaymentUUID:   d.OctoPaymentUUID(),
			InitTime:          d.InitTime(),
			AutoCapture:       d.AutoCapture(),
			Test:              d.Test(),
			Status:            d.Status(),
			Description:       d.Description(),
			CardType:          d.CardType(),
			CardCountry:       d.CardCountry(),
			CardIsPhysical:    d.CardIsPhysical(),
			CardMaskedPan:     d.CardMaskedPan(),
			Rrn:               d.Rrn(),
			RiskLevel:         d.RiskLevel(),
			RefundedSum:       d.RefundedSum(),
			TransferSum:       d.TransferSum(),
			ReturnUrl:         d.ReturnUrl(),
			NotifyUrl:         d.NotifyUrl(),
			OctoPayUrl:        d.OctoPayUrl(),
			Signature:         d.Signature(),
			HashKey:           d.HashKey(),
			PayedTime:         d.PayedTime(),
			Error:             d.Error(),
			ErrMessage:        d.ErrMessage(),
		})

	case details.StripeDetails:
		items := make([]models.StripeItem, len(d.Items()))
		for i, item := range d.Items() {
			items[i] = models.StripeItem{
				PriceID:  item.PriceID(),
				Quantity: item.Quantity(),
			}
		}

		return json.Marshal(&models.StripeDetails{
			Mode:              d.Mode(),
			BillingReason:     d.BillingReason(),
			SessionID:         d.SessionID(),
			ClientReferenceID: d.ClientReferenceID(),
			InvoiceID:         d.InvoiceID(),
			SubscriptionID:    d.SubscriptionID(),
			CustomerID:        d.CustomerID(),
			Items:             items,
			SuccessURL:        d.SuccessURL(),
			CancelURL:         d.CancelURL(),
			URL:               d.URL(),
		})

	default:
		return nil, fmt.Errorf("unsupported details type: %T", d)
	}
}
