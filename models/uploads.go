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
