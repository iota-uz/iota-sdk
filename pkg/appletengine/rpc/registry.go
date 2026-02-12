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

type Method struct {
	AppletName  string
	Name        string
	Visibility  visibility
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
	return r.register(Method{
		AppletName:  appletName,
		Name:        methodName,
		Visibility:  visibilityPublic,
		Middlewares: middlewares,
		Method:      method,
	})
}

func (r *Registry) RegisterServerOnly(appletName, methodName string, method applets.RPCMethod, middlewares []mux.MiddlewareFunc) error {
	return r.register(Method{
		AppletName:  appletName,
		Name:        methodName,
		Visibility:  visibilityServerOnly,
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
