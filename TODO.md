# Lens Package API Refactoring TODO

## Overview
Refactor the lens package API to be more idiomatic, intuitive, and follow Go best practices. The current API has several pain points that we'll address systematically.

## Core Issues Identified

### 1. **Complex Configuration Structures**
- **Problem**: `PanelConfig` and `DashboardConfig` have deeply nested structures
- **Example**: Event configuration requires 4+ levels of nesting
- **Impact**: Hard to construct programmatically, verbose JSON serialization

### 2. **Inconsistent Builder API**
- **Problem**: Builder methods mix concerns (data + events + styling)
- **Example**: `OnDataPointClick()` vs `OnDrillDown()` - unclear which to use
- **Impact**: Cognitive overhead, multiple ways to do the same thing

### 3. **Scattered Type Definitions**
- **Problem**: Core types spread across `types.go`, `config.go`, `errors.go`
- **Example**: `Variable` in `types.go` but `ActionConfig` in `config.go`
- **Impact**: Hard to find related types, import confusion

### 4. **Complex Event System**
- **Problem**: Each event type has its own wrapper struct
- **Example**: `ClickEvent{Action: ActionConfig{...}}` - unnecessary nesting
- **Impact**: Boilerplate code, hard to understand

### 5. **No Fluent Data Source Setup**
- **Problem**: Imperative data source registration is error-prone
- **Example**: Manual `RegisterDataSource()` calls with error handling
- **Impact**: Verbose setup code, easy to forget validation

## Concrete API Improvements

### Current API (Before):
```go
// 1. Verbose panel creation
panel := builder.BarChart().
    ID("sales-by-region").
    Title("Sales by Region").
    DataSource("main-db").
    Query("SELECT region, sales_amount FROM sales_data").
    Position(0, 0).
    Size(6, 4).
    OnDataPointClick(lens.ActionConfig{
        Type: lens.ActionTypeDrillDown,
        DrillDown: &lens.DrillDownAction{
            Filters: map[string]string{
                "region": "{label}",
                "period": "{seriesName}",
            },
        },
    }).
    Build()

// 2. Complex dashboard creation
dashboard := builder.NewDashboard().
    ID("sales-dashboard").
    Title("Sales Dashboard").
    Description("Sales metrics and analysis").
    Grid(12, 120).
    Variable("year", "2024").
    Panel(panel).
    Build()

// 3. Manual executor setup
exec := executor.NewExecutor(nil, 30*time.Second)
pgDS, err := postgres.NewPostgreSQLDataSource(pgConfig)
if err != nil {
    return err
}
err = exec.RegisterDataSource("main-db", pgDS)
if err != nil {
    return err
}
```

### Proposed API (After):
```go
// 1. Simplified panel creation with intelligent defaults
panel := lens.BarChart("sales-by-region", "Sales by Region").
    Data("main-db", "SELECT region, sales_amount FROM sales_data").
    Position(0, 0, 6, 4).  // x, y, width, height in one call
    OnClick().DrillDown("region", "{label}", "period", "{seriesName}")

// 2. Fluent dashboard creation
dashboard := lens.Dashboard("sales-dashboard", "Sales Dashboard").
    Description("Sales metrics and analysis").
    Variable("year", "2024").
    Add(panel)

// 3. Fluent executor setup with validation
exec := lens.Setup().
    PostgreSQL("main-db", pgConfig).
    Timeout(30*time.Second).
    Build()
```

## Implementation Plan

### Phase 1: Core Types Consolidation (2-3 hours)
**Files to modify:**
- `pkg/lens/types.go` - Consolidate all core types
- `pkg/lens/config.go` - Simplify configuration structs
- `pkg/lens/errors.go` - Add structured error types

**Key Changes:**
1. Move all chart/panel types to `types.go`
2. Flatten nested configuration structures
3. Add convenience constructors for common types
4. Improve error types with better context

**Example:**
```go
// Before: Nested configuration
type PanelConfig struct {
    Events *PanelEvents
}
type PanelEvents struct {
    DataPoint *DataPointEvent
}
type DataPointEvent struct {
    Action ActionConfig
}

// After: Flattened with builder
type PanelConfig struct {
    Events []EventConfig  // Single slice instead of nested structs
}
type EventConfig struct {
    Trigger EventTrigger  // click, datapoint, legend, etc.
    Action  ActionConfig
}
```

