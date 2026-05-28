package controllers

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/iota-uz/go-i18n/v2/i18n"
	"github.com/iota-uz/iota-sdk/modules/core/infrastructure/persistence"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/modules/core/services"
	"github.com/iota-uz/iota-sdk/pkg/intl"
	"github.com/iota-uz/iota-sdk/pkg/serrors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// TestBuildParentOptions_ExcludesSubtree verifies the cycle guard: when editing
// a department, that department and its descendants must be removed from the
// parent-select options so the user cannot reparent it under its own subtree.
func TestBuildParentOptions_ExcludesSubtree(t *testing.T) {
	t.Parallel()
	vms := []*viewmodels.Department{
		{ID: "root", Name: "Root"},
		{ID: "child", Name: "Child"},
		{ID: "grandchild", Name: "Grandchild"},
		{ID: "sibling", Name: "Sibling"},
	}
	// Editing "child": exclude child + its descendant grandchild.
	excluded := map[string]struct{}{"child": {}, "grandchild": {}}

	opts := buildParentOptions(vms, excluded)

	require.Len(t, opts, 2)
	got := make(map[string]struct{}, len(opts))
	for _, o := range opts {
		got[o.ID] = struct{}{}
	}
	assert.Contains(t, got, "root")
	assert.Contains(t, got, "sibling")
	assert.NotContains(t, got, "child")
	assert.NotContains(t, got, "grandchild")
}

// TestDepartmentValidationFieldError covers the controller-side helper that
// turns a service or repository error into a (status, field-errors) pair so
// the drawer can re-render with a per-field message instead of leaking a 500.
// Each known sentinel must produce a 4xx + a single field key; unknown errors
// must yield (0, nil) so the caller falls through to its generic 5xx path.
func TestDepartmentValidationFieldError(t *testing.T) {
	t.Parallel()

	// Minimal localizer that returns the requested key verbatim — keeps the
	// assertions tight to the field/key mapping logic instead of i18n strings.
	bundle := i18n.NewBundle(language.English)
	for _, key := range []string{
		"Departments.Errors.SelfLoop",
		"Departments.Errors.Cycle",
		"Departments.Errors.ParentNotFound",
		"Departments.Errors.DuplicateCode",
	} {
		require.NoError(t, bundle.AddMessages(language.English, &i18n.Message{ID: key, Other: key}))
	}
	ctx := intl.WithLocalizer(context.Background(), i18n.NewLocalizer(bundle, "en"))

	tests := []struct {
		name        string
		err         error
		wantStatus  int
		wantField   string
		wantMessage string
	}{
		{
			name:        "self-loop sentinel → ParentID field, 400",
			err:         fmt.Errorf("wrap: %w", services.ErrDepartmentSelfLoop),
			wantStatus:  http.StatusBadRequest,
			wantField:   "ParentID",
			wantMessage: "Departments.Errors.SelfLoop",
		},
		{
			name:        "cycle sentinel → ParentID field, 400",
			err:         fmt.Errorf("wrap: %w", services.ErrDepartmentCycle),
			wantStatus:  http.StatusBadRequest,
			wantField:   "ParentID",
			wantMessage: "Departments.Errors.Cycle",
		},
		{
			name:        "parent-not-found sentinel → ParentID field, 400",
			err:         fmt.Errorf("wrap: %w", services.ErrDepartmentParentNotFound),
			wantStatus:  http.StatusBadRequest,
			wantField:   "ParentID",
			wantMessage: "Departments.Errors.ParentNotFound",
		},
		{
			name:        "duplicate-code sentinel → Code field, 409",
			err:         fmt.Errorf("wrap: %w", persistence.ErrDepartmentDuplicateCode),
			wantStatus:  http.StatusConflict,
			wantField:   "Code",
			wantMessage: "Departments.Errors.DuplicateCode",
		},
		{
			name:        "sentinel inside a serrors.E chain still matches",
			err:         serrors.E(serrors.Op("test"), serrors.KindValidation, fmt.Errorf("xyz: %w", services.ErrDepartmentCycle)),
			wantStatus:  http.StatusBadRequest,
			wantField:   "ParentID",
			wantMessage: "Departments.Errors.Cycle",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			status, fields := departmentValidationFieldError(ctx, tc.err)
			require.Equal(t, tc.wantStatus, status)
			require.Len(t, fields, 1)
			assert.Equal(t, tc.wantMessage, fields[tc.wantField])
		})
	}

	t.Run("unknown error → (0, nil) for fall-through", func(t *testing.T) {
		status, fields := departmentValidationFieldError(ctx, errors.New("something else"))
		assert.Equal(t, 0, status)
		assert.Nil(t, fields)
	})

	t.Run("nil error → (0, nil)", func(t *testing.T) {
		status, fields := departmentValidationFieldError(ctx, nil)
		assert.Equal(t, 0, status)
		assert.Nil(t, fields)
	})

	t.Run("no localizer in ctx → (0, nil) so caller falls through", func(t *testing.T) {
		status, fields := departmentValidationFieldError(context.Background(), services.ErrDepartmentSelfLoop)
		assert.Equal(t, 0, status)
		assert.Nil(t, fields)
	})
}
