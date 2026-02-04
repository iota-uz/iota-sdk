package testharness

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

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

func (c *Cache) Key(suite TestSuite, cfg Config) (string, error) {
	suite = normalizeSuiteForCache(suite)

	suiteBytes, err := json.Marshal(suite)
	if err != nil {
		return "", fmt.Errorf("marshal suite: %w", err)
	}

	h := sha256.New()
	_, _ = h.Write(suiteBytes)
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(cfg.JudgeModel))
	_, _ = h.Write([]byte{0})
	if cfg.IotaSDKRevision != "" {
		_, _ = h.Write([]byte(cfg.IotaSDKRevision))
		_, _ = h.Write([]byte{0})
	}
	if cfg.HostRevision != "" {
		_, _ = h.Write([]byte(cfg.HostRevision))
		_, _ = h.Write([]byte{0})
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func (c *Cache) filePath(key string) (string, error) {
	if !c.Enabled() {
		return "", errors.New("cache disabled")
	}
	if key == "" {
		return "", errors.New("cache key is empty")
	}
	return filepath.Join(c.dir, key+".json"), nil
}

func (c *Cache) LoadReport(key string) (*RunReport, bool, error) {
	p, err := c.filePath(key)
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
	var r RunReport
	if err := json.Unmarshal(b, &r); err != nil {
		return nil, false, err
	}
	return &r, true, nil
}

func (c *Cache) SaveReport(key string, report RunReport) error {
	p, err := c.filePath(key)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, err := json.Marshal(report)
	if err != nil {
		return err
	}
	return os.WriteFile(p, b, 0o644)
}

func normalizeSuiteForCache(suite TestSuite) TestSuite {
	for i := range suite.Tests {
		perms := suite.Tests[i].UserPermissions
		if len(perms) == 0 {
			continue
		}
		cp := append([]string(nil), perms...)
		sort.Strings(cp)
		suite.Tests[i].UserPermissions = cp
	}
	return suite
}
