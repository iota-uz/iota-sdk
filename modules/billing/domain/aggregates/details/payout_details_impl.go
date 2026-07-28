package details

type PayoutOption func(d *payoutDetails)

func PayoutWithData(data map[string]any) PayoutOption {
	return func(d *payoutDetails) {
		d.data = data
	}
}

func NewPayoutDetails(opts ...PayoutOption) PayoutDetails {
	d := &payoutDetails{data: make(map[string]any)}
	for _, opt := range opts {
		opt(d)
	}
	return d
}

type payoutDetails struct {
	data map[string]any
}

func (d *payoutDetails) Data() map[string]any {
	return d.data
}

func (d *payoutDetails) SetData(data map[string]any) PayoutDetails {
	result := *d
	result.data = data
	return &result
}

func (d *payoutDetails) Get(key string) any {
	if d.data == nil {
		return nil
	}
	return d.data[key]
}

func (d *payoutDetails) Set(key string, value any) PayoutDetails {
	result := *d
	result.data = make(map[string]any, len(d.data)+1)
	for k, v := range d.data {
		result.data[k] = v
	}
	result.data[key] = value
	return &result
}
