package viewmodels

import "strings"

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

// GetInitials returns the first letters of each word in the group name
func (g *Group) GetInitials() string {
	if g.Name == "" {
		return ""
	}
	
	words := strings.Fields(g.Name)
	if len(words) == 0 {
		return ""
	}
	
	if len(words) == 1 {
		return strings.ToUpper(string(words[0][0]))
	}
	
	return strings.ToUpper(string(words[0][0]) + string(words[len(words)-1][0]))
}
