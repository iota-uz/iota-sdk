package controllers

import (
	"sort"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/permission"
	"github.com/iota-uz/iota-sdk/modules/core/presentation/viewmodels"
	"github.com/iota-uz/iota-sdk/pkg/rbac"
)

// BuildResourcePermissionGroups builds permission groups organized by resource
// This is shared between RolesController and UsersController
func BuildResourcePermissionGroups(
	schema *rbac.PermissionSchema,
	selected ...*permission.Permission,
) []*viewmodels.ResourcePermissionGroup {
	if schema == nil || len(schema.Sets) == 0 {
		return nil
	}

	isSelected := func(p2 *permission.Permission) bool {
		for _, p1 := range selected {
			if p1.ID == p2.ID {
				return true
			}
		}
		return false
	}

	// Group permission sets by resource
	resourceMap := make(map[string][]*viewmodels.PermissionSetItem)

	for _, set := range schema.Sets {
		permissions := make([]*viewmodels.PermissionItem, 0, len(set.Permissions))
		checkedCount := 0

		// Determine the resource from the first permission in the set
		var resource string
		for _, perm := range set.Permissions {
			checked := isSelected(perm)
			if checked {
				checkedCount++
			}
			permissions = append(permissions, &viewmodels.PermissionItem{
				ID:      perm.ID.String(),
				Name:    perm.Name,
				Checked: checked,
			})

			// Get resource from the first permission
			if resource == "" && perm.Resource != "" {
				resource = string(perm.Resource)
			}
		}

		allChecked := checkedCount == len(set.Permissions) && len(set.Permissions) > 0
		partial := checkedCount > 0 && checkedCount < len(set.Permissions)

		// For single-permission sets, use the permission name as the label
		label := set.Label
		if len(set.Permissions) == 1 && set.Permissions[0] != nil {
			// Use the permission name which will be translated in the template
			label = set.Permissions[0].Name
		}

		permissionSet := &viewmodels.PermissionSetItem{
			Key:         set.Key,
			Label:       label,
			Description: set.Description,
			Checked:     allChecked,
			Partial:     partial,
			Permissions: permissions,
		}

		// Add to resource map
		if resource != "" {
			resourceMap[resource] = append(resourceMap[resource], permissionSet)
		}
	}

	// Convert map to sorted slice
	groups := make([]*viewmodels.ResourcePermissionGroup, 0, len(resourceMap))
	for resource, sets := range resourceMap {
		groups = append(groups, &viewmodels.ResourcePermissionGroup{
			Resource:       resource,
			PermissionSets: sets,
		})
	}

	// Sort by resource name
	sort.Slice(groups, func(i, j int) bool {
		return groups[i].Resource < groups[j].Resource
	})

	return groups
}

// BuildModulePermissionGroups builds permission groups organized by module
// Within each module, permissions are further grouped by resource
func BuildModulePermissionGroups(
	schema *rbac.PermissionSchema,
	selected ...*permission.Permission,
) []*viewmodels.ModulePermissionGroup {
	if schema == nil || len(schema.Sets) == 0 {
		return nil
	}

	isSelected := func(p2 *permission.Permission) bool {
		for _, p1 := range selected {
			if p1.ID == p2.ID {
				return true
			}
		}
		return false
	}

	// Group permission sets by module, then by resource within each module
	moduleResourceMap := make(map[string]map[string][]*viewmodels.PermissionSetItem)

	for _, set := range schema.Sets {
		permissions := make([]*viewmodels.PermissionItem, 0, len(set.Permissions))
		checkedCount := 0

		// Determine the resource from the first permission in the set
		var resource string
		for _, perm := range set.Permissions {
			checked := isSelected(perm)
			if checked {
				checkedCount++
			}
			permissions = append(permissions, &viewmodels.PermissionItem{
				ID:      perm.ID.String(),
				Name:    perm.Name,
				Checked: checked,
			})

			// Get resource from the first permission
			if resource == "" && perm.Resource != "" {
				resource = string(perm.Resource)
			}
		}

		allChecked := checkedCount == len(set.Permissions) && len(set.Permissions) > 0
		partial := checkedCount > 0 && checkedCount < len(set.Permissions)

		// For single-permission sets, use the permission name as the label
		label := set.Label
		if len(set.Permissions) == 1 && set.Permissions[0] != nil {
			// Use the permission name which will be translated in the template
			label = set.Permissions[0].Name
		}

		permissionSet := &viewmodels.PermissionSetItem{
			Key:         set.Key,
			Label:       label,
			Description: set.Description,
			Checked:     allChecked,
			Partial:     partial,
			Permissions: permissions,
		}

		// Group by module, then by resource
		module := set.Module
		if module == "" {
			module = "Other" // Fallback for sets without module
		}

		// Initialize module map if it doesn't exist
		if moduleResourceMap[module] == nil {
			moduleResourceMap[module] = make(map[string][]*viewmodels.PermissionSetItem)
		}

		// Add to resource within the module
		if resource != "" {
			moduleResourceMap[module][resource] = append(moduleResourceMap[module][resource], permissionSet)
		}
	}

	// Convert nested maps to sorted slice
	groups := make([]*viewmodels.ModulePermissionGroup, 0, len(moduleResourceMap))

	for module, resourceMap := range moduleResourceMap {
		// Convert resource map to sorted resource groups
		resourceGroups := make([]*viewmodels.ResourcePermissionGroup, 0, len(resourceMap))
		for resource, sets := range resourceMap {
			resourceGroups = append(resourceGroups, &viewmodels.ResourcePermissionGroup{
				Resource:       resource,
				PermissionSets: sets,
			})
		}

		// Sort resource groups by resource name
		sort.Slice(resourceGroups, func(i, j int) bool {
			return resourceGroups[i].Resource < resourceGroups[j].Resource
		})

		groups = append(groups, &viewmodels.ModulePermissionGroup{
			Module:         module,
			ResourceGroups: resourceGroups,
		})
	}

	// Sort by module name for consistent ordering
	sort.Slice(groups, func(i, j int) bool {
		// Put "Core" first, then alphabetical order
		if groups[i].Module == "Core" {
			return true
		}
		if groups[j].Module == "Core" {
			return false
		}
		return groups[i].Module < groups[j].Module
	})

	return groups
}
