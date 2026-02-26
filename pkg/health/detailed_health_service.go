package health

import (
	"context"
	"fmt"
	"reflect"
	"sync"
	"time"
)

// CheckFunc runs a single health check with the provided context.
type CheckFunc func(ctx context.Context) HealthCheck

// DetailedHealthService exposes detailed diagnostics for internal health views.
type DetailedHealthService interface {
	// GetDetailedHealth returns aggregated check status and capabilities.
	GetDetailedHealth(ctx context.Context) *DetailedHealth
}

// DetailedHealthServiceConfig controls checks, capability probes, and cache TTL.
type DetailedHealthServiceConfig struct {
	Checks       map[string]CheckFunc
	Capabilities CapabilityService
	CacheTTL     time.Duration
}

type detailedHealthServiceImpl struct {
	checks       map[string]CheckFunc
	capabilities CapabilityService
	cacheTTL     time.Duration
	cacheMu      sync.RWMutex
	cacheAt      time.Time
	cacheValue   *DetailedHealth
}

// NewDetailedHealthService constructs a diagnostics service instance.
func NewDetailedHealthService(cfg DetailedHealthServiceConfig) DetailedHealthService {
	checks := make(map[string]CheckFunc, len(cfg.Checks))
	for key, check := range cfg.Checks {
		checks[key] = check
	}
	// keep behavior consistent for nil config maps
	if cfg.Checks == nil {
		checks = map[string]CheckFunc{}
	}
	capabilities := cfg.Capabilities
	if capabilities == nil {
		capabilities = NewCapabilityService(nil)
	}

	cacheTTL := cfg.CacheTTL
	if cacheTTL <= 0 {
		cacheTTL = 3 * time.Second
	}

	return &detailedHealthServiceImpl{
		checks:       checks,
		capabilities: capabilities,
		cacheTTL:     cacheTTL,
	}
}

// GetDetailedHealth returns cached diagnostics when available, otherwise recomputes checks.
func (s *detailedHealthServiceImpl) GetDetailedHealth(ctx context.Context) *DetailedHealth {
	if cached := s.loadFromCache(); cached != nil {
		return cached
	}

	checks := make(map[string]HealthCheck, len(s.checks))
	for key, check := range s.checks {
		if check == nil {
			continue
		}
		checks[key] = safeRunCheck(key, check, ctx)
	}

	health := &DetailedHealth{
		Status:       aggregateStatus(checks),
		Timestamp:    time.Now().UTC(),
		Checks:       checks,
		Capabilities: s.capabilities.GetCapabilities(ctx),
	}

	s.storeInCache(health)
	return cloneDetailedHealth(health)
}

// aggregateStatus reduces individual check statuses into an overall status.
func aggregateStatus(checks map[string]HealthCheck) Status {
	status := StatusHealthy
	for _, check := range checks {
		switch check.Status {
		case StatusDown:
			return StatusDown
		case StatusDegraded, StatusUnknown, StatusDisabled:
			status = StatusDegraded
		case StatusHealthy:
			// keep current aggregate status
		}
	}

	return status
}

func safeRunCheck(name string, check CheckFunc, ctx context.Context) HealthCheck {
	result := HealthCheck{
		Status:  StatusDown,
		Message: "health check failed",
	}

	defer func() {
		if recovered := recover(); recovered != nil {
			result = HealthCheck{
				Status:  StatusDown,
				Message: fmt.Sprintf("health check %q panicked: %v", name, recovered),
			}
		}
	}()

	if check != nil {
		result = check(ctx)
	}

	return result
}

func (s *detailedHealthServiceImpl) loadFromCache() *DetailedHealth {
	s.cacheMu.RLock()
	defer s.cacheMu.RUnlock()

	if s.cacheValue == nil {
		return nil
	}
	if time.Since(s.cacheAt) > s.cacheTTL {
		return nil
	}

	return cloneDetailedHealth(s.cacheValue)
}

func (s *detailedHealthServiceImpl) storeInCache(health *DetailedHealth) {
	s.cacheMu.Lock()
	defer s.cacheMu.Unlock()

	s.cacheAt = time.Now()
	s.cacheValue = cloneDetailedHealth(health)
}

func cloneDetailedHealth(health *DetailedHealth) *DetailedHealth {
	if health == nil {
		return nil
	}

	checks := make(map[string]HealthCheck, len(health.Checks))
	for key, check := range health.Checks {
		cloned := check
		if check.Details != nil {
			cloned.Details = make(map[string]any, len(check.Details))
			for detailKey, detailValue := range check.Details {
				cloned.Details[detailKey] = cloneDetailsValue(detailValue)
			}
		}
		checks[key] = cloned
	}

	capabilities := make([]Capability, len(health.Capabilities))
	copy(capabilities, health.Capabilities)

	return &DetailedHealth{
		Status:       health.Status,
		Timestamp:    health.Timestamp,
		Checks:       checks,
		Capabilities: capabilities,
	}
}

