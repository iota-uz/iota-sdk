package upload

import (
	"fmt"
	"net/url"
	"slices"
	"strings"
	"time"

	"github.com/gabriel-vasile/mimetype"
	"github.com/google/uuid"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
	"github.com/iota-uz/iota-sdk/pkg/configuration"
)

// ---- Value Objects ----

type UploadType string

func (t UploadType) String() string {
	return string(t)
}

const (
	UploadTypeImage    UploadType = "image"
	UploadTypeDocument UploadType = "document"
)

// ---- Interfaces ----

type Size interface {
	String() string
	Bytes() int
	Kilobytes() int
	Megabytes() int
	Gigabytes() int
}

type Upload interface {
	ID() uint
	TenantID() uuid.UUID
	Type() UploadType
	Hash() string
	Path() string
	Name() string
	Size() Size
	IsImage() bool
	PreviewURL() string
	URL() *url.URL
	Mimetype() *mimetype.MIME
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

// ---- Upload Implementation ----

func New(
	hash, path, name string,
	size int,
	mimetype *mimetype.MIME,
) Upload {
	var t UploadType
	if mimetype != nil && strings.HasPrefix(mimetype.String(), "image") {
		t = UploadTypeImage
	} else {
		t = UploadTypeDocument
	}
	return &upload{
		id:        0,
		tenantID:  uuid.Nil,
		hash:      hash,
		path:      path,
		name:      name,
		size:      NewSize(size),
		mimetype:  mimetype,
		_type:     t,
		createdAt: time.Now(),
		updatedAt: time.Now(),
	}
}

func NewWithID(
	id uint,
	tenantID uuid.UUID,
	hash, path, name string,
	size int,
	mimetype *mimetype.MIME,
	_type UploadType,
	createdAt, updatedAt time.Time,
) Upload {
	return &upload{
		id:        id,
		tenantID:  tenantID,
		hash:      hash,
		path:      path,
		name:      name,
		size:      NewSize(size),
		mimetype:  mimetype,
		_type:     _type,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

type upload struct {
	id        uint
	tenantID  uuid.UUID
	hash      string
	path      string
	name      string
	size      Size
	_type     UploadType
	mimetype  *mimetype.MIME
	createdAt time.Time
	updatedAt time.Time
}

func (u *upload) ID() uint {
	return u.id
}

func (u *upload) TenantID() uuid.UUID {
	return u.tenantID
}

func (u *upload) Type() UploadType {
	return u._type
}

func (u *upload) Hash() string {
	return u.hash
}

func (u *upload) Path() string {
	return u.path
}

func (u *upload) Name() string {
	return u.name
}

func (u *upload) Size() Size {
	return u.size
}

func (u *upload) URL() *url.URL {
	conf := configuration.Use()
	return &url.URL{
		Scheme: conf.Scheme(),
		Host:   conf.Domain,
		Path:   u.path,
	}
}

func (u *upload) PreviewURL() string {
	// TODO: this is gotta be implemented better
	if u.mimetype != nil && slices.Contains([]string{".xls", ".xlsx", ".csv"}, u.mimetype.Extension()) {
		return "/assets/" + assets.HashFS.HashName("images/excel-logo.svg")
	}

	return "/" + u.path
}

func (u *upload) IsImage() bool {
	return u.mimetype != nil && strings.HasPrefix(u.mimetype.String(), "image")
}

func (u *upload) Mimetype() *mimetype.MIME {
	return u.mimetype
}

func (u *upload) CreatedAt() time.Time {
	return u.createdAt
}

func (u *upload) UpdatedAt() time.Time {
	return u.updatedAt
}

// ---- Size Implementation ----

func NewSize(size int) Size {
	return &sizeImpl{size: size}
}

type sizeImpl struct {
	size int
}

func (s *sizeImpl) String() string {
	const unit = 1024
	if s.size < unit {
		return fmt.Sprintf("%d B", s.size)
	}
	div, exp := int64(unit), 0
	suffixes := []string{"KB", "MB", "GB", "TB", "PB", "EB"}
	for n := s.size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %s", float64(s.size)/float64(div), suffixes[exp])
}

func (s *sizeImpl) Bytes() int {
	return s.size
}

func (s *sizeImpl) Kilobytes() int {
	return s.size / 1024
}

func (s *sizeImpl) Megabytes() int {
	return s.size / 1024 / 1024
}

func (s *sizeImpl) Gigabytes() int {
	return s.size / 1024 / 1024 / 1024
}
