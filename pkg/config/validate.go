package config

// Validatable is an optional interface for config structs.
// When Register[T] is called and T implements Validatable,
// Validate is invoked after Unmarshal. A non-nil error aborts registration.
type Validatable interface {
	Validate() error
}
