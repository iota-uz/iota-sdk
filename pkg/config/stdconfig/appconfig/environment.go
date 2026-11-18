package appconfig

import "strings"

// Environment is the deployment environment the application runs in.
//
// Use Config.Env to obtain it; the raw Config.Environment string is kept for
// backwards compatibility and for values that are not one of the known
// environments.
type Environment string

const (
	// EnvironmentUnknown is returned for values that are not a recognised environment.
	// It is neither development nor production, so no environment-specific
	// behaviour is enabled for it.
	EnvironmentUnknown Environment = ""
	// EnvironmentDevelopment is a developer machine or a local stack.
	EnvironmentDevelopment Environment = "development"
	// EnvironmentTest is an automated test run (unit, integration, E2E).
	EnvironmentTest Environment = "test"
	// EnvironmentStaging is a shared pre-production deployment (staging, preprod).
	EnvironmentStaging Environment = "staging"
	// EnvironmentProduction is the production deployment.
	EnvironmentProduction Environment = "production"
)

// ParseEnvironment maps a raw APP_ENVIRONMENT value to an Environment.
// Matching is case-insensitive and ignores surrounding whitespace. An empty
// value maps to EnvironmentDevelopment (the Config default). Accepted aliases:
// "dev" and "local" (development), "testing" (test), "stage" and "preprod"
// (staging). The second result is false for unrecognised values, which map to
// EnvironmentUnknown.
func ParseEnvironment(raw string) (Environment, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "", "development", "dev", "local":
		return EnvironmentDevelopment, true
	case "test", "testing":
		return EnvironmentTest, true
	case "staging", "stage", "preprod":
		return EnvironmentStaging, true
	case "production":
		return EnvironmentProduction, true
	default:
		return EnvironmentUnknown, false
	}
}

// String returns the canonical name of the environment.
func (e Environment) String() string { return string(e) }

// IsDevelopment reports whether e is the development environment.
// It is false on staging, test and production.
func (e Environment) IsDevelopment() bool { return e == EnvironmentDevelopment }

// IsTest reports whether e is the test environment.
func (e Environment) IsTest() bool { return e == EnvironmentTest }

// IsStaging reports whether e is the staging environment.
func (e Environment) IsStaging() bool { return e == EnvironmentStaging }

// IsProduction reports whether e is the production environment.
func (e Environment) IsProduction() bool { return e == EnvironmentProduction }

// IsLocal reports whether e runs outside a shared deployment
// (development or test).
func (e Environment) IsLocal() bool { return e == EnvironmentDevelopment || e == EnvironmentTest }

// IsDeployed reports whether e is a shared deployment (staging or production).
func (e Environment) IsDeployed() bool { return e == EnvironmentStaging || e == EnvironmentProduction }
