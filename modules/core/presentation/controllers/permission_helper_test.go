package controllers_test

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/controllers"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
	"github.com/stretchr/testify/require"
)

func TestBuildModulePermissionGroups_SkipsModulesWithEmptyResources(t *testing.T) {
	// Create a schema with permissions that have no resources (empty string)
	schema := &rbac.PermissionSchema{
		Sets: []rbac.PermissionSet{
			{
				Key:         "admin_set",
				Label:       "Administration",
				Description: "Admin permissions",
				Module:      "Administration",
				Permissions: []permission.Permission{
					permission.New(
						permission.WithName("admin.action"),
						permission.WithResource(""), // Empty resource
						permission.WithAction("manage"),
					),
				},
			},
			{
				Key:         "logistics_set",
				Label:       "Logistics Management",
				Description: "Logistics permissions",
				Module:      "Logistics",
				Permissions: []permission.Permission{
					permission.New(
						permission.WithName("logistics.view"),
						permission.WithResource("warehouse"), // Non-empty resource
						permission.WithAction("read"),
					),
				},
			},
			{
				Key:         "aichat_set",
				Label:       "AI Chat",
				Description: "AI Chat permissions",
				Module:      "AIChat",
				Permissions: []permission.Permission{
					permission.New(
						permission.WithName("chat.use"),
						permission.WithResource(""), // Empty resource
						permission.WithAction("use"),
					),
				},
			},
		},
	}

	// Execute the function
	result := controllers.BuildModulePermissionGroups(schema)

	// Verify that only the Logistics module is returned (it has non-empty resources)
	require.Len(t, result, 1, "Expected only 1 module with non-empty resources")
	require.Equal(t, "Logistics", result[0].Module)
	require.Len(t, result[0].ResourceGroups, 1)
	require.Equal(t, "warehouse", result[0].ResourceGroups[0].Resource)
}

func TestBuildModulePermissionGroups_IncludesAllModulesWithResources(t *testing.T) {
	// Create a schema where all modules have resources
	schema := &rbac.PermissionSchema{
		Sets: []rbac.PermissionSet{
			{
				Key:         "user_set",
				Label:       "User Management",
				Description: "User permissions",
				Module:      "Core",
				Permissions: []permission.Permission{
					permission.New(
						permission.WithName("user.manage"),
						permission.WithResource("users"),
						permission.WithAction("manage"),
					),
				},
			},
			{
				Key:         "payment_set",
				Label:       "Payment Management",
				Description: "Payment permissions",
				Module:      "Finance",
				Permissions: []permission.Permission{
					permission.New(
						permission.WithName("payment.view"),
						permission.WithResource("payments"),
						permission.WithAction("read"),
					),
				},
			},
		},
	}

	// Execute the function
	result := controllers.BuildModulePermissionGroups(schema)

	// Verify that both modules are returned
	require.Len(t, result, 2, "Expected 2 modules with resources")

	// Core should come first (sorting logic prioritizes "Core" module)
	require.Equal(t, "Core", result[0].Module)
	require.Len(t, result[0].ResourceGroups, 1)
	require.Equal(t, "users", result[0].ResourceGroups[0].Resource)

	require.Equal(t, "Finance", result[1].Module)
	require.Len(t, result[1].ResourceGroups, 1)
	require.Equal(t, "payments", result[1].ResourceGroups[0].Resource)
}

func TestBuildModulePermissionGroups_ReturnsNilForNilSchema(t *testing.T) {
	result := controllers.BuildModulePermissionGroups(nil)
	require.Nil(t, result)
}

func TestBuildModulePermissionGroups_ReturnsNilForEmptySchema(t *testing.T) {
	schema := &rbac.PermissionSchema{
		Sets: []rbac.PermissionSet{},
	}
	result := controllers.BuildModulePermissionGroups(schema)
	require.Nil(t, result)
}
