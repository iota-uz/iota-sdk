package viewmodels

type Upload struct {
	ID       string
	URL      string
	Path     string
	Hash     string
	Slug     string
	Size     string
	Mimetype string
}

type User struct {
	ID        string
	FirstName string
	LastName  string
}

func (u *User) Title() string {
	if u.FirstName == "" && u.LastName == "" {
		return "User #" + u.ID
	}
	return u.FirstName + " " + u.LastName
}
