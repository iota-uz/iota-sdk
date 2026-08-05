// Package rpc provides this package.
package rpc

import (
	"fmt"
	"sort"
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
	Contract    MethodContract
}

type MethodKind string

const (
	MethodKindQuery    MethodKind = "query"
	MethodKindMutation MethodKind = "mutation"
)

// MethodContract is the serializable server-state policy consumed by the
// generated client-host RPC adapter. Request and response types still come
// from the existing TypedRPCRouter generator.
type MethodContract struct {
	Namespace   string     `json:"namespace"`
	Method      string     `json:"method"`
	Kind        MethodKind `json:"kind"`
	Cacheable   bool       `json:"cacheable,omitempty"`
	MaxRetries  int        `json:"maxRetries,omitempty"`
	Invalidates []string   `json:"invalidates,omitempty"`
}

type ContractOption func(*MethodContract)

func Query(cacheable bool, maxRetries int) ContractOption {
	return func(contract *MethodContract) {
		contract.Kind = MethodKindQuery
		contract.Cacheable = cacheable
		if maxRetries < 0 {
			maxRetries = 0
		}
		if maxRetries > 3 {
			maxRetries = 3
		}
		contract.MaxRetries = maxRetries
	}
}

func Invalidates(methods ...string) ContractOption {
	return func(contract *MethodContract) {
		seen := make(map[string]struct{}, len(methods))
		for _, method := range methods {
			method = strings.TrimSpace(method)
			if method == "" {
				continue
			}
			if _, exists := seen[method]; exists {
				continue
			}
			seen[method] = struct{}{}
			contract.Invalidates = append(contract.Invalidates, method)
		}
	}
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

// RegisterPublicContract registers a public method plus the policy used to
// generate standard React query/mutation clients.
func (r *Registry) RegisterPublicContract(appletName, methodName string, method applets.RPCMethod, middlewares []mux.MiddlewareFunc, options ...ContractOption) error {
	parts := strings.SplitN(strings.TrimSpace(methodName), ".", 2)
	contract := MethodContract{Kind: MethodKindMutation, Method: strings.TrimSpace(methodName)}
	if len(parts) == 2 {
		contract.Namespace = parts[0]
	}
	for _, option := range options {
		if option != nil {
			option(&contract)
		}
	}
	return r.register(Method{AppletName: appletName, Name: methodName, Visibility: visibilityPublic, Target: MethodTargetGo, Middlewares: middlewares, Method: method, Contract: contract})
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
	parts := strings.SplitN(name, ".", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("rpc registry: method %q must be namespaced as <namespace>.<method>", name)
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
	if method.Contract.Method == "" {
		method.Contract = MethodContract{Namespace: parts[0], Method: name, Kind: MethodKindMutation}
	}
	r.methods[name] = method
	return nil
}

// PublicContracts returns a deterministic defensive catalog for typegen and
// development diagnostics.
func (r *Registry) PublicContracts() []MethodContract {
	r.mu.RLock()
	defer r.mu.RUnlock()
	contracts := make([]MethodContract, 0, len(r.methods))
	for _, method := range r.methods {
		if method.Visibility != visibilityPublic {
			continue
		}
		contract := method.Contract
		contract.Invalidates = append([]string(nil), contract.Invalidates...)
		contracts = append(contracts, contract)
	}
	sort.Slice(contracts, func(i, j int) bool { return contracts[i].Method < contracts[j].Method })
	return contracts
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
