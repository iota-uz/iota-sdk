package viewmodels

type Group struct {
	ID          string
	Name        string
	Description string
	Roles       []*Role
	Users       []*User
	CreatedAt   string
	UpdatedAt   string
}

func (g *Group) RolesVerbose() string {
	if len(g.Roles) == 0 {
		return ""
	}
	roles := ""
	for _, role := range g.Roles {
		roles += role.Name + ", "
	}
	return roles[:len(roles)-2]
}

func (g *Group) UsersCount() int {
	return len(g.Users)
}
