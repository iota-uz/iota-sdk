package executor

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"github.com/iota-uz/iota-sdk/pkg/lens"
	"github.com/iota-uz/iota-sdk/pkg/lens/datasource"
)

// MockDataSource is a mock implementation of DataSource for testing
type MockDataSource struct {
	mock.Mock
}

func (m *MockDataSource) Query(ctx context.Context, query datasource.Query) (*datasource.QueryResult, error) {
	args := m.Called(ctx, query)
	return args.Get(0).(*datasource.QueryResult), args.Error(1)
}

func (m *MockDataSource) TestConnection(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockDataSource) GetMetadata() datasource.DataSourceMetadata {
	args := m.Called()
	return args.Get(0).(datasource.DataSourceMetadata)
}

func (m *MockDataSource) ValidateQuery(query datasource.Query) error {
	args := m.Called(query)
	return args.Error(0)
}

func (m *MockDataSource) Close() error {
	args := m.Called()
	return args.Error(0)
}

// MockRegistry is a mock implementation of Registry for testing
type MockRegistry struct {
	mock.Mock
}

func (m *MockRegistry) Register(id string, dataSource datasource.DataSource) error {
	args := m.Called(id, dataSource)
	return args.Error(0)
}

func (m *MockRegistry) Unregister(id string) error {
	args := m.Called(id)
	return args.Error(0)
}

func (m *MockRegistry) Get(id string) (datasource.DataSource, error) {
	args := m.Called(id)
	return args.Get(0).(datasource.DataSource), args.Error(1)
}

func (m *MockRegistry) List() []string {
	args := m.Called()
	return args.Get(0).([]string)
}

func (m *MockRegistry) ListByType(dsType datasource.DataSourceType) []string {
	args := m.Called(dsType)
	return args.Get(0).([]string)
}

func (m *MockRegistry) CreateFromConfig(config datasource.DataSourceConfig) (datasource.DataSource, error) {
	args := m.Called(config)
	return args.Get(0).(datasource.DataSource), args.Error(1)
}

func TestExecutor_Execute(t *testing.T) {
	tests := []struct {
		name           string
		setupMocks     func(*MockDataSource, *MockRegistry)
		query          ExecutionQuery
		expectedResult *ExecutionResult
		expectError    bool
	}{
		{
			name: "successful query execution",
			setupMocks: func(ds *MockDataSource, reg *MockRegistry) {
				ds.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
					&datasource.QueryResult{
						Data: []datasource.DataPoint{
							{
								Timestamp: time.Now(),
								Value:     42.0,
								Labels:    map[string]string{"metric": "cpu"},
								Fields:    map[string]interface{}{"host": "server1"},
							},
						},
						Metadata: datasource.ResultMetadata{
							QueryID:    "test-query",
							RowCount:   1,
							DataSource: "test-ds",
						},
						ExecTime: 100 * time.Millisecond,
					}, nil)
			},
			query: ExecutionQuery{
				DataSourceID: "test-ds",
				Query:        "SELECT * FROM metrics",
				Variables:    map[string]interface{}{"host": "server1"},
				Format:       datasource.FormatTimeSeries,
			},
			expectError: false,
		},
		{
			name: "data source not found",
			setupMocks: func(ds *MockDataSource, reg *MockRegistry) {
				reg.On("Get", "nonexistent-ds").Return((*MockDataSource)(nil), assert.AnError)
			},
			query: ExecutionQuery{
				DataSourceID: "nonexistent-ds",
				Query:        "SELECT * FROM metrics",
			},
			expectError: true,
		},
		{
			name: "query execution error",
			setupMocks: func(ds *MockDataSource, reg *MockRegistry) {
				ds.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
					(*datasource.QueryResult)(nil), assert.AnError)
			},
			query: ExecutionQuery{
				DataSourceID: "test-ds",
				Query:        "INVALID SQL",
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup mocks
			mockDS := new(MockDataSource)
			mockRegistry := new(MockRegistry)
			
			if tt.setupMocks != nil {
				tt.setupMocks(mockDS, mockRegistry)
			}
			
			// Create executor
			executor := NewExecutor(mockRegistry, 30*time.Second)
			
			// Register data source if needed
			if tt.query.DataSourceID == "test-ds" {
				err := executor.RegisterDataSource("test-ds", mockDS)
				require.NoError(t, err)
			}
			
			// Execute query
			result, err := executor.Execute(context.Background(), tt.query)
			
			// Assert results
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.query.DataSourceID, result.Metadata.DataSourceID)
			}
			
			// Verify all expectations
			mockDS.AssertExpectations(t)
			mockRegistry.AssertExpectations(t)
		})
	}
}

