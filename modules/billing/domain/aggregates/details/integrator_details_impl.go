package details

type IntegratorOption func(d *integratorDetails)

func IntegratorWithData(data map[string]any) IntegratorOption {
	return func(d *integratorDetails) {
		d.data = data
	}
}

func IntegratorWithErrorCode(errorCode int32) IntegratorOption {
	return func(d *integratorDetails) {
		d.errorCode = errorCode
	}
}

func IntegratorWithErrorNote(errorNote string) IntegratorOption {
	return func(d *integratorDetails) {
		d.errorNote = errorNote
	}
}

// ---- Implementation ----

func NewIntegratorDetails(opts ...IntegratorOption) IntegratorDetails {
	d := &integratorDetails{
		data:      make(map[string]any),
		errorCode: 0,
		errorNote: "",
	}

	for _, opt := range opts {
		opt(d)
	}

	return d
}

type integratorDetails struct {
	data      map[string]any
	errorCode int32
	errorNote string
}

func (d *integratorDetails) Data() map[string]any {
	return d.data
}

func (d *integratorDetails) ErrorCode() int32 {
	return d.errorCode
}

func (d *integratorDetails) ErrorNote() string {
	return d.errorNote
}

func (d *integratorDetails) Get(key string) any {
	if d.data == nil {
		return nil
	}
	return d.data[key]
}

func (d *integratorDetails) SetData(data map[string]any) IntegratorDetails {
	result := *d
	result.data = data
	return &result
}

func (d *integratorDetails) Set(key string, value any) IntegratorDetails {
	result := *d
	// Deep copy the map to ensure immutability
	result.data = make(map[string]any, len(d.data)+1)
	for k, v := range d.data {
		result.data[k] = v
	}
	result.data[key] = value
	return &result
}

func (d *integratorDetails) SetErrorCode(errorCode int32) IntegratorDetails {
	result := *d
	result.errorCode = errorCode
	return &result
}

func (d *integratorDetails) SetErrorNote(errorNote string) IntegratorDetails {
	result := *d
	result.errorNote = errorNote
	return &result
}
