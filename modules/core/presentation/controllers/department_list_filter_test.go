package controllers_test

import (
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/iota-uz/iota-sdk/modules"
	"github.com/iota-uz/iota-sdk/modules/core/permissions"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/itf"
	"github.com/stretchr/testify/require"
)

// TestDepartmentsController_List_FilterChips covers the chip bar of the
// departments list: parent (including "no parent") and status, combined with
// the out-of-band chip bar on the HTMX response and conditions carried into
// the next page of infinite scroll.
func TestDepartmentsController_List_FilterChips(t *testing.T) {
	suite := itf.NewSuiteBuilder(t).
		WithComponents(modules.Components()...).
		AsUser(permissions.DepartmentRead).
		Build()
	env := suite.Env()
	tenantID := env.TenantID()
	suite.Register(controllers.NewDepartmentsController(env.App))

	insert := func(code, name string, parent *uuid.UUID, status string) uuid.UUID {
		id := uuid.New()
		_, err := env.Tx.Exec(env.Ctx, `INSERT INTO core.departments (id, tenant_id, parent_id, code, name, status)
			VALUES ($1, $2, $3, $4, jsonb_build_object('en', $5::text), $6)`, id, tenantID, parent, code, name, status)
		require.NoError(t, err)
		return id
	}
	root := insert("CHIP-ROOT", "Chip Root", nil, "active")
	insert("CHIP-CHILD", "Chip Child", &root, "active")
	insert("CHIP-OLD", "Chip Archived", nil, "inactive")

	cases := []struct {
		name    string
		query   string
		present []string
		absent  []string
	}{
		{"top level", "?f=parent:empty:", []string{"Chip Root", "Chip Archived"}, []string{"Chip Child"}},
		// The child row names its parent in the "Parent" column, so only the
		// unrelated department must be gone.
		{"under parent", "?f=parent:is:" + root.String(), []string{"Chip Child"}, []string{"Chip Archived"}},
		{"status and parent", "?f=parent:empty:&f=status:is:inactive", []string{"Chip Archived"}, []string{"Chip Root", "Chip Child"}},
	}
	for _, tc := range cases {
		response := suite.GET("/departments" + tc.query).HTMX().Expect(t).Status(200)
		// Department names also appear as options of the "parent" chip, so
		// look at the table part of the response only.
		body := response.Body()
		rows, _, found := strings.Cut(body, `id="department-filters"`)
		require.True(t, found, "%s: the chip bar rides along out-of-band", tc.name)
		require.Contains(t, body, `hx-swap-oob="outerHTML"`)
		for _, name := range tc.present {
			require.Contains(t, rows, name, tc.name)
		}
		for _, name := range tc.absent {
			require.NotContains(t, rows, name, tc.name)
		}
	}

	page := suite.GET("/departments?f=status:is:inactive").Expect(t).Status(200)
	page.Contains(`value="status:is:inactive"`).Contains("filter-changed")
}
