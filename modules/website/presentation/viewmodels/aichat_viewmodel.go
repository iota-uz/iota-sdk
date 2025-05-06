package viewmodels

// AIConfig represents the view model for the AI chat configuration
type AIConfig struct {
	ID           string
	ModelName    string
	ModelType    string
	SystemPrompt string
	Temperature  float32
	MaxTokens    int
	BaseURL      string
	CreatedAt    string
	UpdatedAt    string
	IsDefault    bool
}
