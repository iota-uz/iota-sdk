package controllers

import (
	"testing"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/stretchr/testify/require"
)

func TestDashboardController_RouteAndNavUseConfiguredPermissions(t *testing.T) {
	viewDashboard := permission.New(
		permission.WithName("dashboard.view"),
		permission.WithResource("dashboard"),
		permission.WithAction(permission.ActionRead),
	)
	controller := &DashboardController{navPermissions: []permission.Permission{viewDashboard}}

	descriptor := controller.Descriptor()
	require.Len(t, descriptor.Routes, 1)
	require.Equal(t, "/", descriptor.Routes[0].Path)
	require.Equal(t, []permission.Permission{viewDashboard}, descriptor.Routes[0].Auth.Permissions)

	require.Len(t, descriptor.Nav, 1)
	require.NotNil(t, descriptor.Nav[0].Visibility)
	require.Equal(t, []permission.Permission{viewDashboard}, descriptor.Nav[0].Visibility.Permissions)
}