### Phase 2: Simplified Builder API (3-4 hours)
**Files to modify:**
- `pkg/lens/builder/dashboard.go` - Streamline builder methods
- Add `pkg/lens/builder/fluent.go` - New fluent API functions

**Key Changes:**
1. Add convenience constructors: `lens.BarChart()`, `lens.Dashboard()`
2. Combine related methods: `Position(x, y, w, h)` instead of separate calls
3. Simplify event configuration with method chaining
4. Add validation at build time instead of runtime

**Example:**
```go
// Before: Multiple method calls
builder.BarChart().
    ID("sales").
    Title("Sales").
    Position(0, 0).
    Size(6, 4).
    OnDataPointClick(...)

// After: Combined methods with smart defaults
lens.BarChart("sales", "Sales").  // ID and title required
    Position(0, 0, 6, 4).         // Combined position/size
    OnClick().DrillDown(...)      // Fluent event config
```

### Phase 3: Event System Simplification (2 hours)
**Files to modify:**
- `pkg/lens/config.go` - Flatten event structures
- `pkg/lens/events.go` - Simplify event handling

**Key Changes:**
1. Remove wrapper event types (`ClickEvent`, `DataPointEvent`, etc.)
2. Use single `EventConfig` type with trigger field
3. Add fluent event configuration methods
4. Simplify JavaScript integration

**Example:**
```go
// Before: Multiple wrapper types
type ClickEvent struct {
    Action ActionConfig
}
type DataPointEvent struct {
    Action ActionConfig
}

// After: Single event type
type EventConfig struct {
    Trigger EventTrigger  // "click", "datapoint", "legend"
    Action  ActionConfig
}

// Fluent API
panel.OnClick().Navigate("/details/{label}")
panel.OnDataPoint().DrillDown("category", "{label}")
```

### Phase 4: Fluent Setup API (2 hours)
**Files to create:**
- `pkg/lens/setup.go` - New fluent setup API

**Key Changes:**
1. Create `Setup()` function for fluent configuration
2. Add built-in data source registration with validation
3. Simplify executor creation
4. Add connection testing and error recovery

**Example:**
```go
// New fluent setup API
exec := lens.Setup().
    PostgreSQL("main-db", "postgres://...").
    PostgreSQL("analytics", analyticsConfig).
    Redis("cache", redisConfig).
    Timeout(30 * time.Second).
    TestConnections().  // Validate all connections
    Build()
```

### Phase 5: Documentation and Examples (1 hour)
**Files to modify:**
- `pkg/lens/README.md` - Update with new API examples
- Add `pkg/lens/examples/` directory with common patterns

## Breaking Changes and Migration

### Breaking Changes:
1. `builder.NewDashboard()` → `lens.Dashboard()`
2. Event configuration structure changes
3. Some method signatures simplified

### Migration Guide:
```go
// Old way
dashboard := builder.NewDashboard().
    ID("dashboard-1").
    Title("My Dashboard").
    Build()

// New way  
dashboard := lens.Dashboard("dashboard-1", "My Dashboard")
```

### Backward Compatibility:
- Keep old builder methods as deprecated for 1 version
- Add migration warnings in logs
- Provide automated migration tool

## Success Metrics

### Code Quality Metrics:
- **Reduce boilerplate**: 60% fewer lines for common use cases
- **Type safety**: 100% compile-time validation for required fields
- **API consistency**: Single pattern for all chart types

### Developer Experience:
- **Learning curve**: New users can create dashboard in <10 lines
- **Documentation**: Every public method has examples
- **Error messages**: Actionable error messages with suggestions

### Performance:
- **Build time**: No performance regression in builder API
- **Memory usage**: Reduced allocations through better defaults
- **Validation**: Faster validation through compile-time checks

## Testing Strategy

### Unit Tests:
- Test all new convenience constructors
- Validate backward compatibility with existing configs
- Test error handling and validation

### Integration Tests:
- Test complete dashboard creation workflows
- Validate JSON serialization/deserialization
- Test with real data sources

### Migration Tests:
- Test old API still works (deprecated)
- Test migration from old to new API
- Validate no data loss during migration

This refactoring will make the lens package significantly more intuitive and reduce the learning curve for new users while maintaining all existing functionality.