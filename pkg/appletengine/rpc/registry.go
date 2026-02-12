package rpc

import (
	"fmt"
	"strings"
	"sync"

	"github.com/gorilla/mux"
	"github.com/iota-uz/applets"
)

type visibility int

const (
	visibilityPublic visibility = iota + 1
	visibilityServerOnly
)

type MethodTarget string

const (
	MethodTargetGo  MethodTarget = "go"
	MethodTargetBun MethodTarget = "bun"
)

type Method struct {
	AppletName  string
	Name        string
	Visibility  visibility
	Target      MethodTarget
	Middlewares []mux.MiddlewareFunc
	Method      applets.RPCMethod
}

type Registry struct {
	mu      sync.RWMutex
	methods map[string]Method
}

func NewRegistry() *Registry {
	return &Registry{
		methods: make(map[string]Method),
	}
}

func (r *Registry) RegisterPublic(appletName, methodName string, method applets.RPCMethod, middlewares []mux.MiddlewareFunc) error {
	return r.RegisterPublicWithTarget(appletName, methodName, MethodTargetGo, method, middlewares)
}

func (r *Registry) RegisterPublicWithTarget(appletName, methodName string, target MethodTarget, method applets.RPCMethod, middlewares []mux.MiddlewareFunc) error {
	return r.register(Method{
		AppletName:  appletName,
		Name:        methodName,
		Visibility:  visibilityPublic,
		Target:      target,
		Middlewares: middlewares,
		Method:      method,
	})
}

func (r *Registry) RegisterServerOnly(appletName, methodName string, method applets.RPCMethod, middlewares []mux.MiddlewareFunc) error {
	return r.register(Method{
		AppletName:  appletName,
		Name:        methodName,
		Visibility:  visibilityServerOnly,
		Target:      MethodTargetGo,
		Middlewares: middlewares,
		Method:      method,
	})
}

func (r *Registry) register(method Method) error {
	name := strings.TrimSpace(method.Name)
	if name == "" {
		return fmt.Errorf("rpc registry: method name is required")
	}
	appletName := strings.TrimSpace(method.AppletName)
	if appletName == "" {
		return fmt.Errorf("rpc registry: applet name is required for method %q", name)
	}
	if !strings.HasPrefix(name, appletName+".") {
		return fmt.Errorf("rpc registry: method %q must be namespaced with %q", name, appletName+".")
	}
	if method.Method.Handler == nil {
		return fmt.Errorf("rpc registry: handler is required for method %q", name)
	}
	switch method.Target {
	case "", MethodTargetGo:
		method.Target = MethodTargetGo
	case MethodTargetBun:
		if method.Visibility != visibilityPublic {
			return fmt.Errorf("rpc registry: target %q is only valid for public methods (%q)", MethodTargetBun, name)
		}
	default:
		return fmt.Errorf("rpc registry: unsupported target %q for method %q", method.Target, name)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.methods[name]; exists {
		return fmt.Errorf("rpc registry: duplicate method %q", name)
	}
	method.Name = name
	method.AppletName = appletName
	r.methods[name] = method
	return nil
}

func (r *Registry) SetPublicTargetForApplet(appletName string, target MethodTarget) error {
	appletName = strings.TrimSpace(appletName)
	if appletName == "" {
		return fmt.Errorf("rpc registry: applet name is required")
	}
	switch target {
	case MethodTargetGo, MethodTargetBun:
	default:
		return fmt.Errorf("rpc registry: unsupported target %q", target)
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	for name, method := range r.methods {
		if method.Visibility != visibilityPublic || method.AppletName != appletName {
			continue
		}
		method.Target = target
		r.methods[name] = method
	}
	return nil
}

func (r *Registry) Get(methodName string) (Method, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.methods[methodName]
	return m, ok
}

func (r *Registry) CountPublic() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, m := range r.methods {
		if m.Visibility == visibilityPublic {
			count++
		}
	}
	return count
}

func (r *Registry) CountServerOnly() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, m := range r.methods {
		if m.Visibility == visibilityServerOnly {
			count++
		}
	}
	return count
}
