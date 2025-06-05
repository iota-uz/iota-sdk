package details

type OctoOption func(d *octoDetails)

func OctoWithOctoShopId(octoShopId int32) OctoOption {
	return func(d *octoDetails) {
		d.octoShopId = octoShopId
	}
}

func OctoWithShopTransactionId(shopTransactionId string) OctoOption {
	return func(d *octoDetails) {
		d.shopTransactionId = shopTransactionId
	}
}

func OctoWithOctoPaymentUUID(uuid string) OctoOption {
	return func(d *octoDetails) {
		d.octoPaymentUUID = uuid
	}
}

func OctoWithInitTime(initTime string) OctoOption {
	return func(d *octoDetails) {
		d.initTime = initTime
	}
}

func OctoWithAutoCapture(autoCapture bool) OctoOption {
	return func(d *octoDetails) {
		d.autoCapture = autoCapture
	}
}

func OctoWithTest(test bool) OctoOption {
	return func(d *octoDetails) {
		d.test = test
	}
}

func OctoWithStatus(status string) OctoOption {
	return func(d *octoDetails) {
		d.status = status
	}
}

func OctoWithDescription(description string) OctoOption {
	return func(d *octoDetails) {
		d.description = description
	}
}

func OctoWithCardType(cardType string) OctoOption {
	return func(d *octoDetails) {
		d.cardType = cardType
	}
}

func OctoWithCardCountry(cardCountry string) OctoOption {
	return func(d *octoDetails) {
		d.cardCountry = cardCountry
	}
}

func OctoWithCardIsPhysical(cardIsPhysical bool) OctoOption {
	return func(d *octoDetails) {
		d.cardIsPhysical = cardIsPhysical
	}
}

func OctoWithCardMaskedPan(cardMaskedPan string) OctoOption {
	return func(d *octoDetails) {
		d.cardMaskedPan = cardMaskedPan
	}
}

func OctoWithRrn(rrn string) OctoOption {
	return func(d *octoDetails) {
		d.rrn = rrn
	}
}

func OctoWithRiskLevel(riskLevel int32) OctoOption {
	return func(d *octoDetails) {
		d.riskLevel = riskLevel
	}
}

func OctoWithRefundedSum(sum float64) OctoOption {
	return func(d *octoDetails) {
		d.refundedSum = sum
	}
}

func OctoWithTransferSum(sum float64) OctoOption {
	return func(d *octoDetails) {
		d.transferSum = sum
	}
}

func OctoWithReturnUrl(url string) OctoOption {
	return func(d *octoDetails) {
		d.returnUrl = url
	}
}

func OctoWithNotifyUrl(url string) OctoOption {
	return func(d *octoDetails) {
		d.notifyUrl = url
	}
}

func OctoWithOctoPayUrl(url string) OctoOption {
	return func(d *octoDetails) {
		d.octoPayUrl = url
	}
}

func OctoWithSignature(signature string) OctoOption {
	return func(d *octoDetails) {
		d.signature = signature
	}
}

func OctoWithHashKey(hashKey string) OctoOption {
	return func(d *octoDetails) {
		d.hashKey = hashKey
	}
}

func OctoWithPayedTime(time string) OctoOption {
	return func(d *octoDetails) {
		d.payedTime = time
	}
}

func OctoWithError(code int32) OctoOption {
	return func(d *octoDetails) {
		d.error = code
	}
}

func OctoWithErrMessage(msg string) OctoOption {
	return func(d *octoDetails) {
		d.errMessage = msg
	}
}

