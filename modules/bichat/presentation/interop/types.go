package interop

// InitialContext contains all server-side context passed to the React frontend
type InitialContext struct {
	User   UserContext   `json:"user"`
	Tenant TenantContext `json:"tenant"`
	Locale LocaleContext `json:"locale"`
	Config AppConfig     `json:"config"`
}

// UserContext contains user information passed to frontend
type UserContext struct {
	ID          int64    `json:"id"`
	Email       string   `json:"email"`
	FirstName   string   `json:"firstName"`
	LastName    string   `json:"lastName"`
	Permissions []string `json:"permissions"`
}

// TenantContext contains tenant information
type TenantContext struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// LocaleContext contains locale and translation data
type LocaleContext struct {
	Language     string            `json:"language"`
	Translations map[string]string `json:"translations"`
}

// AppConfig contains application configuration endpoints
type AppConfig struct {
	GraphQLEndpoint string `json:"graphQLEndpoint"`
	StreamEndpoint  string `json:"streamEndpoint"`
}
