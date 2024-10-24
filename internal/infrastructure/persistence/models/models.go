package models

import (
	"time"
)

type Upload struct {
	ID        uint
	Name      string
	Path      string
	Mimetype  string
	Size      float64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Currency struct {
	Code      string `gorm:"primary_key"`
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
	MiddleName       string
	Email            string
	Phone            string
	Salary           float64
	SalaryCurrencyID *uint
	HourlyRate       float64
	Coefficient      float64
	AvatarID         *uint
	CreatedAt        time.Time
	UpdatedAt        time.Time
	SalaryCurrency   Currency
	Position         Position
	Avatar           Upload
}

type EmployeePosition struct {
	EmployeeID uint
	PositionID uint
}

type EmployeeMeta struct {
	EmployeeID        uint
	PrimaryLanguage   string
	SecondaryLanguage string
	TIN               string
	GeneralInfo       string
	YTProfileID       string
	BirthDate         time.Time
	JoinDate          time.Time
	LeaveDate         time.Time
	UpdatedAt         time.Time
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

type Role struct {
	ID          uint
	Name        string
	Description string
	Permissions []Permission `gorm:"many2many:role_permissions;"`
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type User struct {
	ID         uint
	FirstName  string
	LastName   string
	MiddleName *string
	Email      string
	Password   *string
	AvatarID   *uint
	LastLogin  *time.Time
	LastIP     *string
	LastAction *time.Time
	EmployeeID *uint
	CreatedAt  time.Time
	UpdatedAt  time.Time
	Roles      []Role `gorm:"many2many:user_roles;"`
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

type ExpenseCategory struct {
	ID               uint
	Name             string
	Description      *string
	Amount           float64
	AmountCurrencyID string
	AmountCurrency   Currency `gorm:"foreignKey:AmountCurrencyID;references:Code"`
	CreatedAt        time.Time
	UpdatedAt        time.Time
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

type MoneyAccount struct {
	ID                uint
	Name              string
	AccountNumber     string
	Description       string
	Balance           float64
	BalanceCurrencyID string
	Currency          *Currency `gorm:"foreignKey:BalanceCurrencyID;references:Code"`
	CreatedAt         time.Time
	UpdatedAt         time.Time
}

type Transaction struct {
	ID                   uint
	Amount               float64
	OriginAccountID      *uint
	DestinationAccountID *uint
	TransactionDate      time.Time
	AccountingPeriod     time.Time
	TransactionType      string
	Comment              string
	CreatedAt            time.Time
}

type Expense struct {
	ID            uint
	TransactionID uint
	CategoryID    uint
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Transaction   *Transaction     `gorm:"foreignKey:TransactionID;references:ID"`
	Category      *ExpenseCategory `gorm:"foreignKey:CategoryID;references:ID"`
}

type Payment struct {
	ID             uint
	StageID        uint
	TransactionID  uint
	CounterpartyID uint
	Transaction    *Transaction `gorm:"foreignKey:TransactionID;references:ID"`
	CreatedAt      time.Time
	UpdatedAt      time.Time
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

type Like struct {
	ID        uint
	ArticleID uint
	UserID    uint
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

type Permission struct {
	ID       uint
	Name     string
	Resource string
	Action   string
	Modifier string
}

type RolePermission struct {
	RoleID       uint
	PermissionID uint
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

type Vacancy struct {
	ID        uint
	URL       string
	Title     string
	Body      string
	Hidden    bool
	CreatedAt time.Time
	UpdatedAt time.Time
}

type SalaryRange struct {
	MinSalary           float64
	MaxSalary           float64
	MinSalaryCurrencyID *uint
	MaxSalaryCurrencyID *uint
	VacancyID           uint
	MinSalaryCurrency   Currency
	MaxSalaryCurrency   Currency
	Vacancy             Vacancy
}

type Applicant struct {
	ID                 uint
	FirstName          string
	LastName           string
	MiddleName         string
	PrimaryLanguage    string
	SecondaryLanguage  string
	Email              string
	Phone              string
	ExperienceInMonths int
	VacancyID          uint
	CreatedAt          time.Time
	Vacancy            Vacancy
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

type ApplicantSkill struct {
	ApplicantID uint
	SkillID     uint
}

type ApplicantComment struct {
	ID          uint
	ApplicantID uint
	UserID      uint
	Content     string
	CreatedAt   time.Time
	Applicant   Applicant
	User        User
}

type Application struct {
	ID          uint
	ApplicantID uint
	VacancyID   uint
	CreatedAt   time.Time
	Applicant   Applicant
	Vacancy     Vacancy
}

type InterviewQuestion struct {
	ID          uint
	Title       string
	Description string
	Type        string
	Language    string
	Difficulty  string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type Interview struct {
	ID            uint
	ApplicationID uint
	InterviewerID uint
	Date          time.Time
	CreatedAt     time.Time
	Application   Application
	Interviewer   User
}

type InterviewRating struct {
	ID            uint
	InterviewID   uint
	InterviewerID uint
	QuestionID    uint
	Rating        int
	Comment       string
	CreatedAt     time.Time
	Interview     Interview
	Interviewer   User
	Question      InterviewQuestion
}

type ContactFormSubmission struct {
	ID        uint
	Name      string
	Email     string
	Phone     string
	Company   string
	Message   string
	CreatedAt time.Time
}

type BlogPost struct {
	ID        uint
	Title     string
	Content   string
	AuthorID  *uint
	PictureID *uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Author    User
	Picture   Upload
}

type BlogPostTag struct {
	ID        uint
	Name      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type BlogPostTagRelation struct {
	PostID uint
	TagID  uint
}

type BlogComment struct {
	ID        uint
	PostID    uint
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Post      BlogPost
}

type BlogLike struct {
	ID        uint
	PostID    uint
	CreatedAt time.Time
	UpdatedAt time.Time
	Post      BlogPost
}

type WebsitePage struct {
	ID        uint
	Path      string
	SEOTitle  string
	SEODesc   string
	SEOKeys   string
	SEOH1     string
	SEOH2     string
	SEOImg    string
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WebsitePageView struct {
	ID        uint
	PageID    uint
	UserAgent string
	IP        string
	CreatedAt time.Time
	Page      WebsitePage
}

type WarehouseUnit struct {
	ID          int
	Name        string
	Description *string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type InventoryCheck struct {
	ID        int64
	Status    string
	CreatedAt time.Time
}

type InventoryCheckResult struct {
	ID               int64
	InventoryCheckID int64
	PositionID       int64
	ExpectedQuantity int
	ActualQuantity   int
	Difference       int
	CreatedAt        time.Time
}

type WarehouseOrder struct {
	ID        int64
	Type      string
	Status    string
	CreatedAt time.Time
}

type OrderItem struct {
	OrderID   int64
	ProductID int64
	CreatedAt time.Time
}

type WarehousePosition struct {
	ID        int64
	Title     string
	Barcode   string
	UnitID    int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type WarehouseProduct struct {
	ID         int64
	PositionID int64
	Rfid       string
	Status     string
	CreatedAt  time.Time
	UpdatedAt  time.Time
}
