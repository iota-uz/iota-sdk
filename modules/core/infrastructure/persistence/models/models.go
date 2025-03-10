package models

import (
	"database/sql"
	"time"
)

type Upload struct {
	ID        uint
	Hash      string
	Path      string
	Name      string
	Size      int
	Mimetype  string
	Type      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Currency struct {
	Code      string
	Name      string
	Symbol    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Company struct {
	ID        uint
	Name      string
	About     string
	Address   string
	Phone     string
	LogoID    *uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Logo      Upload
}

type Permission struct {
	ID          string
	Name        string
	Resource    string
	Action      string
	Modifier    string
	Description sql.NullString
}

type RolePermission struct {
	RoleID       uint
	PermissionID uint
}

type Role struct {
	ID          uint
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type User struct {
	ID         uint
	FirstName  string
	LastName   string
	MiddleName sql.NullString
	Email      string
	Password   sql.NullString
	AvatarID   sql.NullInt32
	LastLogin  sql.NullTime
	LastIP     sql.NullString
	UILanguage string
	LastAction sql.NullTime
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type UserRole struct {
	UserID    uint
	RoleID    uint
	CreatedAt time.Time
}

type UploadedImage struct {
	ID        uint
	UploadID  uint
	Type      string
	Size      float64
	Width     int
	Height    int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Session struct {
	Token     string
	UserID    uint
	ExpiresAt time.Time
	IP        string
	UserAgent string
	CreatedAt time.Time
}

type AuthenticationLog struct {
	ID        uint
	UserID    uint
	IP        string
	UserAgent string
	CreatedAt time.Time
}

type Tab struct {
	ID       uint
	Href     string
	Position uint
	UserID   uint
}

type Passport struct {
	ID                  string
	FirstName           sql.NullString
	LastName            sql.NullString
	MiddleName          sql.NullString
	Gender              sql.NullString
	BirthDate           sql.NullTime
	BirthPlace          sql.NullString
	Nationality         sql.NullString
	PassportType        sql.NullString
	PassportNumber      sql.NullString
	Series              sql.NullString
	IssuingCountry      sql.NullString
	IssuedAt            sql.NullTime
	IssuedBy            sql.NullString
	ExpiresAt           sql.NullTime
	MachineReadableZone sql.NullString
	BiometricData       []byte // JSONB data stored as bytes
	SignatureImage      []byte // Binary data
	Remarks             sql.NullString
	CreatedAt           time.Time
	UpdatedAt           time.Time
}

type Group struct {
	ID          string
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type GroupUser struct {
	GroupID   string
	UserID    uint
	CreatedAt time.Time
}

type GroupRole struct {
	GroupID   string
	RoleID    uint
	CreatedAt time.Time
}
