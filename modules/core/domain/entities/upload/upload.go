package upload

import (
	"fmt"
	"net/url"
	"slices"
	"time"

	"github.com/gabriel-vasile/mimetype"

	"github.com/iota-uz/iota-sdk/modules/core/presentation/assets"
)

// ----

// ---- Interfaces ----

type Size interface {
	Bytes() int
	Kilobytes() int
	Megabytes() int
	Gigabytes() int
}

type Upload interface {
	ID() uint
	Hash() string
	Path() string
	Size() Size
	URL() url.URL
	Mimetype() mimetype.MIME
	CreatedAt() time.Time
	UpdatedAt() time.Time
}

// ---- Upload Implementation ----

func New() Upload {
	return &upload{}
}

func NewWithID(
	id uint,
	hash, path string,
	size int,
	mimetype mimetype.MIME,
	createdAt, updatedAt time.Time,
) Upload {
	return &upload{
		id:        id,
		hash:      hash,
		path:      path,
		size:      NewSize(size),
		mimetype:  mimetype,
		createdAt: createdAt,
		updatedAt: updatedAt,
	}
}

type upload struct {
	id        uint
	hash      string
	path      string
	size      Size
	mimetype  mimetype.MIME
	createdAt time.Time
	updatedAt time.Time
}

func (u *upload) ID() uint {
	return u.id
}

func (u *upload) Hash() string {
	return u.hash
}

func (u *upload) Path() string {
	return u.path
}

func (u *upload) Size() Size {
	return u.size
}

//
//	Scheme      string
//	Opaque      string    // encoded opaque data
//	User        *Userinfo // username and password information
//	Host        string    // host or host:port (see Hostname and Port methods)
//	Path        string    // path (relative paths may omit leading slash)
//	RawPath     string    // encoded path hint (see EscapedPath method)
//	OmitHost    bool      // do not emit empty host (authority)
//	ForceQuery  bool      // append a query ('?') even if RawQuery is empty
//	RawQuery    string    // encoded query values, without '?'
//	Fragment    string    // fragment for references, without '#'
//	RawFragment string    // encoded fragment hint (see EscapedFragment method)

func (u *upload) URL() url.URL {
	return url.URL{
		Scheme: "https",
		Path:   u.path,
	}
}

func (u *upload) PreviewURL() string {
	// TODO: this is gotta be implemented better
	if slices.Contains([]string{".xls", ".xlsx", ".csv"}, u.mimetype.Extension()) {
		return "/assets/" + assets.HashFS.HashName("images/excel-logo.svg")
	}

	return "/" + u.path
}

func (u *upload) Mimetype() mimetype.MIME {
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
