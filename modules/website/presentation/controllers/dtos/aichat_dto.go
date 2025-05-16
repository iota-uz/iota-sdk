package dtos

type ChatMessage struct {
	Message string `json:"message"`
	Phone   string `json:"phone,omitempty"`
}

type ChatResponse struct {
	ThreadID string `json:"thread_id"`
}

type ThreadMessage struct {
	Role      string `json:"role"`
	Message   string `json:"message"`
	Timestamp string `json:"timestamp"`
}

type ThreadMessagesResponse struct {
	Messages []ThreadMessage `json:"messages"`
}

// AIConfigRequest represents a request to create or update an AI chat configuration
type AIConfigRequest struct {
	ModelName    string  `json:"ModelName"`
	ModelType    string  `json:"ModelType"`
	SystemPrompt string  `json:"SystemPrompt"`
	Temperature  float32 `json:"Temperature"`
	MaxTokens    int     `json:"MaxTokens"`
}

// AIConfigResponse represents the response for an AI chat configuration
type AIConfigResponse struct {
	ID           string  `json:"id"`
	ModelName    string  `json:"model_name"`
	ModelType    string  `json:"model_type"`
	SystemPrompt string  `json:"system_prompt"`
	Temperature  float32 `json:"temperature"`
	MaxTokens    int     `json:"max_tokens"`
	CreatedAt    string  `json:"created_at"`
	UpdatedAt    string  `json:"updated_at"`
}
