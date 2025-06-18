package executor

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
)

// Executor handles query execution across multiple data sources
type Executor interface {
	// Execute executes a query against a data source
	Execute(ctx context.Context, query ExecutionQuery) (*ExecutionResult, error)
	
	// ExecutePanel executes a query for a specific panel
	ExecutePanel(ctx context.Context, panel lens.PanelConfig, variables map[string]interface{}) (*ExecutionResult, error)
	
	// ExecuteDashboard executes all queries for a dashboard
	ExecuteDashboard(ctx context.Context, dashboard lens.DashboardConfig) (*DashboardResult, error)
	
	// RegisterDataSource registers a data source for execution
	RegisterDataSource(id string, ds datasource.DataSource) error
	
	// Close closes the executor and all registered data sources
	Close() error
}

// ExecutionQuery represents a query to be executed
type ExecutionQuery struct {
	DataSourceID string                 // ID of the data source to query
	Query        string                 // Query string with variables
	Variables    map[string]interface{} // Variables to interpolate
	TimeRange    lens.TimeRange         // Time range for the query
	MaxRows      int                    // Maximum rows to return
	Timeout      time.Duration          // Query timeout
	Format       datasource.QueryFormat // Expected result format
}

// ExecutionResult represents the result of query execution
type ExecutionResult struct {
	Data      []datasource.DataPoint // Query result data
	Metadata  ExecutionMetadata      // Execution metadata
	Error     error                  // Execution error if any
	Columns   []datasource.ColumnInfo // Column information
	ExecTime  time.Duration          // Execution time
	CacheHit  bool                   // Whether result came from cache
}

// ExecutionMetadata contains metadata about query execution
type ExecutionMetadata struct {
	QueryID        string    // Unique query identifier
	DataSourceID   string    // Data source used
	ExecutedAt     time.Time // Execution timestamp
	RowCount       int       // Number of rows returned
	ProcessingTime time.Duration // Processing time
	QueryHash      string    // Hash of the query for caching
}

// DashboardResult represents the result of executing all dashboard queries
type DashboardResult struct {
	PanelResults map[string]*ExecutionResult // Results by panel ID
	Errors       []error                     // Any errors encountered
	ExecutedAt   time.Time                   // When execution started
	Duration     time.Duration               // Total execution time
}

// executor is the default implementation
type executor struct {
	mu          sync.RWMutex
	dataSources map[string]datasource.DataSource
	registry    datasource.Registry
	timeout     time.Duration
}

// NewExecutor creates a new query executor
func NewExecutor(registry datasource.Registry, timeout time.Duration) Executor {
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	
	return &executor{
		dataSources: make(map[string]datasource.DataSource),
		registry:    registry,
		timeout:     timeout,
	}
}

// Execute executes a query against a data source
func (e *executor) Execute(ctx context.Context, query ExecutionQuery) (*ExecutionResult, error) {
	start := time.Now()
	
	// Get data source
	e.mu.RLock()
	ds, exists := e.dataSources[query.DataSourceID]
	e.mu.RUnlock()
	
	if !exists {
		// Try to get from registry
		var err error
		ds, err = e.registry.Get(query.DataSourceID)
		if err != nil {
			return nil, fmt.Errorf("data source not found: %s", query.DataSourceID)
		}
		
		// Cache it
		e.mu.Lock()
		e.dataSources[query.DataSourceID] = ds
		e.mu.Unlock()
	}
	
	// Set timeout
	timeout := query.Timeout
	if timeout == 0 {
		timeout = e.timeout
	}
	
	queryCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	
	// Build datasource query
	dsQuery := datasource.Query{
		ID:            generateQueryID(),
		Raw:           query.Query,
		Variables:     query.Variables,
		TimeRange:     query.TimeRange,
		MaxDataPoints: query.MaxRows,
		Format:        query.Format,
	}
	
	// Execute query
	result, err := ds.Query(queryCtx, dsQuery)
	if err != nil {
		return &ExecutionResult{
			Error:    err,
			ExecTime: time.Since(start),
			Metadata: ExecutionMetadata{
				QueryID:      dsQuery.ID,
				DataSourceID: query.DataSourceID,
				ExecutedAt:   start,
			},
		}, err
	}
	
	// Convert to execution result
	execResult := &ExecutionResult{
		Data:     result.Data,
		Columns:  result.Columns,
		ExecTime: time.Since(start),
		Metadata: ExecutionMetadata{
			QueryID:        dsQuery.ID,
			DataSourceID:   query.DataSourceID,
			ExecutedAt:     start,
			RowCount:       len(result.Data),
			ProcessingTime: result.ExecTime,
			QueryHash:      generateQueryHash(query),
		},
	}
	
	return execResult, nil
}

