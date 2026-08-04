// Package viewmodels provides this package.
package viewmodels

type Currency struct {
	Code   string
	Name   string
	Symbol string
}

type Upload struct {
	ID        string
	Hash      string
	Slug      string // Filesystem-safe storage slug (defaults to Hash if no original name).
	Name      string // Original filename as uploaded by the user, e.g. "scan.pdf".
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
	UsersCount  int
	CreatedAt   string
	UpdatedAt   string
	CanUpdate   bool
	CanDelete   bool
}
