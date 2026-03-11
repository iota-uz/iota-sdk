package policy

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"gopkg.in/yaml.v3"
)

func DefaultConfig() Config {
	return Config{
		Environments: map[string]EnvironmentPolicy{
			"development": {
				AllowedHosts:     []string{"localhost", "127.0.0.1", "::1", "db", "postgres"},
				AllowDestructive: true,
				RequireYes:       true,
				RequireTicket:    false,
			},
			"production": {
				AllowedHosts:     []string{},
				AllowDestructive: false,
				RequireYes:       true,
				RequireTicket:    true,
			},
		},
	}
}

func Load(path string) (Config, []byte, error) {
	const op serrors.Op = "dbctl.policy.Load"
	if strings.TrimSpace(path) == "" {
		cfg := DefaultConfig()
		payload, err := json.Marshal(cfg)
		if err != nil {
			return Config{}, nil, serrors.E(op, err, "marshal default policy")
		}
		return cfg, payload, nil
	}
	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, nil, serrors.E(op, err, "load policy file")
	}
	var cfg Config
	decoder := yaml.NewDecoder(bytes.NewReader(payload))
	decoder.KnownFields(true)
	if err := decoder.Decode(&cfg); err != nil {
		return Config{}, nil, serrors.E(op, err, "parse policy file")
	}
	if len(cfg.Environments) == 0 {
		return Config{}, nil, serrors.E(op, serrors.KindValidation, "policy has no environments")
	}
	return cfg, payload, nil
}

func HashPolicy(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func Evaluate(cfg Config, target Target, destructive bool) Decision {
	env := strings.TrimSpace(target.Environment)
	ep, ok := cfg.Environments[env]
	decision := Decision{
		Allowed: true,
	}
	if !ok {
		return decision.Denied(fmt.Sprintf("no policy configured for environment %q", env))
	}

	decision.RequireYes = ep.RequireYes
	decision.RequireTicket = ep.RequireTicket
	decision.AllowDestructive = ep.AllowDestructive

	if !hostAllowed(ep.AllowedHosts, target.Host) {
		return decision.Denied(fmt.Sprintf("host %q is not allowed by policy for %s", target.Host, env))
	}
	if destructive && !ep.AllowDestructive {
		return decision.Denied("destructive operation is forbidden by policy")
	}
	return decision
}

func hostAllowed(patterns []string, host string) bool {
	h := strings.ToLower(strings.TrimSpace(host))
	if h == "" {
		return false
	}
	for _, p := range patterns {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}
		if h == p {
			return true
		}
		if strings.HasPrefix(p, "*.") && strings.HasSuffix(h, strings.TrimPrefix(p, "*")) {
			return true
		}
	}
	return false
}
