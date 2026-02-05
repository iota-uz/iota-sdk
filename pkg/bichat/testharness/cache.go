package testharness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Cache is a local-dev cache used only to avoid repeated LLM judge calls.
// It MUST NOT cache execution (SSE/GraphQL), which should reflect the current server state.
type Cache struct {
	enabled bool
	dir     string
}

func NewCache(cfg Config) *Cache {
	if !cfg.CacheEnabled {
		return &Cache{enabled: false}
	}
	return &Cache{
		enabled: true,
		dir:     cfg.CacheDir,
	}
}

func (c *Cache) Enabled() bool {
	return c != nil && c.enabled
}

func (c *Cache) judgeFilePath(key string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("cache disabled")
	}
	if strings.TrimSpace(key) == "" {
		return "", errors.New("cache key is empty")
	}
	return filepath.Join(c.dir, "judge", key+".json"), nil
}

func (c *Cache) JudgeKey(model string, judgeUserPrompt string) string {
	h := sha256.New()
	_, _ = h.Write([]byte(model))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(judgeSystemPrompt))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(judgeUserPrompt))
	return hex.EncodeToString(h.Sum(nil))
}

func (c *Cache) LoadJudgeVerdict(key string) (*JudgeVerdict, bool, error) {
	p, err := c.judgeFilePath(key)
	if err != nil {
		return nil, false, err
	}
	b, err := os.ReadFile(p)
	if errors.Is(err, os.ErrNotExist) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var v JudgeVerdict
	if err := json.Unmarshal(b, &v); err != nil {
		return nil, false, err
	}
	return &v, true, nil
}

func (c *Cache) SaveJudgeVerdict(key string, verdict JudgeVerdict) error {
	p, err := c.judgeFilePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(verdict)
	if err != nil {
		return fmt.Errorf("marshal verdict: %w", err)
	}
	return os.WriteFile(p, b, 0o644)
}
