package details

type CashOption func(d *cashDetails)

func CashWithData(data map[string]any) CashOption {
	return func(d *cashDetails) {
		d.data = data
	}
}

// ---- Implementation ----

func NewCashDetails(opts ...CashOption) CashDetails {
	d := &cashDetails{
		data: make(map[string]any),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

type cashDetails struct {
	data map[string]any
}

func (d *cashDetails) Data() map[string]any {
	return d.data
}

func (d *cashDetails) SetData(data map[string]any) CashDetails {
	result := *d
	result.data = data
	return &result
}

func (d *cashDetails) Get(key string) any {
	if d.data == nil {
		return nil
	}
	return d.data[key]
}

func (d *cashDetails) Set(key string, value any) CashDetails {
	result := *d
	// Deep copy the map to ensure immutability
	result.data = make(map[string]any, len(d.data)+1)
	for k, v := range d.data {
		result.data[k] = v
	}
	result.data[key] = value
	return &result
}
