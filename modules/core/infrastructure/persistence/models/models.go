package models

import (
	"database/sql"
	"time"
)

type Upload struct {
	ID        uint
	Hash      string
	Path      string
	Size      int
	Mimetype  string
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

type Position struct {
	ID          uint
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type DifficultyLevel struct {
	ID          uint
	Name        string
	Description string
	Coefficient float64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type TaskType struct {
	ID          uint
	Icon        string
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Setting struct {
	ID            uint
	DefaultRisks  float64
	DefaultMargin float64
	IncomeTaxRate float64
	SocialTaxRate float64
	UpdatedAt     time.Time
}

type Employee struct {
	ID               uint
	FirstName        string
	LastName         string
	MiddleName       sql.NullString
	Email            string
	Phone            sql.NullString
	Salary           float64
	SalaryCurrencyID sql.NullString
	HourlyRate       float64
	Coefficient      float64
	AvatarID         *uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
}

type EmployeeMeta struct {
	PrimaryLanguage   sql.NullString
	SecondaryLanguage sql.NullString
	Tin               sql.NullString
	Pin               sql.NullString
	Notes             sql.NullString
	BirthDate         sql.NullTime
	HireDate          sql.NullTime
	ResignationDate   sql.NullTime
}

type EmployeePosition struct {
	EmployeeID uint
	PositionID uint
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
	EmployeeID sql.NullInt32
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type TelegramSession struct {
	UserID    uint
	Session   string
	CreatedAt time.Time
}

type UserRole struct {
	UserID    uint
	RoleID    uint
	CreatedAt time.Time
}

type Prompt struct {
	ID          string
	Title       string
	Description string
	Prompt      string
	CreatedAt   time.Time
}

type EmployeeContact struct {
	ID         uint
	EmployeeID uint
	Type       string
	Value      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Customer struct {
	ID         uint
	FirstName  string
	LastName   string
	MiddleName string
	Email      string
	Phone      string
	CompanyID  *uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Company    Company
}

type CustomerContact struct {
	ID         uint
	CustomerID uint
	Type       string
	Value      string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type Project struct {
	ID          uint
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ProjectStage struct {
	ID        uint
	ProjectID uint
	Name      string
	Margin    float64
	Risks     float64
	StartDate time.Time
	EndDate   time.Time
	CreatedAt time.Time
	UpdatedAt time.Time
}

type ProjectTask struct {
	ID          uint
	Title       string
	Description string
	StageID     uint
	TypeID      uint
	LevelID     uint
	ParentID    *uint
	CreatedAt   time.Time
	UpdatedAt   time.Time
	Stage       ProjectStage
	Type        TaskType
	Level       DifficultyLevel
	Parent      *ProjectTask
}

type Estimate struct {
	ID         uint
	TaskID     uint
	EmployeeID uint
	Hours      float64
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Task       ProjectTask
	Employee   Employee
}

type Folder struct {
	ID        uint
	Name      string
	IconID    *uint
	ParentID  *uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Icon      Upload
	Parent    *Folder
}

type Article struct {
	ID         uint
	Title      string
	Content    string
	TitleEmoji string
	AuthorID   *uint
	PictureID  *uint
	FolderID   *uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Author     User
	Picture    Upload
	Folder     Folder
}

type Comment struct {
	ID        uint
	ArticleID uint
	UserID    uint
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Article   Article
	User      User
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
	Upload    Upload
}

type ActionLog struct {
	ID        uint
	Method    string
	Path      string
	UserID    *uint
	After     string
	Before    string
	UserAgent string
	IP        string
	CreatedAt time.Time
}

type Dialogue struct {
	ID        uint
	UserID    uint
	Label     string
	Messages  string
	CreatedAt time.Time
	UpdatedAt time.Time
	User      User
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

type Skill struct {
	ID          uint
	Name        string
	Description string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type EmployeeSkill struct {
	EmployeeID uint
	SkillID    uint
}

type Tab struct {
	ID       uint
	Href     string
	Position uint
	UserID   uint
}
