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
