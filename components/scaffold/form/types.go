package form

// --- Form Configuration ---

// FormConfig holds the configuration for a dynamic form
type FormConfig struct {
	Title       string
	SaveURL     string
	DeleteURL   string
	SubmitLabel string
	Fields      []Field
}

// NewFormConfig creates a new FormConfig with page title, form action URL, and submit button label
func NewFormConfig(title, saveURL, deleteURL, submitLabel string, fields ...Field) *FormConfig {
	return &FormConfig{
		Title:       title,
		SaveURL:     saveURL,
		DeleteURL:   deleteURL,
		SubmitLabel: submitLabel,
		Fields:      fields,
	}
}

// Add appends one or more Field implementations to the form and returns the config
func (cfg *FormConfig) Add(fields ...Field) *FormConfig {
	cfg.Fields = append(cfg.Fields, fields...)
	return cfg
}

// WithMethod sets the HTTP method for the form
func (cfg *FormConfig) WithMethod(method string) *FormConfig {
	// TODO: Implement method setting
	return cfg
}
