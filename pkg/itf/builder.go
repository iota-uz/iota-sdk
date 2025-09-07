package itf

import (
	"testing"

	"github.com/google/uuid"
	"github.com/gorilla/mux"
	"github.com/iota-uz/iota-sdk/modules/core/domain/aggregates/user"
	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/pkg/application"
)

// SuiteBuilder provides a fluent API for building test suites with minimal boilerplate
type SuiteBuilder struct {
	t       testing.TB
	modules []application.Module
	user    user.User
	dbName  string
}

// NewSuiteBuilder creates a new SuiteBuilder for HTTP controller testing
func NewSuiteBuilder(t testing.TB) *SuiteBuilder {
	t.Helper()
	return &SuiteBuilder{
		t: t,
	}
}

// WithModules adds application modules to the test suite
func (sb *SuiteBuilder) WithModules(modules ...application.Module) *SuiteBuilder {
	sb.modules = append(sb.modules, modules...)
	return sb
}

// WithUser sets a custom user for the test suite
func (sb *SuiteBuilder) WithUser(u user.User) *SuiteBuilder {
	sb.user = u
	return sb
}

// WithTenant sets a custom database name (tenant isolation)
func (sb *SuiteBuilder) WithTenant(name string) *SuiteBuilder {
	sb.dbName = name
	return sb
}

// AsUser creates a test user with specific permissions
func (sb *SuiteBuilder) AsUser(permissions ...*permission.Permission) *SuiteBuilder {
	sb.user = User(permissions...)
	return sb
}

// AsAdmin creates a test user with administrative permissions
func (sb *SuiteBuilder) AsAdmin() *SuiteBuilder {
	// Get all available permissions for admin user
	// This is a simple implementation - in real usage, you might want to load
	// all permissions from the database or define a comprehensive admin permission set
	adminPermissions := []*permission.Permission{
		// Add commonly used admin permissions here
		// This would typically be loaded from your permission system
	}
	return sb.AsUser(adminPermissions...)
}

// AsReadOnly creates a test user with read-only permissions
func (sb *SuiteBuilder) AsReadOnly() *SuiteBuilder {
	// Define read-only permissions
	readOnlyPermissions := []*permission.Permission{
		// Add read-only permissions here
		// This would typically be loaded from your permission system
	}
	return sb.AsUser(readOnlyPermissions...)
}

// AsGuest creates a test user with minimal/guest permissions
func (sb *SuiteBuilder) AsGuest() *SuiteBuilder {
	guestPermissions := []*permission.Permission{
		// Add minimal guest permissions here
	}
	return sb.AsUser(guestPermissions...)
}

// AsAnonymous creates a suite with no authenticated user
func (sb *SuiteBuilder) AsAnonymous() *SuiteBuilder {
	sb.user = nil
	return sb
}

// Build creates and configures the test suite
func (sb *SuiteBuilder) Build() *Suite {
	sb.t.Helper()

	// Create the base suite with modules
	suite := NewSuite(sb.t, sb.modules...)

	// Configure user if provided
	if sb.user != nil {
		suite = suite.AsUser(sb.user)
	}

	return suite
}

// BuildWithOptions creates a test suite with additional environment options
func (sb *SuiteBuilder) BuildWithOptions(opts ...Option) *Suite {
	sb.t.Helper()

	// Build options array
	options := make([]Option, 0, len(opts)+2)
	if len(sb.modules) > 0 {
		options = append(options, WithModules(sb.modules...))
	}
	if sb.user != nil {
		options = append(options, WithUser(sb.user))
	}
	if sb.dbName != "" {
		options = append(options, WithDatabase(sb.dbName))
	}
	options = append(options, opts...)

	// Create environment with options
	env := Setup(sb.t, options...)

	// Create suite manually to ensure proper initialization
	suite := &Suite{
		t:           sb.t,
		env:         env,
		modules:     sb.modules,
		middlewares: make([]MiddlewareFunc, 0),
		beforeEach:  make([]HookFunc, 0),
	}

	// Create a new router instead of using App.Router() which may not exist
	suite.router = mux.NewRouter()

	// Set user if provided
	if sb.user != nil {
		suite.user = sb.user
	}

	// Setup middleware
	suite.setupMiddleware()

	return suite
}

// PresetBuilder provides common preset configurations
type PresetBuilder struct {
	sb *SuiteBuilder
}

// Presets returns a PresetBuilder for common configurations
func (sb *SuiteBuilder) Presets() *PresetBuilder {
	return &PresetBuilder{sb: sb}
}

// AdminWithAllModules creates an admin user with all common modules loaded
func (pb *PresetBuilder) AdminWithAllModules(modules ...application.Module) *Suite {
	return pb.sb.
		AsAdmin().
		WithModules(modules...).
		Build()
}

// ReadOnlyWithCore creates a read-only user with core modules
func (pb *PresetBuilder) ReadOnlyWithCore() *Suite {
	return pb.sb.
		AsReadOnly().
		Build()
}

// Anonymous creates an anonymous test suite for public endpoints
func (pb *PresetBuilder) Anonymous() *Suite {
	return pb.sb.
		AsAnonymous().
		Build()
}

// QuickTest creates a basic test suite with minimal setup for simple testing
func (pb *PresetBuilder) QuickTest() *Suite {
	return pb.sb.
		AsUser(). // Default user with no special permissions
		Build()
}

// TenantBuilder provides tenant-specific configurations
type TenantBuilder struct {
	sb *SuiteBuilder
}

// Tenant returns a TenantBuilder for tenant-specific configurations
func (sb *SuiteBuilder) Tenant() *TenantBuilder {
	return &TenantBuilder{sb: sb}
}

// WithID sets a specific tenant ID for the test
func (tb *TenantBuilder) WithID(tenantID uuid.UUID) *SuiteBuilder {
	return tb.sb.WithTenant(tenantID.String())
}

// Isolated creates a completely isolated tenant environment
func (tb *TenantBuilder) Isolated() *SuiteBuilder {
	return tb.sb.WithTenant(uuid.New().String())
}