func TestExecutor_ExecutePanel(t *testing.T) {
	// Setup mocks
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	mockDS.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		&datasource.QueryResult{
			Data: []datasource.DataPoint{
				{
					Timestamp: time.Now(),
					Value:     100.0,
					Labels:    map[string]string{"panel": "cpu-usage"},
				},
			},
			Metadata: datasource.ResultMetadata{
				QueryID:    "panel-query",
				RowCount:   1,
				DataSource: "postgres",
			},
			ExecTime: 50 * time.Millisecond,
		}, nil)
	
	// Create executor and register data source
	executor := NewExecutor(mockRegistry, 30*time.Second)
	err := executor.RegisterDataSource("postgres", mockDS)
	require.NoError(t, err)
	
	// Create test panel
	panel := lens.PanelConfig{
		ID:    "cpu-panel",
		Title: "CPU Usage",
		Type:  lens.ChartTypeLine,
		DataSource: lens.DataSourceConfig{
			Type: "postgres",
			Ref:  "postgres",
		},
		Query: "SELECT timestamp, cpu_percent FROM metrics WHERE host = $host",
		Options: map[string]interface{}{
			"maxRows": 500,
			"timeout": "10s",
		},
	}
	
	variables := map[string]interface{}{
		"host": "server1",
	}
	
	// Execute panel query
	result, err := executor.ExecutePanel(context.Background(), panel, variables)
	
	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, "postgres", result.Metadata.DataSourceID)
	assert.Len(t, result.Data, 1)
	
	// Verify expectations
	mockDS.AssertExpectations(t)
}

func TestExecutor_ExecuteDashboard(t *testing.T) {
	// Setup mocks
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	// Mock multiple queries for different panels
	mockDS.On("Query", mock.Anything, mock.MatchedBy(func(q datasource.Query) bool {
		return q.Raw == "SELECT * FROM cpu_metrics"
	})).Return(&datasource.QueryResult{
		Data: []datasource.DataPoint{{Timestamp: time.Now(), Value: 80.0}},
		Metadata: datasource.ResultMetadata{QueryID: "cpu-query", RowCount: 1},
	}, nil)
	
	mockDS.On("Query", mock.Anything, mock.MatchedBy(func(q datasource.Query) bool {
		return q.Raw == "SELECT * FROM memory_metrics"
	})).Return(&datasource.QueryResult{
		Data: []datasource.DataPoint{{Timestamp: time.Now(), Value: 60.0}},
		Metadata: datasource.ResultMetadata{QueryID: "memory-query", RowCount: 1},
	}, nil)
	
	// Create executor and register data source
	executor := NewExecutor(mockRegistry, 30*time.Second)
	err := executor.RegisterDataSource("postgres", mockDS)
	require.NoError(t, err)
	
	// Create test dashboard
	dashboard := lens.DashboardConfig{
		ID:   "test-dashboard",
		Name: "Test Dashboard",
		Variables: []lens.Variable{
			{Name: "environment", Value: "production"},
		},
		Panels: []lens.PanelConfig{
			{
				ID:    "cpu-panel",
				Title: "CPU Usage",
				Type:  lens.ChartTypeLine,
				DataSource: lens.DataSourceConfig{
					Type: "postgres",
					Ref:  "postgres",
				},
				Query: "SELECT * FROM cpu_metrics",
			},
			{
				ID:    "memory-panel",
				Title: "Memory Usage",
				Type:  lens.ChartTypeBar,
				DataSource: lens.DataSourceConfig{
					Type: "postgres", 
					Ref:  "postgres",
				},
				Query: "SELECT * FROM memory_metrics",
			},
		},
	}
	
	// Execute dashboard
	result, err := executor.ExecuteDashboard(context.Background(), dashboard)
	
	// Assert results
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.PanelResults, 2)
	assert.Contains(t, result.PanelResults, "cpu-panel")
	assert.Contains(t, result.PanelResults, "memory-panel")
	assert.Empty(t, result.Errors)
	
	// Verify expectations
	mockDS.AssertExpectations(t)
}

