package validators

type ValidationError struct {
	Fields map[string]string
}

func (v *ValidationError) Error() string {
	return "validators failed"
}

func NewValidationError(fields map[string]string) *ValidationError {
	return &ValidationError{Fields: fields}
}

func (v *ValidationError) HasField(field string) bool {
	_, exists := v.Fields[field]
	return exists
}

func (v *ValidationError) Field(field string) string {
	return v.Fields[field]
}

func (v *ValidationError) FieldsMap() map[string]string {
	return v.Fields
}
