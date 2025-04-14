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
		firstWord := []rune(words[0])
		if len(firstWord) > 0 {
			return strings.ToUpper(string(firstWord[0]))
		}
		return ""
	}

	firstWord := []rune(words[0])
	lastWord := []rune(words[len(words)-1])

	initials := ""
	if len(firstWord) > 0 {
		initials += strings.ToUpper(string(firstWord[0]))
	}
	if len(lastWord) > 0 {
		initials += strings.ToUpper(string(lastWord[0]))
	}

	return initials
}
