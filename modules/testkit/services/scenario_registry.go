package services

import (
	"context"
	"fmt"

	"github.com/iota-uz/iota-sdk/pkg/testenv"
)

// RegisterScenarios exposes existing presets through the versioned registry.
// These presets mutate common baseline records and require a dedicated database.
// Cleanup is provided by the environment owner rather than truncating a shared DB.
func (s *TestDataService) RegisterScenarios(registry *testenv.Registry, cleanup testenv.CleanupFunc) error {
	for _, entry := range s.GetAvailableScenarios() {
		name, ok := entry["name"].(string)
		if !ok {
			return fmt.Errorf("invalid preset name")
		}
		if err := registry.Register(testenv.Definition{Name: name, Version: "1", Isolation: "dedicated", RequiredCapabilities: []string{"postgres"}, InputSchema: map[string]any{"type": "object", "additionalProperties": false}, OutputSchema: map[string]any{"type": "object"}}, func(ctx context.Context, input testenv.Input) (testenv.Result, error) {
			preset, exists := s.getScenario(input.Name)
			if !exists {
				return testenv.Result{}, fmt.Errorf("preset no longer available")
			}
			data, err := s.PopulateData(ctx, preset)
			return testenv.Result{Data: data}, err
		}, cleanup); err != nil {
			return err
		}
	}
	return nil
}