// cloneDetailsValue performs a controlled deep copy for mutable detail values.
func cloneDetailsValue(value any) any {
	if value == nil {
		return nil
	}

	if typed, ok := value.(map[string]any); ok {
		cloned := make(map[string]any, len(typed))
		for detailKey, detailValue := range typed {
			cloned[detailKey] = cloneDetailsValue(detailValue)
		}
		return cloned
	}

	if typed, ok := value.([]any); ok {
		cloned := make([]any, len(typed))
		for i, detailValue := range typed {
			cloned[i] = cloneDetailsValue(detailValue)
		}
		return cloned
	}

	rv := reflect.ValueOf(value)
	switch rv.Kind() {
	case reflect.Interface:
		if rv.IsNil() {
			return nil
		}
		return cloneDetailsValue(rv.Interface())
	case reflect.Invalid:
		return nil
	case reflect.Bool:
		return rv.Bool()
	case reflect.Int:
		return rv.Int()
	case reflect.Int8:
		return int8(rv.Int())
	case reflect.Int16:
		return int16(rv.Int())
	case reflect.Int32:
		return int32(rv.Int())
	case reflect.Int64:
		return rv.Int()
	case reflect.Uint:
		return rv.Uint()
	case reflect.Uint8:
		return uint8(rv.Uint())
	case reflect.Uint16:
		return uint16(rv.Uint())
	case reflect.Uint32:
		return uint32(rv.Uint())
	case reflect.Uint64:
		return rv.Uint()
	case reflect.Uintptr:
		return rv.Uint()
	case reflect.Float32:
		return float32(rv.Float())
	case reflect.Float64:
		return rv.Float()
	case reflect.Complex64:
		return rv.Complex()
	case reflect.Complex128:
		return rv.Complex()
	case reflect.String:
		return rv.String()
	case reflect.Array:
		cloned := reflect.New(rv.Type()).Elem()
		for idx := 0; idx < rv.Len(); idx++ {
			clonedValue := cloneDetailsValue(rv.Index(idx).Interface())
			if clonedValue == nil {
				cloned.Index(idx).Set(reflect.Zero(rv.Type().Elem()))
				continue
			}
			clonedReflectValue := reflect.ValueOf(clonedValue)
			if !clonedReflectValue.Type().AssignableTo(rv.Type().Elem()) {
				if clonedReflectValue.Type().ConvertibleTo(rv.Type().Elem()) {
					clonedReflectValue = clonedReflectValue.Convert(rv.Type().Elem())
				} else {
					continue
				}
			}
			cloned.Index(idx).Set(clonedReflectValue)
		}
		return cloned.Interface()
	case reflect.Chan:
		return value
	case reflect.Func:
		return value
	case reflect.UnsafePointer:
		return value
	case reflect.Map:
		if rv.IsNil() {
			return nil
		}
		if rv.Type().Key().Kind() != reflect.String {
			return value
		}
		cloned := reflect.MakeMapWithSize(rv.Type(), rv.Len())
		for _, mapKey := range rv.MapKeys() {
			clonedValue := cloneDetailsValue(rv.MapIndex(mapKey).Interface())
			if clonedValue == nil {
				cloned.SetMapIndex(mapKey, reflect.Zero(rv.Type().Elem()))
				continue
			}
			clonedReflectValue := reflect.ValueOf(clonedValue)
			if !clonedReflectValue.Type().AssignableTo(rv.Type().Elem()) {
				if clonedReflectValue.Type().ConvertibleTo(rv.Type().Elem()) {
					clonedReflectValue = clonedReflectValue.Convert(rv.Type().Elem())
				} else {
					continue
				}
			}
			cloned.SetMapIndex(mapKey, clonedReflectValue)
		}
		return cloned.Interface()
	case reflect.Slice:
		if rv.IsNil() {
			return nil
		}
		cloned := reflect.MakeSlice(rv.Type(), rv.Len(), rv.Len())
		for idx := 0; idx < rv.Len(); idx++ {
			clonedValue := cloneDetailsValue(rv.Index(idx).Interface())
			if clonedValue == nil {
				cloned.Index(idx).Set(reflect.Zero(rv.Type().Elem()))
				continue
			}
			clonedReflectValue := reflect.ValueOf(clonedValue)
			if !clonedReflectValue.Type().AssignableTo(rv.Type().Elem()) {
				if clonedReflectValue.Type().ConvertibleTo(rv.Type().Elem()) {
					clonedReflectValue = clonedReflectValue.Convert(rv.Type().Elem())
				} else {
					continue
				}
			}
			cloned.Index(idx).Set(clonedReflectValue)
		}
		return cloned.Interface()
	case reflect.Pointer:
		if rv.IsNil() {
			return nil
		}
		cloned := reflect.New(rv.Type().Elem())
		elemValue := cloneDetailsValue(rv.Elem().Interface())
		if elemValue != nil {
			elemReflectValue := reflect.ValueOf(elemValue)
			if !elemReflectValue.Type().AssignableTo(cloned.Elem().Type()) {
				if elemReflectValue.Type().ConvertibleTo(cloned.Elem().Type()) {
					elemReflectValue = elemReflectValue.Convert(cloned.Elem().Type())
				} else {
					cloned.Elem().Set(rv.Elem())
					return cloned.Interface()
				}
			}
			cloned.Elem().Set(elemReflectValue)
		}
		return cloned.Interface()
	case reflect.Struct:
		cloned := reflect.New(rv.Type()).Elem()
		for idx := 0; idx < rv.NumField(); idx++ {
			field := rv.Field(idx)
			if !field.CanInterface() {
				continue
			}
			targetField := cloned.Field(idx)
			if !targetField.CanSet() {
				continue
			}
			clonedValue := reflect.ValueOf(cloneDetailsValue(field.Interface()))
			if !clonedValue.IsValid() {
				continue
			}
			if clonedValue.Type().AssignableTo(targetField.Type()) {
				targetField.Set(clonedValue)
			} else if clonedValue.Type().ConvertibleTo(targetField.Type()) {
				targetField.Set(clonedValue.Convert(targetField.Type()))
			}
		}
		return cloned.Interface()
	default:
		return value
	}
}
