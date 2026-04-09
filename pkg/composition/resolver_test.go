package composition

import (
	"testing"

	"github.com/stretchr/testify/require"
)

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

func TestResolverTypedResolveAndProvide(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "greeting"},
		build: func(builder *Builder) error {
			serviceResolver := Use[*greetingService]()
			Provide[*greetingService](builder, func() *greetingService {
				return &greetingService{value: "hello"}
			})
			Provide[greetingPort](builder, func(container *Container) (greetingPort, error) {
				return serviceResolver.Resolve(container)
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	service, err := Resolve[*greetingService](container)
	require.NoError(t, err)
	require.Equal(t, "hello", service.Greet())

	port, err := Resolve[greetingPort](container)
	require.NoError(t, err)
	require.Equal(t, "hello", port.Greet())
}
