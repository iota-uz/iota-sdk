package viewmodels

type Currency struct {
	Code   string
	Name   string
	Symbol string
}

type Upload struct {
	ID        string
	Hash      string
	URL       string
	Mimetype  string
	Size      string
	CreatedAt string
	UpdatedAt string
}

type Role struct {
	ID          string
	Type        string
	Name        string
	Description string
	CreatedAt   string
	UpdatedAt   string
	CanUpdate   bool
	CanDelete   bool
}

type Tab struct {
	ID   string
	Href string
}
