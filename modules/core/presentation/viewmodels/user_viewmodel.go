package viewmodels

import (
	"unicode"
)

type User struct {
	ID         string
	FirstName  string
	LastName   string
	MiddleName string
	Email      string
	UILanguage string
	LastAction string
	CreatedAt  string
	UpdatedAt  string
	AvatarID   string
	Roles      []*Role
	Avatar     *Upload
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
	firstName := []rune(u.FirstName)
	lastName := []rune(u.LastName)
	initials := []rune{firstName[0], lastName[0]}
	for i, r := range initials {
		initials[i] = unicode.ToUpper(r)
	}
	return string(initials)
}
