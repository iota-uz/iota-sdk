package pagination_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/pkg/config"
	"github.com/iota-uz/iota-sdk/pkg/config/providers/static"
	"github.com/iota-uz/iota-sdk/pkg/config/stdconfig/httpconfig/pagination"
)

func buildSource(t *testing.T, values map[string]any) config.Source {
	t.Helper()
	src, err := config.Build(static.New(values))
	if err != nil {
		t.Fatalf("config.Build: %v", err)
	}
	return src
}

func TestDefaults_Pagination(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, nil))
	cfg, err := config.Register[pagination.Config](r)
	if err != nil {
		t.Fatalf("Register: %v", err)
	}

	if cfg.PageSize != 25 {
		t.Errorf("PageSize default: got %d, want 25", cfg.PageSize)
	}
	if cfg.MaxPageSize != 100 {
		t.Errorf("MaxPageSize default: got %d, want 100", cfg.MaxPageSize)
	}
}

func TestValidate_MaxPageSizeLessThanPageSize(t *testing.T) {
	t.Parallel()

	r := config.NewRegistry(buildSource(t, map[string]any{
		"http.pagination.pagesize":    50,
		"http.pagination.maxpagesize": 10,
	}))
	_, err := config.Register[pagination.Config](r)
	if err == nil {
		t.Fatal("expected validation error when maxpagesize < pagesize, got nil")
	}
}
