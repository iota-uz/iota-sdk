package form

// --- Form Configuration ---

// FormConfig holds the configuration for a dynamic form
type FormConfig struct {
	Title       string
	Action      string
	SubmitLabel string
	Fields      []Field
}

// NewFormConfig creates a new FormConfig with page title, form action URL, and submit button label
func NewFormConfig(title, action, submitLabel string, fields ...Field) *FormConfig {
	return &FormConfig{
		Title:       title,
		Action:      action,
		SubmitLabel: submitLabel,
		Fields:      fields,
	}
}

// AddFields appends one or more Field implementations to the form and returns the config
func (cfg *FormConfig) Add(fields ...Field) *FormConfig {
	cfg.Fields = append(cfg.Fields, fields...)
	return cfg
}
