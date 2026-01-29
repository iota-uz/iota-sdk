package models

import (
	"bytes"
	"database/sql"
	"database/sql/driver"
	"fmt"
	"time"
)

type Tenant struct {
	ID            string
	Name          string
	Domain        sql.NullString
	Phone         sql.NullString
	Email         sql.NullString
	IsActive      bool
	LogoID        sql.NullInt32
	LogoCompactID sql.NullInt32
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

type Upload struct {
	ID        uint
	TenantID  string
	Hash      string
	Path      string
	Slug      string
	Name      string
	Size      int
	Mimetype  string
	Type      string
	Source    string
	GeoPoint  *Point
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
	TenantID  string
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
	Type        string
	TenantID    string
	Name        string
	Description sql.NullString
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type User struct {
	ID                uint
	TenantID          string // UUID stored as string
	Type              string
	FirstName         string
	LastName          string
	MiddleName        sql.NullString
	Email             string
	Phone             sql.NullString
	Password          sql.NullString
	AvatarID          sql.NullInt32
	LastLogin         sql.NullTime
	LastIP            sql.NullString
	UILanguage        string
	LastAction        sql.NullTime
	CreatedAt         time.Time
	UpdatedAt         time.Time
	IsBlocked         bool
	BlockReason       sql.NullString
	BlockedAt         sql.NullTime
	BlockedBy         sql.NullInt64
	BlockedByTenantID sql.NullString
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
	TenantID  string // UUID stored as string
	UserID    uint
	ExpiresAt time.Time
	IP        string
	UserAgent string
	CreatedAt time.Time
}

type AuthenticationLog struct {
	ID        uint
	TenantID  string // UUID stored as string
	UserID    uint
	IP        string
	UserAgent string
	CreatedAt time.Time
}

type Passport struct {
	ID                  string
	TenantID            string
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
	Type        string
	TenantID    string
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

type Point struct {
	X float64
	Y float64
}

func (p *Point) Value() (driver.Value, error) {
	buf := new(bytes.Buffer)
	fmt.Fprintf(buf, "(%f, %f)", p.X, p.Y)
	return buf.Bytes(), nil
}

func (p *Point) Scan(val any) error {
	var s string
	switch v := val.(type) {
	case []byte:
		s = string(v)
	case string:
		s = v
	default:
		return fmt.Errorf("unsupported type: %s", fmt.Sprintf("%T", v))
	}

	_, err := fmt.Sscanf(s, "(%f,%f)", &p.X, &p.Y)
	if err != nil {
		return err
	}
	return nil
}

func (p *Point) String() string {
	return fmt.Sprintf("(%v, %v)", p.X, p.Y)
}
