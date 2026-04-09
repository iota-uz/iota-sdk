package composition

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

type repoA struct{ value string }
type repoB struct{ value int }

type serviceX struct {
	a *repoA
	b *repoB
}

func newServiceX(a *repoA, b *repoB) *serviceX {
	return &serviceX{a: a, b: b}
}

func newServiceXErr(a *repoA, b *repoB) (*serviceX, error) {
	if b.value < 0 {
		return nil, errors.New("negative")
	}
	return &serviceX{a: a, b: b}, nil
}

func TestProvideFunc_NoError(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			Provide[*repoA](builder, &repoA{value: "hello"})
			Provide[*repoB](builder, &repoB{value: 42})
			ProvideFunc(builder, newServiceX)
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	svc, err := Resolve[*serviceX](container)
	require.NoError(t, err)
	require.Equal(t, "hello", svc.a.value)
	require.Equal(t, 42, svc.b.value)
}

func TestProvideFunc_WithError(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			Provide[*repoA](builder, &repoA{value: "ok"})
			Provide[*repoB](builder, &repoB{value: 5})
			ProvideFunc(builder, newServiceXErr)
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	svc, err := Resolve[*serviceX](container)
	require.NoError(t, err)
	require.Equal(t, "ok", svc.a.value)
}

func TestProvideFunc_MissingDependency(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			Provide[*repoA](builder, &repoA{value: "ok"})
			// repoB intentionally not provided
			ProvideFunc(builder, newServiceX)
			return nil
		},
	})
	require.NoError(t, err)

	_, err = engine.Compile(BuildContext{})
	require.Error(t, err)
	require.Contains(t, strings.ToUpper(err.Error()), "NOT PROVIDED")
}

func TestProvideAs_BothKeys(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "greet"},
		build: func(builder *Builder) error {
			ProvideAs[greetingPort, *greetingService](builder, &greetingService{value: "via-as"})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	svc, err := Resolve[*greetingService](container)
	require.NoError(t, err)
	require.Equal(t, "via-as", svc.Greet())

	port, err := Resolve[greetingPort](container)
	require.NoError(t, err)
	require.Equal(t, "via-as", port.Greet())

	// Both keys must point at the same instance.
	require.Same(t, svc, port)
}
