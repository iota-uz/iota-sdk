package lens

import (
	"encoding/json"
)

// FromJSON creates a DashboardConfig from JSON string with validation
func FromJSON(jsonStr string) (*DashboardConfig, error) {
	var config DashboardConfig
	err := json.Unmarshal([]byte(jsonStr), &config)
	if err != nil {
		return nil, err
	}

	validator := NewValidator()
	result := validator.Validate(&config)
	if !result.IsValid() {
		return nil, &ValidationError{
			Field:   "config",
			Message: "validation failed after JSON deserialization",
		}
	}

	return &config, nil
}

// FromJSONBytes creates a DashboardConfig from JSON bytes with validation
func FromJSONBytes(data []byte) (*DashboardConfig, error) {
	var config DashboardConfig
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}

	validator := NewValidator()
	result := validator.Validate(&config)
	if !result.IsValid() {
		return nil, &ValidationError{
			Field:   "config",
			Message: "validation failed after JSON deserialization",
		}
	}

	return &config, nil
}

// FromJSONUnsafe creates a DashboardConfig from JSON string without validation
func FromJSONUnsafe(jsonStr string) (*DashboardConfig, error) {
	var config DashboardConfig
	err := json.Unmarshal([]byte(jsonStr), &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// FromJSONBytesUnsafe creates a DashboardConfig from JSON bytes without validation
func FromJSONBytesUnsafe(data []byte) (*DashboardConfig, error) {
	var config DashboardConfig
	err := json.Unmarshal(data, &config)
	if err != nil {
		return nil, err
	}
	return &config, nil
}
