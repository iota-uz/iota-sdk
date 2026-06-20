// Package pagination provides typed configuration for pagination settings.
// It is a stdconfig package intended to be registered via config.Register[pagination.Config].
package pagination

import "fmt"

// Config holds page-size settings.
//
// Env prefix: "http.pagination" (e.g. PAGE_SIZE → http.pagination.pagesize).
type Config struct {
	// PageSize is the default number of items per page.
	PageSize int `koanf:"pagesize" default:"25"`
	// MaxPageSize is the maximum allowed page size.
	MaxPageSize int `koanf:"maxpagesize" default:"100"`
}

// ConfigPrefix returns the koanf prefix for pagination ("http.pagination").
func (Config) ConfigPrefix() string { return "http.pagination" }

// Validate checks that PageSize and MaxPageSize are positive and consistent.
func (c *Config) Validate() error {
	if c.PageSize <= 0 {
		return fmt.Errorf("paginationconfig: pagesize must be positive, got %d", c.PageSize)
	}
	if c.MaxPageSize < c.PageSize {
		return fmt.Errorf("paginationconfig: maxpagesize (%d) must be >= pagesize (%d)", c.MaxPageSize, c.PageSize)
	}
	return nil
}