func NewOctoDetails(
	shopTransactionId string,
	opts ...OctoOption,
) OctoDetails {
	d := &octoDetails{
		octoShopId:        0,
		shopTransactionId: shopTransactionId,
		octoPaymentUUID:   "",
		initTime:          "",
		autoCapture:       true,
		test:              false,
		status:            "",
		description:       "",
		cardType:          "",
		cardCountry:       "",
		cardIsPhysical:    false,
		cardMaskedPan:     "",
		rrn:               "",
		riskLevel:         0,
		refundedSum:       0.0,
		transferSum:       0.0,
		returnUrl:         "",
		notifyUrl:         "",
		octoPayUrl:        "",
		signature:         "",
		hashKey:           "",
		payedTime:         "",
		error:             0,
		errMessage:        "",
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

type octoDetails struct {
	octoShopId        int32
	shopTransactionId string
	octoPaymentUUID   string
	initTime          string
	autoCapture       bool
	test              bool
	status            string
	description       string
	cardType          string
	cardCountry       string
	cardIsPhysical    bool
	cardMaskedPan     string
	rrn               string
	riskLevel         int32
	refundedSum       float64
	transferSum       float64
	returnUrl         string
	notifyUrl         string
	octoPayUrl        string
	signature         string
	hashKey           string
	payedTime         string
	error             int32
	errMessage        string
}

func (d *octoDetails) OctoShopId() int32 {
	return d.octoShopId
}

func (d *octoDetails) ShopTransactionId() string {
	return d.shopTransactionId
}

func (d *octoDetails) OctoPaymentUUID() string {
	return d.octoPaymentUUID
}

func (d *octoDetails) InitTime() string {
	return d.initTime
}

func (d *octoDetails) AutoCapture() bool {
	return d.autoCapture
}

func (d *octoDetails) Test() bool {
	return d.test
}

func (d *octoDetails) Status() string {
	return d.status
}

func (d *octoDetails) Description() string {
	return d.description
}

func (d *octoDetails) CardType() string {
	return d.cardType
}

func (d *octoDetails) CardCountry() string {
	return d.cardCountry
}

func (d *octoDetails) CardIsPhysical() bool {
	return d.cardIsPhysical
}

func (d *octoDetails) CardMaskedPan() string {
	return d.cardMaskedPan
}

func (d *octoDetails) Rrn() string {
	return d.rrn
}

func (d *octoDetails) RiskLevel() int32 {
	return d.riskLevel
}

func (d *octoDetails) RefundedSum() float64 {
	return d.refundedSum
}

func (d *octoDetails) TransferSum() float64 {
	return d.transferSum
}

func (d *octoDetails) ReturnUrl() string {
	return d.returnUrl
}

func (d *octoDetails) NotifyUrl() string {
	return d.notifyUrl
}

func (d *octoDetails) OctoPayUrl() string {
	return d.octoPayUrl
}

func (d *octoDetails) Signature() string {
	return d.signature
}

func (d *octoDetails) HashKey() string {
	return d.hashKey
}

func (d *octoDetails) PayedTime() string {
	return d.payedTime
}

func (d *octoDetails) Error() int32 {
	return d.error
}

func (d *octoDetails) ErrMessage() string {
	return d.errMessage
}

func (d *octoDetails) SetOctoShopId(octoShopId int32) OctoDetails {
	result := *d
	result.octoShopId = octoShopId
	return &result
}

func (d *octoDetails) SetShopTransactionId(shopTransactionId string) OctoDetails {
	result := *d
	result.shopTransactionId = shopTransactionId
	return &result
}

func (d *octoDetails) SetOctoPaymentUUID(octoPaymentUUID string) OctoDetails {
	result := *d
	result.octoPaymentUUID = octoPaymentUUID
	return &result
}

func (d *octoDetails) SetInitTime(initTime string) OctoDetails {
	result := *d
	result.initTime = initTime
	return &result
}

func (d *octoDetails) SetAutoCapture(autoCapture bool) OctoDetails {
	result := *d
	result.autoCapture = autoCapture
	return &result
}

func (d *octoDetails) SetTest(test bool) OctoDetails {
	result := *d
	result.test = test
	return &result
}

func (d *octoDetails) SetStatus(status string) OctoDetails {
	result := *d
	result.status = status
	return &result
}

func (d *octoDetails) SetDescription(description string) OctoDetails {
	result := *d
	result.description = description
	return &result
}

func (d *octoDetails) SetCardType(cardType string) OctoDetails {
	result := *d
	result.cardType = cardType
	return &result
}

func (d *octoDetails) SetCardCountry(cardCountry string) OctoDetails {
	result := *d
	result.cardCountry = cardCountry
	return &result
}

func (d *octoDetails) SetCardIsPhysical(cardIsPhysical bool) OctoDetails {
	result := *d
	result.cardIsPhysical = cardIsPhysical
	return &result
}

func (d *octoDetails) SetCardMaskedPan(cardMaskedPan string) OctoDetails {
	result := *d
	result.cardMaskedPan = cardMaskedPan
	return &result
}

func (d *octoDetails) SetRrn(rrn string) OctoDetails {
	result := *d
	result.rrn = rrn
	return &result
}

func (d *octoDetails) SetRiskLevel(riskLevel int32) OctoDetails {
	result := *d
	result.riskLevel = riskLevel
	return &result
}

func (d *octoDetails) SetRefundedSum(refundedSum float64) OctoDetails {
	result := *d
	result.refundedSum = refundedSum
	return &result
}

func (d *octoDetails) SetTransferSum(transferSum float64) OctoDetails {
	result := *d
	result.transferSum = transferSum
	return &result
}

func (d *octoDetails) SetReturnUrl(returnUrl string) OctoDetails {
	result := *d
	result.returnUrl = returnUrl
	return &result
}

func (d *octoDetails) SetNotifyUrl(notifyUrl string) OctoDetails {
	result := *d
	result.notifyUrl = notifyUrl
	return &result
}

func (d *octoDetails) SetOctoPayUrl(octoPayUrl string) OctoDetails {
	result := *d
	result.octoPayUrl = octoPayUrl
	return &result
}

func (d *octoDetails) SetSignature(signature string) OctoDetails {
	result := *d
	result.signature = signature
	return &result
}

func (d *octoDetails) SetHashKey(hashKey string) OctoDetails {
	result := *d
	result.hashKey = hashKey
	return &result
}

func (d *octoDetails) SetPayedTime(payedTime string) OctoDetails {
	result := *d
	result.payedTime = payedTime
	return &result
}

func (d *octoDetails) SetError(errCode int32) OctoDetails {
	result := *d
	result.error = errCode
	return &result
}

func (d *octoDetails) SetErrMessage(errMessage string) OctoDetails {
	result := *d
	result.errMessage = errMessage
	return &result
}