// ExecutePanel executes a query for a specific panel
func (e *executor) ExecutePanel(ctx context.Context, panel lens.PanelConfig, variables map[string]interface{}) (*ExecutionResult, error) {
	// Build execution query from panel config
	query := ExecutionQuery{
		DataSourceID: panel.DataSource.Ref,
		Query:        panel.Query,
		Variables:    variables,
		MaxRows:      1000, // Default max rows
		Format:       datasource.FormatTimeSeries, // Default format
	}
	
	// Set format based on panel type
	switch panel.Type {
	case lens.ChartTypeTable:
		query.Format = datasource.FormatTable
	case lens.ChartTypeLine, lens.ChartTypeArea, lens.ChartTypeBar, lens.ChartTypeColumn:
		query.Format = datasource.FormatTimeSeries
	}
	
	// Extract timeout from panel options
	if timeoutStr, ok := panel.Options["timeout"].(string); ok {
		if timeout, err := time.ParseDuration(timeoutStr); err == nil {
			query.Timeout = timeout
		}
	}
	
	// Extract max rows from panel options
	if maxRows, ok := panel.Options["maxRows"].(int); ok {
		query.MaxRows = maxRows
	}
	
	return e.Execute(ctx, query)
}

// ExecuteDashboard executes all queries for a dashboard
func (e *executor) ExecuteDashboard(ctx context.Context, dashboard lens.DashboardConfig) (*DashboardResult, error) {
	start := time.Now()
	result := &DashboardResult{
		PanelResults: make(map[string]*ExecutionResult),
		ExecutedAt:   start,
	}
	
	// Convert dashboard variables to map
	variables := make(map[string]interface{})
	for _, variable := range dashboard.Variables {
		variables[variable.Name] = variable.Value
	}
	
	// Execute queries for each panel concurrently
	var wg sync.WaitGroup
	var mu sync.Mutex
	
	for _, panel := range dashboard.Panels {
		wg.Add(1)
		go func(p lens.PanelConfig) {
			defer wg.Done()
			
			panelResult, err := e.ExecutePanel(ctx, p, variables)
			
			mu.Lock()
			defer mu.Unlock()
			
			if err != nil {
				result.Errors = append(result.Errors, fmt.Errorf("panel %s: %w", p.ID, err))
				// Store error result
				result.PanelResults[p.ID] = &ExecutionResult{
					Error: err,
					Metadata: ExecutionMetadata{
						DataSourceID: p.DataSource.Ref,
						ExecutedAt:   time.Now(),
					},
				}
			} else {
				result.PanelResults[p.ID] = panelResult
			}
		}(panel)
	}
	
	wg.Wait()
	result.Duration = time.Since(start)
	
	return result, nil
}

// RegisterDataSource registers a data source for execution
func (e *executor) RegisterDataSource(id string, ds datasource.DataSource) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	e.dataSources[id] = ds
	return nil
}

// Close closes the executor and all registered data sources
func (e *executor) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	var errors []error
	for id, ds := range e.dataSources {
		if err := ds.Close(); err != nil {
			errors = append(errors, fmt.Errorf("failed to close data source %s: %w", id, err))
		}
	}
	
	// Clear the map
	e.dataSources = make(map[string]datasource.DataSource)
	
	if len(errors) > 0 {
		return fmt.Errorf("errors closing data sources: %v", errors)
	}
	
	return nil
}

// Helper functions

func generateQueryID() string {
	return fmt.Sprintf("query_%d", time.Now().UnixNano())
}

func generateQueryHash(query ExecutionQuery) string {
	// Simple hash based on query content
	return fmt.Sprintf("%x", 
		fmt.Sprintf("%s:%s:%v", query.DataSourceID, query.Query, query.Variables))
}

// DefaultExecutor is a global executor instance
var DefaultExecutor Executor

// Initialize default executor
func init() {
	DefaultExecutor = NewExecutor(datasource.DefaultRegistry, 30*time.Second)
}

// Global convenience functions

// Execute executes a query using the default executor
func Execute(ctx context.Context, query ExecutionQuery) (*ExecutionResult, error) {
	return DefaultExecutor.Execute(ctx, query)
}

// ExecutePanel executes a panel query using the default executor
func ExecutePanel(ctx context.Context, panel lens.PanelConfig, variables map[string]interface{}) (*ExecutionResult, error) {
	return DefaultExecutor.ExecutePanel(ctx, panel, variables)
}

// ExecuteDashboard executes all dashboard queries using the default executor
func ExecuteDashboard(ctx context.Context, dashboard lens.DashboardConfig) (*DashboardResult, error) {
	return DefaultExecutor.ExecuteDashboard(ctx, dashboard)
}