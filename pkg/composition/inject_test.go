package composition

import (
	"errors"
	"strings"
	"testing"

	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/pkg/application"
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

func compileWithBuild(t *testing.T, build func(*Builder) error) error {
	t.Helper()
	engine := NewEngine()
	require.NoError(t, engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build:      build,
	}))
	_, err := engine.Compile(BuildContext{})
	return err
}

func TestProvideFunc_PanicsOnNilConstructor(t *testing.T) {
	require.PanicsWithValue(t,
		"composition: ProvideFunc: constructor is nil",
		func() {
			_ = compileWithBuild(t, func(builder *Builder) error {
				ProvideFunc(builder, nil)
				return nil
			})
		},
		"ProvideFunc must panic on nil constructor",
	)
}

func TestProvideFunc_PanicsOnNonFunction(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			ProvideFunc(builder, 42) // int, not a func
			return nil
		})
	})
}

func TestProvideFunc_PanicsOnWrongReturnCount(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			// zero return values — rejected
			ProvideFunc(builder, func() {})
			return nil
		})
	})
}

func TestProvideFunc_PanicsOnVariadic(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			// variadic constructor — rejected at registration
			ProvideFunc(builder, func(a *repoA, opts ...string) *serviceX {
				return &serviceX{a: a}
			})
			return nil
		})
	})
}

func TestProvideFuncAs_HappyPath(t *testing.T) {
	// Count constructor invocations to verify the interface bridge reuses
	// the concrete singleton instead of running the constructor twice.
	var callCount int
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "greet"},
		build: func(builder *Builder) error {
			ProvideFuncAs[greetingPort](builder, func() *greetingService {
				callCount++
				return &greetingService{value: "once"}
			})
			// Downstream constructor depends on the interface key — must
			// receive the same value the concrete key resolves to.
			ProvideFunc(builder, func(port greetingPort) *greetingConsumer {
				return &greetingConsumer{port: port}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	concrete, err := Resolve[*greetingService](container)
	require.NoError(t, err)
	require.Equal(t, "once", concrete.Greet())

	port, err := Resolve[greetingPort](container)
	require.NoError(t, err)
	require.Equal(t, "once", port.Greet())

	// Concrete + interface must resolve to the exact same instance so that
	// mutations or identity checks remain consistent across consumers.
	require.Same(t, concrete, port.(*greetingService))

	consumer, err := Resolve[*greetingConsumer](container)
	require.NoError(t, err)
	require.Same(t, concrete, consumer.port.(*greetingService))

	// Factory must run exactly once despite being reachable via three keys.
	require.Equal(t, 1, callCount)
}

// Regression test for PR #726: when the constructor returns a concrete type
// and a component keys it by an interface the concrete satisfies, a sibling
// ProvideFunc must be able to resolve that interface as a parameter without
// the reflection injector panicking or double-constructing.
func TestProvideFuncAs_InterfaceConsumerResolves(t *testing.T) {
	err := compileWithBuild(t, func(builder *Builder) error {
		ProvideFuncAs[greetingPort](builder, func() *greetingService {
			return &greetingService{value: "consumer"}
		})
		ProvideFunc(builder, func(p greetingPort) *repoA {
			return &repoA{value: p.Greet()}
		})
		return nil
	})
	require.NoError(t, err)
}

func TestProvideFuncAs_PanicsWhenInterfaceEqualsReturn(t *testing.T) {
	require.PanicsWithValue(t,
		"composition: ProvideFuncAs[composition.greetingPort]: constructor already returns composition.greetingPort; use ProvideFunc instead",
		func() {
			_ = compileWithBuild(t, func(builder *Builder) error {
				// Constructor already returns the interface — no bridge needed
				ProvideFuncAs[greetingPort](builder, func() greetingPort {
					return &greetingService{value: "same"}
				})
				return nil
			})
		},
		"ProvideFuncAs[I] must reject constructors whose return type == I",
	)
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

func TestContributeControllersFunc_SingleController(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			Provide[*repoA](builder, &repoA{value: "ctrl"})
			ContributeControllersFunc(builder, func(a *repoA) application.Controller {
				return &stubController{key: a.value}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)

	ctrls := container.Controllers()
	require.Len(t, ctrls, 1)
	require.Equal(t, "ctrl", ctrls[0].Key())
}

func TestContributeControllersFunc_SliceOfControllers(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			Provide[*repoA](builder, &repoA{value: "a"})
			Provide[*repoB](builder, &repoB{value: 1})
			ContributeControllersFunc(builder, func(a *repoA, b *repoB) []application.Controller {
				return []application.Controller{
					&stubController{key: a.value},
					&stubController{key: "after-" + a.value},
				}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)
	require.Len(t, container.Controllers(), 2)
}

func TestContributeControllersFunc_DropsNil(t *testing.T) {
	engine := NewEngine()
	err := engine.Register(testComponent{
		descriptor: Descriptor{Name: "x"},
		build: func(builder *Builder) error {
			// Constructor intentionally returns nil to signal "disabled".
			ContributeControllersFunc(builder, func() application.Controller { return nil })
			// A sibling constructor returns a real controller.
			ContributeControllersFunc(builder, func() application.Controller {
				return &stubController{key: "real"}
			})
			return nil
		},
	})
	require.NoError(t, err)

	container, err := engine.Compile(BuildContext{})
	require.NoError(t, err)
	ctrls := container.Controllers()
	require.Len(t, ctrls, 1)
	require.Equal(t, "real", ctrls[0].Key())
}

func TestContributeControllersFunc_PanicsOnBadReturnType(t *testing.T) {
	require.Panics(t, func() {
		_ = compileWithBuild(t, func(builder *Builder) error {
			// Return type must be application.Controller or []application.Controller.
			ContributeControllersFunc(builder, func() string { return "oops" })
			return nil
		})
	})
}

// greetingConsumer depends on the greetingPort interface and is used to
// exercise ProvideFuncAs interface-bridge consumption.
type greetingConsumer struct {
	port greetingPort
}

// stubController is a minimal application.Controller for tests.
type stubController struct{ key string }

func (s *stubController) Key() string { return s.key }

func (s *stubController) Register(_ *mux.Router) {}
