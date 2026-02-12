// Package branding provides UI customization types for BiChat applications.
// Consumers can inject branding configuration to customize the chat interface
// without modifying the core BiChat module.
package branding

// Config holds UI branding configuration for the chat interface.
// All fields are optional - defaults are used when not specified.
type Config struct {
	// AppName is the application name displayed in the UI.
	// Example: "Business Assistant", "Data Analyst"
	AppName string `json:"appName,omitempty"`

	// LogoURL is the URL to the application logo.
	// Can be absolute or relative to the application base path.
	LogoURL string `json:"logoUrl,omitempty"`

	// Welcome configures the welcome screen shown for new chats.
	Welcome WelcomeConfig `json:"welcome,omitempty"`

	// Theme configures visual styling (optional, CSS variables preferred).
	Theme *ThemeConfig `json:"theme,omitempty"`
}

// WelcomeConfig configures the welcome screen content.
type WelcomeConfig struct {
	// Title is the main heading on the welcome screen.
	// Falls back to translation key "welcome.title" if empty.
	Title string `json:"title,omitempty"`

	// Description is the subtitle/description text.
	// Falls back to translation key "welcome.description" if empty.
	Description string `json:"description,omitempty"`

	// ExamplePrompts are suggested prompts shown to users.
	// If empty, defaults from translations are used.
	ExamplePrompts []ExamplePrompt `json:"examplePrompts,omitempty"`
}

// ExamplePrompt represents a clickable example prompt on the welcome screen.
type ExamplePrompt struct {
	// Category groups the prompt (e.g., "Analysis", "Reports").
	Category string `json:"category"`

	// Text is the actual prompt text that gets sent when clicked.
	Text string `json:"text"`

	// Icon is an optional icon identifier (e.g., "chart-bar", "file-text").
	// Uses Phosphor Icons naming convention.
	Icon string `json:"icon,omitempty"`
}

// ThemeConfig provides optional theme overrides.
// Prefer using CSS variables (--bichat-primary, etc.) for theming.
type ThemeConfig struct {
	// PrimaryColor is the main accent color (hex format).
	PrimaryColor string `json:"primaryColor,omitempty"`

	// BackgroundColor is the chat background color.
	BackgroundColor string `json:"backgroundColor,omitempty"`

	// TextColor is the primary text color.
	TextColor string `json:"textColor,omitempty"`
}

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() Config {
	return Config{
		AppName: "BiChat",
		Welcome: WelcomeConfig{
			ExamplePrompts: []ExamplePrompt{
				{Category: "Analysis", Text: "Show me sales trends for the last quarter", Icon: "chart-bar"},
				{Category: "Reports", Text: "Generate a summary of recent activity", Icon: "file-text"},
				{Category: "Insights", Text: "What are the top performing items?", Icon: "lightbulb"},
			},
		},
	}
}

// Merge combines this config with another, with other taking precedence.
// Empty/zero values in other are ignored.
func (c Config) Merge(other Config) Config {
	result := c

	if other.AppName != "" {
		result.AppName = other.AppName
	}
	if other.LogoURL != "" {
		result.LogoURL = other.LogoURL
	}
	if other.Welcome.Title != "" {
		result.Welcome.Title = other.Welcome.Title
	}
	if other.Welcome.Description != "" {
		result.Welcome.Description = other.Welcome.Description
	}
	if len(other.Welcome.ExamplePrompts) > 0 {
		result.Welcome.ExamplePrompts = other.Welcome.ExamplePrompts
	}
	if other.Theme != nil {
		result.Theme = other.Theme
	}

	return result
}
