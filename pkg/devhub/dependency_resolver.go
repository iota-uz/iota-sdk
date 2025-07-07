package devhub

import (
	"fmt"
)

// DependencyResolver handles service dependency ordering
type DependencyResolver struct {
	services map[string]ServiceConfig
}

func NewDependencyResolver(configs []ServiceConfig) *DependencyResolver {
	services := make(map[string]ServiceConfig)
	for _, cfg := range configs {
		services[cfg.Name] = cfg
	}
	return &DependencyResolver{services: services}
}

// GetStartOrder returns services in the order they should be started
func (d *DependencyResolver) GetStartOrder() ([]string, error) {
	visited := make(map[string]bool)
	inProgress := make(map[string]bool)
	order := []string{}

	var visit func(name string) error
	visit = func(name string) error {
		if visited[name] {
			return nil
		}

		if inProgress[name] {
			return fmt.Errorf("circular dependency detected involving service: %s", name)
		}

		inProgress[name] = true

		if svc, exists := d.services[name]; exists {
			// Visit dependencies first
			for _, dep := range svc.Needs {
				if _, depExists := d.services[dep]; !depExists {
					return fmt.Errorf("service %s depends on non-existent service: %s", name, dep)
				}
				if err := visit(dep); err != nil {
					return err
				}
			}
		}

		visited[name] = true
		delete(inProgress, name)
		order = append(order, name)

		return nil
	}

	// Visit all services
	for name := range d.services {
		if err := visit(name); err != nil {
			return nil, err
		}
	}

	return order, nil
}

// GetDependents returns services that depend on the given service
func (d *DependencyResolver) GetDependents(serviceName string) []string {
	dependents := []string{}

	for name, svc := range d.services {
		for _, dep := range svc.Needs {
			if dep == serviceName {
				dependents = append(dependents, name)
				break
			}
		}
	}

	return dependents
}