func TestExecutor_RegisterDataSource(t *testing.T) {
	mockRegistry := new(MockRegistry)
	executor := NewExecutor(mockRegistry, 30*time.Second)
	
	mockDS := new(MockDataSource)
	
	// Register data source
	err := executor.RegisterDataSource("test-ds", mockDS)
	assert.NoError(t, err)
	
	// Try to register again - should succeed (overwrite)
	err = executor.RegisterDataSource("test-ds", mockDS)
	assert.NoError(t, err)
}

func TestExecutor_Close(t *testing.T) {
	mockRegistry := new(MockRegistry)
	executor := NewExecutor(mockRegistry, 30*time.Second)
	
	mockDS1 := new(MockDataSource)
	mockDS2 := new(MockDataSource)
	
	// Setup close expectations
	mockDS1.On("Close").Return(nil)
	mockDS2.On("Close").Return(nil)
	
	// Register data sources
	err := executor.RegisterDataSource("ds1", mockDS1)
	require.NoError(t, err)
	err = executor.RegisterDataSource("ds2", mockDS2)
	require.NoError(t, err)
	
	// Close executor
	err = executor.Close()
	assert.NoError(t, err)
	
	// Verify expectations
	mockDS1.AssertExpectations(t)
	mockDS2.AssertExpectations(t)
}

func TestGenerateQueryID(t *testing.T) {
	id1 := generateQueryID()
	id2 := generateQueryID()
	
	assert.NotEmpty(t, id1)
	assert.NotEmpty(t, id2)
	assert.NotEqual(t, id1, id2) // Should be unique
	assert.Contains(t, id1, "query_")
}

func TestGenerateQueryHash(t *testing.T) {
	query1 := ExecutionQuery{
		DataSourceID: "ds1",
		Query:        "SELECT * FROM table1",
		Variables:    map[string]interface{}{"var1": "value1"},
	}
	
	query2 := ExecutionQuery{
		DataSourceID: "ds1", 
		Query:        "SELECT * FROM table1",
		Variables:    map[string]interface{}{"var1": "value1"},
	}
	
	query3 := ExecutionQuery{
		DataSourceID: "ds1",
		Query:        "SELECT * FROM table2", // Different query
		Variables:    map[string]interface{}{"var1": "value1"},
	}
	
	hash1 := generateQueryHash(query1)
	hash2 := generateQueryHash(query2)
	hash3 := generateQueryHash(query3)
	
	assert.Equal(t, hash1, hash2) // Same queries should have same hash
	assert.NotEqual(t, hash1, hash3) // Different queries should have different hash
}

func TestExecutor_ConcurrentExecution(t *testing.T) {
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	// Mock multiple concurrent queries
	mockDS.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		&datasource.QueryResult{
			Data: []datasource.DataPoint{
				{Timestamp: time.Now(), Value: 100.0},
			},
			Metadata: datasource.ResultMetadata{QueryID: "concurrent", RowCount: 1},
		}, nil).Maybe()
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	err := executor.RegisterDataSource("test-ds", mockDS)
	require.NoError(t, err)
	
	// Execute multiple queries concurrently
	const numQueries = 10
	queries := make([]ExecutionQuery, numQueries)
	for i := 0; i < numQueries; i++ {
		queries[i] = ExecutionQuery{
			DataSourceID: "test-ds",
			Query:        fmt.Sprintf("SELECT * FROM metrics_%d", i),
			Format:       datasource.FormatTimeSeries,
		}
	}
	
	results := make([]*ExecutionResult, numQueries)
	errors := make([]error, numQueries)
	var wg sync.WaitGroup
	
	// Execute all queries concurrently
	for i := 0; i < numQueries; i++ {
		wg.Add(1)
		go func(index int) {
			defer wg.Done()
			results[index], errors[index] = executor.Execute(context.Background(), queries[index])
		}(i)
	}
	
	wg.Wait()
	
	// Verify all queries succeeded
	for i := 0; i < numQueries; i++ {
		assert.NoError(t, errors[i], "Query %d failed", i)
		assert.NotNil(t, results[i], "Result %d is nil", i)
		assert.Equal(t, "test-ds", results[i].Metadata.DataSourceID)
	}
}

