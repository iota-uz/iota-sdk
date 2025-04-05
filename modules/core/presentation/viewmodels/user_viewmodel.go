package viewmodels

import (
	"unicode"
)

type User struct {
	ID          string
	FirstName   string
	LastName    string
	MiddleName  string
	Email       string
	Phone       string
	UILanguage  string
	LastAction  string
	CreatedAt   string
	UpdatedAt   string
	AvatarID    string
	Roles       []*Role
	Groups      []*Group
	Permissions []*Permission
	Avatar      *Upload
}

func (u *User) FullName() string {
	return u.FirstName + " " + u.LastName + " " + u.MiddleName
}

func (u *User) RolesVerbose() string {
	if len(u.Roles) == 0 {
		return ""
	}
	roles := ""
	for _, role := range u.Roles {
		roles += role.Name + ", "
	}
	return roles[:len(roles)-2]
}

func (u *User) Initials() string {
	initials := ""
	if u.FirstName != "" {
		initials += string(unicode.ToUpper(rune(u.FirstName[0])))
	}
	if u.LastName != "" {
		initials += string(unicode.ToUpper(rune(u.LastName[0])))
	}
	if initials == "" {
		return "NA"
	}
	return initials
}
