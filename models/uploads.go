package models

import "time"

type Uploads struct {
	Id         int64         `gql:"id" db:"id"`
	Name       string        `gql:"name" db:"name"`
	Path       string        `gql:"path" db:"path"`
	UploaderId JsonNullInt64 `gql:"uploader_id" db:"uploader_id"`
	Mimetype   string        `gql:"mimetype" db:"mimetype"`
	Size       float64       `gql:"size" db:"size"`
	CreatedAt  *time.Time    `gql:"created_at" db:"created_at"`
	UpdatedAt  *time.Time    `gql:"updated_at" db:"updated_at"`
}

func (u *Uploads) Pk() interface{} {
	return u.Id
}

func (u *Uploads) PkField() *Field {
	return &Field{Name: "id", Type: Integer}
}

func (u *Uploads) Table() string {
	return "uploads"
}
