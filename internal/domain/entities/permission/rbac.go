package permission

type Rbac struct {
	permissions []Permission
}

func NewRbac() *Rbac {
	return &Rbac{
		permissions: Permissions,
	}
}

func (r *Rbac) Register(permissions ...Permission) {
	r.permissions = append(r.permissions, permissions...)
}

func (r *Rbac) Permissions() []Permission {
	return r.permissions
}
