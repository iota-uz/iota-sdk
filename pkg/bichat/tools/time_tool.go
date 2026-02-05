package tools

import (
	"context"
	"fmt"
	"time"

	"github.com/iota-uz/iota-sdk/pkg/bichat/agents"
)

// GetCurrentTimeTool returns the current date and time in a specified timezone.
// This helps LLMs interpret relative date queries like 'last month', 'this quarter', 'YTD'.
type GetCurrentTimeTool struct{}

// NewGetCurrentTimeTool creates a new get current time tool.
func NewGetCurrentTimeTool() agents.Tool {
	return &GetCurrentTimeTool{}
}

// Name returns the tool name.
func (t *GetCurrentTimeTool) Name() string {
	return "get_current_time"
}

// Description returns the tool description for the LLM.
func (t *GetCurrentTimeTool) Description() string {
	return "Get the current date and time in the specified timezone. " +
		"Use this to interpret relative date queries like 'last month', 'this quarter', 'YTD'."
}

// Parameters returns the JSON Schema for tool parameters.
func (t *GetCurrentTimeTool) Parameters() map[string]any {
	return map[string]any{
		"type": "object",
		"properties": map[string]any{
			"timezone": map[string]any{
				"type":        "string",
				"description": "Timezone name (e.g., 'Asia/Tashkent', 'UTC'). Default: 'UTC'",
				"default":     "UTC",
			},
		},
	}
}

// timeToolInput represents the parsed input parameters.
type timeToolInput struct {
	Timezone string `json:"timezone,omitempty"`
}

// timeToolOutput represents the formatted output.
type timeToolOutput struct {
	Timezone    string `json:"timezone"`
	CurrentTime string `json:"current_time"`
	Date        string `json:"date"`
	Time        string `json:"time"`
	DayOfWeek   string `json:"day_of_week"`
	Year        int    `json:"year"`
	Month       string `json:"month"`
	MonthNumber int    `json:"month_number"`
	Day         int    `json:"day"`
	Hour        int    `json:"hour"`
	Minute      int    `json:"minute"`
	Second      int    `json:"second"`
	WeekOfYear  int    `json:"week_of_year"`
	Quarter     int    `json:"quarter"`
}

// Call executes the get current time operation.
func (t *GetCurrentTimeTool) Call(ctx context.Context, input string) (string, error) {
	// Parse input
	params, err := agents.ParseToolInput[timeToolInput](input)
	if err != nil {
		// Default to UTC if parsing fails
		params = timeToolInput{Timezone: "UTC"}
	}

	timezone := params.Timezone
	if timezone == "" {
		timezone = "UTC"
	}

	// Load timezone location
	loc, err := time.LoadLocation(timezone)
	if err != nil {
		return FormatToolError(
			ErrCodeInvalidRequest,
			fmt.Sprintf("invalid timezone: %v", err),
			"Use IANA timezone names (e.g., 'UTC', 'Asia/Tashkent', 'America/New_York')",
			"Common timezones: UTC, Europe/London, Asia/Tokyo",
		), nil
	}

	now := time.Now().In(loc)

	// Build response
	response := timeToolOutput{
		Timezone:    timezone,
		CurrentTime: now.Format(time.RFC3339),
		Date:        now.Format("2006-01-02"),
		Time:        now.Format("15:04:05"),
		DayOfWeek:   now.Weekday().String(),
		Year:        now.Year(),
		Month:       now.Month().String(),
		MonthNumber: int(now.Month()),
		Day:         now.Day(),
		Hour:        now.Hour(),
		Minute:      now.Minute(),
		Second:      now.Second(),
		WeekOfYear:  getWeekOfYear(now),
		Quarter:     getQuarter(now),
	}

	return agents.FormatToolOutput(response)
}

// getWeekOfYear returns the ISO week number.
func getWeekOfYear(t time.Time) int {
	_, week := t.ISOWeek()
	return week
}

// getQuarter returns the quarter number (1-4).
func getQuarter(t time.Time) int {
	month := int(t.Month())
	return (month-1)/3 + 1
}
