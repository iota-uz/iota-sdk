package dtos

type ChatMessage struct {
	Message string `json:"message"`
	Contact string `json:"phone,omitempty"`
}

type ChatResponse struct {
	ThreadID string `json:"thread_id"`
}

type ThreadMessage struct {
	Role    string `json:"role"`
	Message string `json:"message"`
}

type ThreadMessagesResponse struct {
	Messages []ThreadMessage `json:"messages"`
}
