package policy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

const defaultPolicyPath = ".dbctl/policy.yaml"

var allowedCredentialEmission = map[string]struct{}{
	"masked":     {},
	"token_only": {},
}

func Load(path string) (Config, []byte, error) {
	if strings.TrimSpace(path) == "" {
		path = defaultPolicyPath
	}
	payload, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return Config{}, nil, fmt.Errorf("load policy file %s: %w", path, err)
	}
	var cfg Config
	if err := yaml.Unmarshal(payload, &cfg); err != nil {
		return Config{}, nil, fmt.Errorf("parse policy file %s: %w", path, err)
	}
	if len(cfg.Environments) == 0 {
		return Config{}, nil, fmt.Errorf("policy has no environments")
	}
	if strings.TrimSpace(cfg.Credentials.Emission) == "" {
		cfg.Credentials.Emission = "token_only"
	}
	if _, ok := allowedCredentialEmission[cfg.Credentials.Emission]; !ok {
		return Config{}, nil, fmt.Errorf("unsupported credentials.emission %q", cfg.Credentials.Emission)
	}
	if cfg.Credentials.TokenTTLSecond <= 0 {
		cfg.Credentials.TokenTTLSecond = 3600
	}
	return cfg, payload, nil
}

func HashPolicy(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func Evaluate(cfg Config, target Target, destructive bool) Decision {
	env := strings.TrimSpace(target.Environment)
	if env == "" {
		env = "development"
	}
	ep, ok := cfg.Environments[env]
	decision := Decision{
		Allowed:            true,
		CredentialEmission: cfg.Credentials.Emission,
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
