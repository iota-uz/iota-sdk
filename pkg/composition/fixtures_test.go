package composition

// Shared test fixtures used across multiple *_test.go files in this package.
// Kept here so that test-only types do not bleed into production build tags.

type greetingPort interface {
	Greet() string
}

type greetingService struct {
	value string
}

func (s *greetingService) Greet() string {
	return s.value
}

type testComponent struct {
	descriptor Descriptor
	build      func(*Builder) error
}

func (c testComponent) Descriptor() Descriptor {
	return c.descriptor
}

func (c testComponent) Build(builder *Builder) error {
	if c.build == nil {
		return nil
	}
	return c.build(builder)
}
