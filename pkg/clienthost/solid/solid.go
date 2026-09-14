// Package solid connects a Go-owned client route to a colocated Solid TSX
// module. Import is declarative: the SDK generator discovers it statically;
// calling it at package initialization performs no I/O or frontend build.
package solid

import (
	"crypto/sha256"
	"fmt"
	"path"
	"runtime"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/clienthost"
)

type MethodKind string

const (
	MethodQuery  MethodKind = "query"
	MethodAction MethodKind = "action"
)

type Method struct {
	Name        string
	Kind        MethodKind
	Cacheable   bool
	MaxRetries  int
	Invalidates []string
}

type MethodOption func(*Method)

func Cacheable(maxRetries int) MethodOption {
	return func(method *Method) {
		method.Cacheable = true
		method.MaxRetries = maxRetries
	}
}

func Invalidates(names ...string) MethodOption {
	return func(method *Method) {
		method.Invalidates = append([]string(nil), names...)
	}
}

func Query[Request, Response any](name string, options ...MethodOption) Method {
	return newMethod(MethodQuery, name, options)
}

func Action[Request, Response any](name string, options ...MethodOption) Method {
	return newMethod(MethodAction, name, options)
}

func newMethod(kind MethodKind, name string, options []MethodOption) Method {
	method := Method{Name: strings.TrimSpace(name), Kind: kind}
	for _, option := range options {
		if option != nil {
			option(&method)
		}
	}
	return method
}

type Option func(*descriptor)

type descriptor struct {
	methods []Method
}

func WithRPC(methods ...Method) Option {
	return func(descriptor *descriptor) {
		descriptor.methods = append(descriptor.methods, methods...)
	}
}

type Component[Props any] struct {
	featureID  string
	sourcePath string
	methods    []Method
}

// Import declares a TSX module relative to the calling Go source file.
//
//go:noinline
func Import[Props any](source string, options ...Option) Component[Props] {
	if err := validateSource(source); err != nil {
		panic(err)
	}
	programCounter, _, _, ok := runtime.Caller(1)
	if !ok {
		panic("solid.Import: caller is unavailable")
	}
	function := runtime.FuncForPC(programCounter)
	if function == nil {
		panic("solid.Import: caller package is unavailable")
	}
	packagePath := callerPackage(function.Name())
	descriptor := descriptor{}
	for _, option := range options {
		if option != nil {
			option(&descriptor)
		}
	}
	if err := validateMethods(descriptor.methods); err != nil {
		panic("solid.Import: " + err.Error())
	}
	return Component[Props]{
		featureID:  FeatureID(packagePath, source),
		sourcePath: source,
		methods:    descriptor.methods,
	}
}

func validateMethods(methods []Method) error {
	seen := make(map[string]MethodKind, len(methods))
	queries := make(map[string]struct{}, len(methods))
	for _, method := range methods {
		if method.Name == "" {
			return fmt.Errorf("RPC method name is required")
		}
		if previous, exists := seen[method.Name]; exists {
			return fmt.Errorf("duplicate RPC method %q (%s and %s)", method.Name, previous, method.Kind)
		}
		seen[method.Name] = method.Kind
		if method.Kind == MethodQuery {
			queries[method.Name] = struct{}{}
			if len(method.Invalidates) > 0 {
				return fmt.Errorf("query %q cannot invalidate queries", method.Name)
			}
			if method.Cacheable && (method.MaxRetries < 0 || method.MaxRetries > 3) {
				return fmt.Errorf("query %q retry count must be between 0 and 3", method.Name)
			}
		} else if method.Kind == MethodAction {
			if method.Cacheable {
				return fmt.Errorf("action %q cannot be cacheable", method.Name)
			}
		} else {
			return fmt.Errorf("RPC method %q has unsupported kind %q", method.Name, method.Kind)
		}
	}
	for _, method := range methods {
		for _, invalidated := range method.Invalidates {
			if _, exists := queries[invalidated]; !exists {
				return fmt.Errorf("action %q invalidates unknown query %q", method.Name, invalidated)
			}
		}
	}
	return nil
}

func (component Component[Props]) FeatureID() string  { return component.featureID }
func (component Component[Props]) SourcePath() string { return component.sourcePath }
func (component Component[Props]) Methods() []Method {
	return append([]Method(nil), component.methods...)
}

func (component Component[Props]) Render(props Props) clienthost.RenderedFeature {
	return rendered[Props]{featureID: component.featureID, props: props}
}

type rendered[Props any] struct {
	featureID string
	props     Props
}

func (rendered rendered[Props]) FeatureID() string { return rendered.featureID }
func (rendered rendered[Props]) Props() any        { return rendered.props }

func FeatureID(packagePath, source string) string {
	sum := sha256.Sum256([]byte(strings.TrimSpace(packagePath) + "\x00" + path.Clean(source)))
	return fmt.Sprintf("solid-%x", sum[:12])
}

func callerPackage(function string) string {
	if index := strings.LastIndex(function, "."); index >= 0 {
		return function[:index]
	}
	return function
}

func validateSource(source string) error {
	if strings.TrimSpace(source) != source || !strings.HasPrefix(source, "./") || path.Ext(source) != ".tsx" {
		return fmt.Errorf("solid.Import: source %q must be a clean relative ./path.tsx", source)
	}
	clean := path.Clean(source)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return fmt.Errorf("solid.Import: source %q escapes the declaring Go package", source)
	}
	return nil
}
