package details

type ClickOption func(d *clickDetails)

func ClickWithServiceID(serviceID int64) ClickOption {
	return func(d *clickDetails) {
		d.serviceID = serviceID
	}
}

func ClickWithMerchantID(merchantID int64) ClickOption {
	return func(d *clickDetails) {
		d.merchantID = merchantID
	}
}

func ClickWithMerchantUserID(merchantUserID int64) ClickOption {
	return func(d *clickDetails) {
		d.merchantUserID = merchantUserID
	}
}

func ClickWithMerchantPrepareID(merchantPrepareID int64) ClickOption {
	return func(d *clickDetails) {
		d.merchantPrepareID = merchantPrepareID
	}
}

func ClickWithMerchantConfirmID(merchantConfirmID int64) ClickOption {
	return func(d *clickDetails) {
		d.merchantConfirmID = merchantConfirmID
	}
}

func ClickWithPayDocId(payDocId int64) ClickOption {
	return func(d *clickDetails) {
		d.payDocId = payDocId
	}
}

func ClickWithPaymentID(paymentID int64) ClickOption {
	return func(d *clickDetails) {
		d.paymentID = paymentID
	}
}

func ClickWithPaymentStatus(paymentStatus int32) ClickOption {
	return func(d *clickDetails) {
		d.paymentStatus = paymentStatus
	}
}

func ClickWithSignTime(signTime string) ClickOption {
	return func(d *clickDetails) {
		d.signTime = signTime
	}
}

func ClickWithSignString(signString string) ClickOption {
	return func(d *clickDetails) {
		d.signString = signString
	}
}

func ClickWithErrorCode(errorCode int32) ClickOption {
	return func(d *clickDetails) {
		d.errorCode = errorCode
	}
}

func ClickWithErrorNote(errorNote string) ClickOption {
	return func(d *clickDetails) {
		d.errorNote = errorNote
	}
}

func ClickWithLink(link string) ClickOption {
	return func(d *clickDetails) {
		d.link = link
	}
}

func ClickWithParams(params map[string]any) ClickOption {
	return func(d *clickDetails) {
		d.params = params
	}
}

// ---- Implementation ----

func NewClickDetails(
	merchantTransID string,
	opts ...ClickOption,
) ClickDetails {
	d := &clickDetails{
		serviceID:         0,
		merchantID:        0,
		merchantUserID:    0,
		merchantTransID:   merchantTransID,
		merchantPrepareID: 0,
		merchantConfirmID: 0,
		paymentID:         0,
		paymentStatus:     0,
		errorCode:         0,
		errorNote:         "",
		link:              "",
		params:            map[string]any{},
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

type clickDetails struct {
	serviceID         int64
	merchantID        int64
	merchantUserID    int64
	merchantTransID   string
	merchantPrepareID int64
	merchantConfirmID int64
	payDocId          int64
	paymentID         int64
	paymentStatus     int32
	signTime          string
	signString        string
	errorCode         int32
	errorNote         string
	link              string
	params            map[string]any
}

func (d *clickDetails) ServiceID() int64 {
	return d.serviceID
}

func (d *clickDetails) MerchantID() int64 {
	return d.merchantID
}

func (d *clickDetails) MerchantUserID() int64 {
	return d.merchantUserID
}

func (d *clickDetails) MerchantTransID() string {
	return d.merchantTransID
}

func (d *clickDetails) MerchantPrepareID() int64 {
	return d.merchantPrepareID
}

func (d *clickDetails) MerchantConfirmID() int64 {
	return d.merchantConfirmID
}

func (d *clickDetails) PayDocId() int64 {
	return d.payDocId
}

func (d *clickDetails) PaymentID() int64 {
	return d.paymentID
}

func (d *clickDetails) PaymentStatus() int32 {
	return d.paymentStatus
}

func (d *clickDetails) SignTime() string {
	return d.signTime
}

func (d *clickDetails) SignString() string {
	return d.signString
}

func (d *clickDetails) ErrorCode() int32 {
	return d.errorCode
}

func (d *clickDetails) ErrorNote() string {
	return d.errorNote
}

func (d *clickDetails) Link() string {
	return d.link
}

func (d *clickDetails) Params() map[string]any {
	return d.params
}

func (d *clickDetails) SetServiceID(serviceID int64) ClickDetails {
	result := *d
	result.serviceID = serviceID
	return &result
}

func (d *clickDetails) SetMerchantID(merchantID int64) ClickDetails {
	result := *d
	result.merchantID = merchantID
	return &result
}

func (d *clickDetails) SetMerchantUserID(merchantUserID int64) ClickDetails {
	result := *d
	result.merchantUserID = merchantUserID
	return &result
}

func (d *clickDetails) SetMerchantPrepareID(merchantPrepareID int64) ClickDetails {
	result := *d
	result.merchantPrepareID = merchantPrepareID
	return &result
}

func (d *clickDetails) SetMerchantConfirmID(merchantConfirmID int64) ClickDetails {
	result := *d
	result.merchantConfirmID = merchantConfirmID
	return &result
}

func (d *clickDetails) SetPayDocId(payDocId int64) ClickDetails {
	result := *d
	result.payDocId = payDocId
	return &result
}

func (d *clickDetails) SetPaymentID(paymentID int64) ClickDetails {
	result := *d
	result.paymentID = paymentID
	return &result
}

func (d *clickDetails) SetPaymentStatus(paymentStatus int32) ClickDetails {
	result := *d
	result.paymentStatus = paymentStatus
	return &result
}

func (d *clickDetails) SetSignTime(signTime string) ClickDetails {
	result := *d
	result.signTime = signTime
	return &result
}

func (d *clickDetails) SetSignString(signString string) ClickDetails {
	result := *d
	result.signString = signString
	return &result
}

func (d *clickDetails) SetErrorCode(errorCode int32) ClickDetails {
	result := *d
	result.errorCode = errorCode
	return &result
}

func (d *clickDetails) SetErrorNote(errorNote string) ClickDetails {
	result := *d
	result.errorNote = errorNote
	return &result
}

func (d *clickDetails) SetLink(link string) ClickDetails {
	result := *d
	result.link = link
	return &result
}

func (d *clickDetails) SetParams(params map[string]any) ClickDetails {
	result := *d
	result.params = params
	return &result
}
