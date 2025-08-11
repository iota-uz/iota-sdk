package viewmodels

type Permission struct {
	ID       string
	Name     string
	Resource string
	Action   string
	Modifier string
}

func (p *Permission) DisplayName() string {
	if p.Name != "" {
		return p.Name
	}

	return p.Action + " " + p.Resource + " (" + p.Modifier + ")"
}

// PermissionItem represents a single permission with its selection state
// Used for UI rendering in forms
type PermissionItem struct {
	ID      string
	Name    string
	Checked bool
}

// PermissionGroup represents a group of permissions categorized by resource
// Used for UI rendering in forms
type PermissionGroup struct {
	Resource    string
	Permissions []*PermissionItem
}

// PermissionSetItem represents a permission set with its selection state
// Used when RBAC has a schema configured
type PermissionSetItem struct {
	Key         string
	Label       string
	Description string
	Checked     bool
	Partial     bool              // True if only some permissions in the set are checked
	Permissions []*PermissionItem // Child permissions in this set
}

// ResourcePermissionGroup represents permissions grouped by resource
// Each resource can have multiple permission sets
type ResourcePermissionGroup struct {
	Resource       string
	PermissionSets []*PermissionSetItem
}
