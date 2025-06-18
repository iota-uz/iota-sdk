package datasource

import (
	"fmt"
	"sync"
)

// Registry manages data source instances
type Registry interface {
	// Register adds a data source to the registry
	Register(id string, dataSource DataSource) error
	
	// Unregister removes a data source from the registry
	Unregister(id string) error
	
	// Get retrieves a data source by ID
	Get(id string) (DataSource, error)
	
	// List returns all registered data source IDs
	List() []string
	
	// ListByType returns data source IDs filtered by type
	ListByType(dsType DataSourceType) []string
	
	// CreateFromConfig creates a data source from configuration
	CreateFromConfig(config DataSourceConfig) (DataSource, error)
}

// Factory creates data sources from configuration
type Factory interface {
	// Create creates a data source instance from configuration
	Create(config DataSourceConfig) (DataSource, error)
	
	// SupportedTypes returns the data source types this factory supports
	SupportedTypes() []DataSourceType
	
	// ValidateConfig validates a data source configuration
	ValidateConfig(config DataSourceConfig) error
}

// registry is the default implementation
type registry struct {
	mu          sync.RWMutex
	dataSources map[string]DataSource
	factories   map[DataSourceType]Factory
}

// NewRegistry creates a new data source registry
func NewRegistry() Registry {
	return &registry{
		dataSources: make(map[string]DataSource),
		factories:   make(map[DataSourceType]Factory),
	}
}

// Register adds a data source to the registry
func (r *registry) Register(id string, dataSource DataSource) error {
	if id == "" {
		return fmt.Errorf("data source ID cannot be empty")
	}
	
	if dataSource == nil {
		return fmt.Errorf("data source cannot be nil")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	if _, exists := r.dataSources[id]; exists {
		return fmt.Errorf("data source with ID %s already registered", id)
	}
	
	r.dataSources[id] = dataSource
	return nil
}

// Unregister removes a data source from the registry
func (r *registry) Unregister(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	dataSource, exists := r.dataSources[id]
	if !exists {
		return fmt.Errorf("data source with ID %s not found", id)
	}
	
	// Close the data source connection
	if err := dataSource.Close(); err != nil {
		return fmt.Errorf("failed to close data source %s: %w", id, err)
	}
	
	delete(r.dataSources, id)
	return nil
}

// Get retrieves a data source by ID
func (r *registry) Get(id string) (DataSource, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	dataSource, exists := r.dataSources[id]
	if !exists {
		return nil, fmt.Errorf("data source with ID %s not found", id)
	}
	
	return dataSource, nil
}

// List returns all registered data source IDs
func (r *registry) List() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	ids := make([]string, 0, len(r.dataSources))
	for id := range r.dataSources {
		ids = append(ids, id)
	}
	
	return ids
}

// ListByType returns data source IDs filtered by type
func (r *registry) ListByType(dsType DataSourceType) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	var ids []string
	for id, ds := range r.dataSources {
		if ds.GetMetadata().Type == dsType {
			ids = append(ids, id)
		}
	}
	
	return ids
}

// CreateFromConfig creates a data source from configuration
func (r *registry) CreateFromConfig(config DataSourceConfig) (DataSource, error) {
	if err := r.validateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}
	
	r.mu.RLock()
	factory, exists := r.factories[config.Type]
	r.mu.RUnlock()
	
	if !exists {
		return nil, fmt.Errorf("no factory registered for data source type: %s", config.Type)
	}
	
	return factory.Create(config)
}

// validateConfig validates a data source configuration
func (r *registry) validateConfig(config DataSourceConfig) error {
	if config.Type == "" {
		return fmt.Errorf("data source type is required")
	}
	
	if config.Name == "" {
		return fmt.Errorf("data source name is required")
	}
	
	if config.URL == "" {
		return fmt.Errorf("data source URL is required")
	}
	
	return nil
}

// RegisterFactory registers a factory for a specific data source type
func (r *registry) RegisterFactory(dsType DataSourceType, factory Factory) error {
	if factory == nil {
		return fmt.Errorf("factory cannot be nil")
	}
	
	r.mu.Lock()
	defer r.mu.Unlock()
	
	r.factories[dsType] = factory
	return nil
}

// GetRegisteredTypes returns all registered data source types
func (r *registry) GetRegisteredTypes() []DataSourceType {
	r.mu.RLock()
	defer r.mu.RUnlock()
	
	types := make([]DataSourceType, 0, len(r.factories))
	for dsType := range r.factories {
		types = append(types, dsType)
	}
	
	return types
}

// Close closes all registered data sources
func (r *registry) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	
	var errors []error
	
	for id, ds := range r.dataSources {
		if err := ds.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close data source %s: %w", id, err))
		}
	}
	
	// Clear all data
	r.dataSources = make(map[string]DataSource)
	
	if len(errors) > 0 {
		return fmt.Errorf("errors occurred while closing data sources: %v", errors)
	}
	
	return nil
}

// DefaultRegistry is a global registry instance
var DefaultRegistry = NewRegistry()

// Global convenience functions that use the default registry

// Register adds a data source to the default registry
func Register(id string, dataSource DataSource) error {
	return DefaultRegistry.Register(id, dataSource)
}

// Unregister removes a data source from the default registry
func Unregister(id string) error {
	return DefaultRegistry.Unregister(id)
}

// Get retrieves a data source by ID from the default registry
func Get(id string) (DataSource, error) {
	return DefaultRegistry.Get(id)
}

// List returns all registered data source IDs from the default registry
func List() []string {
	return DefaultRegistry.List()
}

// CreateFromConfig creates a data source from configuration using the default registry
func CreateFromConfig(config DataSourceConfig) (DataSource, error) {
	return DefaultRegistry.CreateFromConfig(config)
}