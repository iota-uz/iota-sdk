package details

type Details interface {
}

type ClickDetails interface {
	Details

	ServiceID() int64

	MerchantID() int64
	MerchantUserID() int64
	MerchantTransID() string
	MerchantPrepareID() int64
	MerchantConfirmID() int64

	PayDocId() int64
	PaymentID() int64
	PaymentStatus() int32

	SignTime() string
	SignString() string

	ErrorCode() int32
	ErrorNote() string

	Link() string
	Params() map[string]any

	SetServiceID(serviceID int64) ClickDetails
	SetMerchantID(merchantID int64) ClickDetails
	SetMerchantUserID(merchantUserID int64) ClickDetails
	SetMerchantPrepareID(merchantPrepareID int64) ClickDetails
	SetMerchantConfirmID(merchantConfirmID int64) ClickDetails

	SetPayDocId(payDocId int64) ClickDetails
	SetPaymentID(paymentID int64) ClickDetails
	SetPaymentStatus(paymentStatus int32) ClickDetails

	SetSignTime(signTime string) ClickDetails
	SetSignString(signString string) ClickDetails

	SetErrorCode(errorCode int32) ClickDetails
	SetErrorNote(errorNote string) ClickDetails

	SetLink(link string) ClickDetails
	SetParams(params map[string]any) ClickDetails
}

type PaymeReceiver interface {
	ID() string
	Amount() float64
}

type PaymeDetails interface {
	Details

	MerchantID() string

	ID() string

	Transaction() string

	State() int32

	Time() int64
	CreatedTime() int64
	PerformTime() int64
	CancelTime() int64

	Account() map[string]any

	Receivers() []PaymeReceiver

	Additional() map[string]any

	Reason() int32

	ErrorCode() int32

	Link() string

	Params() map[string]any

	SetMerchantID(merchantID string) PaymeDetails
	SetID(id string) PaymeDetails
	SetTransaction(transaction string) PaymeDetails
	SetState(state int32) PaymeDetails
	SetTime(time int64) PaymeDetails
	SetCreatedTime(createdTime int64) PaymeDetails
	SetPerformTime(performTime int64) PaymeDetails
	SetCancelTime(cancelTime int64) PaymeDetails
	SetAccount(account map[string]any) PaymeDetails
	SetReceivers(receivers []PaymeReceiver) PaymeDetails
	SetAdditional(additional map[string]any) PaymeDetails
	SetReason(reason int32) PaymeDetails
	SetErrorCode(errorCode int32) PaymeDetails
	SetLink(link string) PaymeDetails

	SetParams(params map[string]any) PaymeDetails
}

type OctoDetails interface {
	Details

	OctoShopId() int32
	ShopTransactionId() string
	OctoPaymentUUID() string

	InitTime() string
	AutoCapture() bool
	Test() bool

	Status() string

	Description() string

	CardType() string
	CardCountry() string
	CardIsPhysical() bool
	CardMaskedPan() string

	Rrn() string
	RiskLevel() int32

	RefundedSum() float64
	TransferSum() float64

	ReturnUrl() string
	NotifyUrl() string
	OctoPayUrl() string

	Signature() string
	HashKey() string

	PayedTime() string

	Error() int32
	ErrMessage() string

	SetOctoShopId(octoShopId int32) OctoDetails
	SetShopTransactionId(shopTransactionId string) OctoDetails
	SetOctoPaymentUUID(octoPaymentUUID string) OctoDetails

	SetInitTime(initTime string) OctoDetails
	SetAutoCapture(autoCapture bool) OctoDetails
	SetTest(test bool) OctoDetails

	SetStatus(status string) OctoDetails

	SetDescription(description string) OctoDetails

	SetCardType(cardType string) OctoDetails
	SetCardCountry(cardCountry string) OctoDetails
	SetCardIsPhysical(cardIsPhysical bool) OctoDetails
	SetCardMaskedPan(cardMaskedPan string) OctoDetails

	SetRrn(rrn string) OctoDetails
	SetRiskLevel(riskLevel int32) OctoDetails

	SetRefundedSum(refundedSum float64) OctoDetails
	SetTransferSum(transferSum float64) OctoDetails

	SetReturnUrl(returnUrl string) OctoDetails
	SetNotifyUrl(notifyUrl string) OctoDetails
	SetOctoPayUrl(octoPayUrl string) OctoDetails

	SetSignature(signature string) OctoDetails
	SetHashKey(hashKey string) OctoDetails

	SetPayedTime(payedTime string) OctoDetails

	SetError(errCode int32) OctoDetails
	SetErrMessage(errMessage string) OctoDetails
}

type StripeItem interface {
	PriceID() string
	Quantity() int64
	AdjustableQuantity() StripeItemAdjustableQuantity
}

type StripeItemAdjustableQuantity interface {
	Enabled() bool
	Maximum() int64
	Minimum() int64
}

type StripeSubscriptionData interface {
	Description() string
	TrialPeriodDays() int64
}

type StripeDetails interface {
	Details

	Mode() string
	BillingReason() string

	SessionID() string
	ClientReferenceID() string

	InvoiceID() string
	SubscriptionID() string
	CustomerID() string

	Items() []StripeItem

	SubscriptionData() StripeSubscriptionData

	SuccessURL() string
	CancelURL() string
	URL() string

	SetMode(mode string) StripeDetails
	SetBillingReason(billingReason string) StripeDetails

	SetSessionID(sessionID string) StripeDetails
	SetClientReferenceID(clientReferenceID string) StripeDetails

	SetInvoiceID(invoiceID string) StripeDetails
	SetSubscriptionID(subscriptionID string) StripeDetails
	SetCustomerID(customerID string) StripeDetails

	SetItems(items []StripeItem) StripeDetails

	SetSuccessURL(successURL string) StripeDetails
	SetCancelURL(cancelURL string) StripeDetails
	SetURL(url string) StripeDetails
}