func TestExecutor_TimeoutHandling(t *testing.T) {
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	// Mock a slow query that times out
	mockDS.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		func(ctx context.Context, query datasource.Query) *datasource.QueryResult {
			// Simulate a slow query
			select {
			case <-time.After(2 * time.Second):
				return &datasource.QueryResult{
					Data: []datasource.DataPoint{{Timestamp: time.Now(), Value: 42.0}},
					Metadata: datasource.ResultMetadata{QueryID: "slow", RowCount: 1},
				}
			case <-ctx.Done():
				return &datasource.QueryResult{}
			}
		},
		func(ctx context.Context, query datasource.Query) error {
			// Simulate a slow query
			select {
			case <-time.After(2 * time.Second):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}).Maybe()
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	err := executor.RegisterDataSource("slow-ds", mockDS)
	require.NoError(t, err)
	
	// Test with short timeout
	query := ExecutionQuery{
		DataSourceID: "slow-ds",
		Query:        "SELECT * FROM slow_table",
		Timeout:      100 * time.Millisecond,
	}
	
	start := time.Now()
	result, err := executor.Execute(context.Background(), query)
	duration := time.Since(start)
	
	// Should timeout quickly
	assert.True(t, duration < 500*time.Millisecond, "Query should have timed out quickly")
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.NotNil(t, result.Error)
}

func TestExecutor_DataSourceCaching(t *testing.T) {
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	// Mock registry to return data source only once
	mockRegistry.On("Get", "cached-ds").Return(mockDS, nil).Once()
	
	mockDS.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		&datasource.QueryResult{
			Data: []datasource.DataPoint{{Timestamp: time.Now(), Value: 42.0}},
			Metadata: datasource.ResultMetadata{QueryID: "cached", RowCount: 1},
		}, nil).Twice()
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	
	query := ExecutionQuery{
		DataSourceID: "cached-ds",
		Query:        "SELECT * FROM metrics",
		Format:       datasource.FormatTimeSeries,
	}
	
	// First execution - should fetch from registry
	result1, err1 := executor.Execute(context.Background(), query)
	assert.NoError(t, err1)
	assert.NotNil(t, result1)
	
	// Second execution - should use cached data source
	result2, err2 := executor.Execute(context.Background(), query)
	assert.NoError(t, err2)
	assert.NotNil(t, result2)
	
	// Verify registry was called only once
	mockRegistry.AssertExpectations(t)
	mockDS.AssertExpectations(t)
}

func TestExecutor_ErrorPropagation(t *testing.T) {
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	testError := errors.New("database connection failed")
	mockDS.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		(*datasource.QueryResult)(nil), testError)
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	err := executor.RegisterDataSource("error-ds", mockDS)
	require.NoError(t, err)
	
	query := ExecutionQuery{
		DataSourceID: "error-ds",
		Query:        "SELECT * FROM metrics",
	}
	
	result, err := executor.Execute(context.Background(), query)
	
	// Should return error but not fail completely
	assert.Error(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, testError, result.Error)
	assert.Equal(t, "error-ds", result.Metadata.DataSourceID)
	assert.NotZero(t, result.ExecTime)
}

func TestExecutor_ExecuteDashboard_PartialFailure(t *testing.T) {
	mockDS1 := new(MockDataSource)
	mockDS2 := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	// Mock first data source to succeed
	mockDS1.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		&datasource.QueryResult{
			Data: []datasource.DataPoint{{Timestamp: time.Now(), Value: 80.0}},
			Metadata: datasource.ResultMetadata{QueryID: "success", RowCount: 1},
		}, nil)
	
	// Mock second data source to fail
	testError := errors.New("query failed")
	mockDS2.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		(*datasource.QueryResult)(nil), testError)
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	executor.RegisterDataSource("success-ds", mockDS1)
	executor.RegisterDataSource("error-ds", mockDS2)
	
	dashboard := lens.DashboardConfig{
		ID:   "mixed-results-dashboard",
		Name: "Mixed Results Dashboard",
		Panels: []lens.PanelConfig{
			{
				ID:    "success-panel",
				Title: "Success Panel",
				Type:  lens.ChartTypeLine,
				DataSource: lens.DataSourceConfig{
					Type: "postgres",
					Ref:  "success-ds",
				},
				Query: "SELECT * FROM success_metrics",
			},
			{
				ID:    "error-panel",
				Title: "Error Panel",
				Type:  lens.ChartTypeBar,
				DataSource: lens.DataSourceConfig{
					Type: "postgres",
					Ref:  "error-ds",
				},
				Query: "SELECT * FROM error_metrics",
			},
		},
	}
	
	result, err := executor.ExecuteDashboard(context.Background(), dashboard)
	
	// Should not return error at dashboard level
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Len(t, result.PanelResults, 2)
	
	// Check that one succeeded and one failed
	assert.Len(t, result.Errors, 1)
	assert.Contains(t, result.Errors[0].Error(), "error-panel")
	
	// Verify individual panel results
	successResult := result.PanelResults["success-panel"]
	assert.NotNil(t, successResult)
	assert.NoError(t, successResult.Error)
	assert.Len(t, successResult.Data, 1)
	
	errorResult := result.PanelResults["error-panel"]
	assert.NotNil(t, errorResult)
	assert.Error(t, errorResult.Error)
	assert.Equal(t, testError, errorResult.Error)
}

