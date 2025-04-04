// Code generated by github.com/99designs/gqlgen, DO NOT EDIT.

package model

import (
	"time"

	"github.com/iota-uz/iota-sdk/modules/core/domain/entities/upload"
)

type Mutation struct {
}

type PaginatedUsers struct {
	Data  []*User `json:"data"`
	Total int64   `json:"total"`
}

type Query struct {
}

type Session struct {
	Token     string    `json:"token"`
	UserID    int64     `json:"userId"`
	IP        string    `json:"ip"`
	UserAgent string    `json:"userAgent"`
	ExpiresAt time.Time `json:"expiresAt"`
	CreatedAt time.Time `json:"createdAt"`
}

type Subscription struct {
}

type Upload struct {
	ID       int64             `json:"id"`
	URL      string            `json:"url"`
	Hash     string            `json:"hash"`
	Path     string            `json:"path"`
	Name     string            `json:"name"`
	Mimetype string            `json:"mimetype"`
	Type     upload.UploadType `json:"type"`
	Size     int               `json:"size"`
}

type UploadFilter struct {
	MimeType       *string            `json:"mimeType,omitempty"`
	MimeTypePrefix *string            `json:"mimeTypePrefix,omitempty"`
	Type           *upload.UploadType `json:"type,omitempty"`
}

type User struct {
	ID         int64     `json:"id"`
	FirstName  string    `json:"firstName"`
	LastName   string    `json:"lastName"`
	Email      string    `json:"email"`
	UILanguage string    `json:"uiLanguage"`
	UpdatedAt  time.Time `json:"updatedAt"`
	CreatedAt  time.Time `json:"createdAt"`
}
