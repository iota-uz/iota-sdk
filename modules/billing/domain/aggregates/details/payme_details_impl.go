package details

type PaymeOption func(d *paymeDetails)

func PaymeWithMerchantID(merchantID string) PaymeOption {
	return func(d *paymeDetails) {
		d.merchantID = merchantID
	}
}

func PaymeWithID(id string) PaymeOption {
	return func(d *paymeDetails) {
		d.id = id
	}
}

func PaymeWithTransaction(transaction string) PaymeOption {
	return func(d *paymeDetails) {
		d.transaction = transaction
	}
}

func PaymeWithState(state int32) PaymeOption {
	return func(d *paymeDetails) {
		d.state = state
	}
}

func PaymeWithTime(time int64) PaymeOption {
	return func(d *paymeDetails) {
		d.time = time
	}
}

func PaymeWithCreatedTime(createdTime int64) PaymeOption {
	return func(d *paymeDetails) {
		d.createdTime = createdTime
	}
}

func PaymeWithPerformTime(performTime int64) PaymeOption {
	return func(d *paymeDetails) {
		d.performTime = performTime
	}
}

func PaymeWithCancelTime(cancelTime int64) PaymeOption {
	return func(d *paymeDetails) {
		d.cancelTime = cancelTime
	}
}

func PaymeWithAccount(account map[string]any) PaymeOption {
	return func(d *paymeDetails) {
		d.account = account
	}
}

func PaymeWithReceivers(receivers []PaymeReceiver) PaymeOption {
	return func(d *paymeDetails) {
		d.receivers = receivers
	}
}

func PaymeWithAdditional(additional map[string]any) PaymeOption {
	return func(d *paymeDetails) {
		d.additional = additional
	}
}

func PaymeWithReason(reason int32) PaymeOption {
	return func(d *paymeDetails) {
		d.reason = reason
	}
}

func PaymeWithErrorCode(errorCode int32) PaymeOption {
	return func(d *paymeDetails) {
		d.errorCode = errorCode
	}
}

func PaymeWithLink(link string) PaymeOption {
	return func(d *paymeDetails) {
		d.link = link
	}
}

func PaymeWithParams(params map[string]any) PaymeOption {
	return func(d *paymeDetails) {
		d.params = params
	}
}

func NewPaymeDetails(
	transaction string,
	options ...PaymeOption,
) PaymeDetails {
	d := &paymeDetails{
		merchantID:  "",
		id:          "",
		transaction: transaction,
		state:       0,
		time:        0,
		createdTime: 0,
		performTime: 0,
		cancelTime:  0,
		account:     map[string]any{},
		receivers:   []PaymeReceiver{},
		additional:  map[string]any{},
		reason:      0,
		errorCode:   0,
		link:        "",
		params:      map[string]any{},
	}

	for _, opt := range options {
		opt(d)
	}

	return d
}

func NewPaymeReceiver(
	id string,
	amount float64,
) PaymeReceiver {
	r := &paymeReceiver{
		id:     id,
		amount: amount,
	}

	return r
}

type paymeReceiver struct {
	id     string
	amount float64
}

func (r *paymeReceiver) ID() string {
	return r.id
}

func (r *paymeReceiver) Amount() float64 {
	return r.amount
}

type paymeDetails struct {
	merchantID  string
	id          string
	transaction string
	state       int32
	time        int64
	createdTime int64
	performTime int64
	cancelTime  int64
	account     map[string]any
	receivers   []PaymeReceiver
	additional  map[string]any
	reason      int32
	errorCode   int32
	link        string
	params      map[string]any
}

func (d *paymeDetails) MerchantID() string {
	return d.merchantID
}

func (d *paymeDetails) ID() string {
	return d.id
}

func (d *paymeDetails) Transaction() string {
	return d.transaction
}

func (d *paymeDetails) State() int32 {
	return d.state
}

func (d *paymeDetails) Time() int64 {
	return d.time
}

func (d *paymeDetails) CreatedTime() int64 {
	return d.createdTime
}

func (d *paymeDetails) PerformTime() int64 {
	return d.performTime
}

func (d *paymeDetails) CancelTime() int64 {
	return d.cancelTime
}

func (d *paymeDetails) Account() map[string]any {
	return d.account
}

func (d *paymeDetails) Receivers() []PaymeReceiver {
	return d.receivers
}

func (d *paymeDetails) Additional() map[string]any {
	return d.additional
}

func (d *paymeDetails) Reason() int32 {
	return d.reason
}

func (d *paymeDetails) ErrorCode() int32 {
	return d.errorCode
}

func (d *paymeDetails) Link() string {
	return d.link
}

func (d *paymeDetails) Params() map[string]any {
	return d.params
}

func (d *paymeDetails) SetMerchantID(merchantID string) PaymeDetails {
	result := *d
	result.merchantID = merchantID
	return &result
}

func (d *paymeDetails) SetID(id string) PaymeDetails {
	result := *d
	result.id = id
	return &result
}

func (d *paymeDetails) SetTransaction(transaction string) PaymeDetails {
	result := *d
	result.transaction = transaction
	return &result
}

func (d *paymeDetails) SetState(state int32) PaymeDetails {
	result := *d
	result.state = state
	return &result
}

func (d *paymeDetails) SetTime(time int64) PaymeDetails {
	result := *d
	result.time = time
	return &result
}

func (d *paymeDetails) SetCreatedTime(createdTime int64) PaymeDetails {
	result := *d
	result.createdTime = createdTime
	return &result
}

func (d *paymeDetails) SetPerformTime(performTime int64) PaymeDetails {
	result := *d
	result.performTime = performTime
	return &result
}

func (d *paymeDetails) SetCancelTime(cancelTime int64) PaymeDetails {
	result := *d
	result.cancelTime = cancelTime
	return &result
}

func (d *paymeDetails) SetAccount(account map[string]any) PaymeDetails {
	result := *d
	result.account = account
	return &result
}

func (d *paymeDetails) SetReceivers(receivers []PaymeReceiver) PaymeDetails {
	result := *d
	result.receivers = receivers
	return &result
}

func (d *paymeDetails) SetAdditional(additional map[string]any) PaymeDetails {
	result := *d
	result.additional = additional
	return &result
}

func (d *paymeDetails) SetErrorCode(errorCode int32) PaymeDetails {
	result := *d
	result.errorCode = errorCode
	return &result
}

func (d *paymeDetails) SetReason(reason int32) PaymeDetails {
	result := *d
	result.reason = reason
	return &result
}

func (d *paymeDetails) SetLink(link string) PaymeDetails {
	result := *d
	result.link = link
	return &result
}

func (d *paymeDetails) SetParams(params map[string]any) PaymeDetails {
	result := *d
	result.params = params
	return &result
}