func TestExecutor_PanelOptionsParsing(t *testing.T) {
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	mockDS.On("Query", mock.Anything, mock.MatchedBy(func(q datasource.Query) bool {
		return q.MaxDataPoints == 500 && q.Format == datasource.FormatTable
	})).Return(&datasource.QueryResult{
		Data: []datasource.DataPoint{{Timestamp: time.Now(), Value: 42.0}},
		Metadata: datasource.ResultMetadata{QueryID: "options", RowCount: 1},
	}, nil)
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	executor.RegisterDataSource("options-ds", mockDS)
	
	panel := lens.PanelConfig{
		ID:    "options-panel",
		Title: "Options Panel",
		Type:  lens.ChartTypeTable,
		DataSource: lens.DataSourceConfig{
			Type: "postgres",
			Ref:  "options-ds",
		},
		Query: "SELECT * FROM metrics",
		Options: map[string]interface{}{
			"maxRows": 500,
			"timeout": "5s",
		},
	}
	
	result, err := executor.ExecutePanel(context.Background(), panel, nil)
	
	assert.NoError(t, err)
	assert.NotNil(t, result)
	mockDS.AssertExpectations(t)
}

func BenchmarkExecutor_Execute(b *testing.B) {
	// Setup
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	mockDS.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		&datasource.QueryResult{
			Data: []datasource.DataPoint{
				{Timestamp: time.Now(), Value: 42.0},
			},
			Metadata: datasource.ResultMetadata{QueryID: "bench", RowCount: 1},
		}, nil)
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	executor.RegisterDataSource("test-ds", mockDS)
	
	query := ExecutionQuery{
		DataSourceID: "test-ds",
		Query:        "SELECT * FROM metrics",
		Format:       datasource.FormatTimeSeries,
	}
	
	// Benchmark
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.Execute(context.Background(), query)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkExecutor_ExecuteDashboard(b *testing.B) {
	mockDS := new(MockDataSource)
	mockRegistry := new(MockRegistry)
	
	mockDS.On("Query", mock.Anything, mock.AnythingOfType("datasource.Query")).Return(
		&datasource.QueryResult{
			Data: []datasource.DataPoint{{Timestamp: time.Now(), Value: 42.0}},
			Metadata: datasource.ResultMetadata{QueryID: "bench", RowCount: 1},
		}, nil)
	
	executor := NewExecutor(mockRegistry, 30*time.Second)
	executor.RegisterDataSource("bench-ds", mockDS)
	
	// Create dashboard with multiple panels
	panels := make([]lens.PanelConfig, 10)
	for i := 0; i < 10; i++ {
		panels[i] = lens.PanelConfig{
			ID:    fmt.Sprintf("panel-%d", i),
			Title: fmt.Sprintf("Panel %d", i),
			Type:  lens.ChartTypeLine,
			DataSource: lens.DataSourceConfig{
				Type: "postgres",
				Ref:  "bench-ds",
			},
			Query: fmt.Sprintf("SELECT * FROM metrics_%d", i),
		}
	}
	
	dashboard := lens.DashboardConfig{
		ID:     "bench-dashboard",
		Name:   "Benchmark Dashboard",
		Panels: panels,
	}
	
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := executor.ExecuteDashboard(context.Background(), dashboard)
		if err != nil {
			b.Fatal(err)
		}
	}
}