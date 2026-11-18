package appconfig_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/appconfig"
)

func TestParseEnvironment(t *testing.T) {
	t.Parallel()

	cases := []struct {
		raw   string
		want  appconfig.Environment
		known bool
	}{
		{"", appconfig.EnvironmentDevelopment, true},
		{"development", appconfig.EnvironmentDevelopment, true},
		{"dev", appconfig.EnvironmentDevelopment, true},
		{"local", appconfig.EnvironmentDevelopment, true},
		{"test", appconfig.EnvironmentTest, true},
		{"testing", appconfig.EnvironmentTest, true},
		{"staging", appconfig.EnvironmentStaging, true},
		{"stage", appconfig.EnvironmentStaging, true},
		{"preprod", appconfig.EnvironmentStaging, true},
		{" Staging ", appconfig.EnvironmentStaging, true},
		{"production", appconfig.EnvironmentProduction, true},
		{"PRODUCTION", appconfig.EnvironmentProduction, true},
		{"qa-42", appconfig.EnvironmentUnknown, false},
	}
	for _, tc := range cases {
		t.Run(tc.raw, func(t *testing.T) {
			t.Parallel()
			got, known := appconfig.ParseEnvironment(tc.raw)
			if got != tc.want || known != tc.known {
				t.Errorf("ParseEnvironment(%q) = (%q, %v), want (%q, %v)", tc.raw, got, known, tc.want, tc.known)
			}
		})
	}
}

func TestConfigEnvironmentPredicates(t *testing.T) {
	t.Parallel()

	type preds struct{ dev, test, staging, prod, local, deployed bool }
	cases := map[string]preds{
		"development": {dev: true, local: true},
		"test":        {test: true, local: true},
		"staging":     {staging: true, deployed: true},
		"preprod":     {staging: true, deployed: true},
		"production":  {prod: true, deployed: true},
		"qa-42":       {},
	}
	for raw, want := range cases {
		t.Run(raw, func(t *testing.T) {
			t.Parallel()
			cfg := appconfig.Config{Environment: raw}
			env := cfg.Env()
			got := preds{
				dev:      cfg.IsDevelopment(),
				test:     cfg.IsTest(),
				staging:  cfg.IsStaging(),
				prod:     cfg.IsProduction(),
				local:    env.IsLocal(),
				deployed: env.IsDeployed(),
			}
			if got != want {
				t.Errorf("environment=%q: got %+v, want %+v", raw, got, want)
			}
		})
	}
}

func TestIsDevelopment_FalseOnStaging(t *testing.T) {
	t.Parallel()

	cfg := appconfig.Config{Environment: "staging"}
	if cfg.IsDevelopment() {
		t.Error("IsDevelopment: expected false for environment=staging")
	}
}
