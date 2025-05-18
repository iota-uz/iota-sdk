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

type PaymeDetails interface {
	Details
}

type OctoDetails interface {
	Details
}

type StripeDetails interface {
	Details
}
