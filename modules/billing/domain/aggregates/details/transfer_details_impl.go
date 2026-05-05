// Package details provides this package.
package details

type TransferOption func(d *transferDetails)

func TransferWithData(data map[string]any) TransferOption {
	return func(d *transferDetails) {
		d.data = data
	}
}

func TransferWithComment(comment string) TransferOption {
	return func(d *transferDetails) {
		d.comment = comment
	}
}

// ---- Implementation ----

func NewTransferDetails(opts ...TransferOption) TransferDetails {
	d := &transferDetails{
		data: make(map[string]any),
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

type transferDetails struct {
	data    map[string]any
	comment string
}

func (d *transferDetails) Data() map[string]any {
	return d.data
}

func (d *transferDetails) SetData(data map[string]any) TransferDetails {
	result := *d
	result.data = data
	return &result
}

func (d *transferDetails) Get(key string) any {
	if d.data == nil {
		return nil
	}
	return d.data[key]
}

func (d *transferDetails) Set(key string, value any) TransferDetails {
	result := *d
	result.data = make(map[string]any, len(d.data)+1)
	for k, v := range d.data {
		result.data[k] = v
	}
	result.data[key] = value
	return &result
}

func (d *transferDetails) Comment() string {
	return d.comment
}

func (d *transferDetails) SetComment(comment string) TransferDetails {
	result := *d
	result.comment = comment
	return &result
}
