// Types for API requests and responses
export interface CreateThreadRequest {
  message: string
  phone: string
}

export interface ThreadResponse {
  thread_id: string
}

export interface Message {
  role: 'user' | 'assistant'
  message: string
  timestamp: string // ISO format: "2006-01-02T15:04:05Z07:00"
}

export interface MessagesResponse {
  messages: Message[]
}

export interface AddMessageRequest {
  message: string
}

// API service for chat functionality
class ChatApiService {
  private apiEndpoint = '';

  setApiEndpoint(endpoint: string) {
    this.apiEndpoint = endpoint;
  }

  async createThread(data: CreateThreadRequest): Promise<ThreadResponse> {
    const response = await fetch(`${this.apiEndpoint}/messages`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`);
    }

    return await response.json();
  }

  // Get all messages for a thread
  async getMessages(threadId: string): Promise<MessagesResponse> {
    const response = await fetch(`${this.apiEndpoint}/messages/${threadId}`, {
      method: 'GET',
      headers: {
        'Content-Type': 'application/json',
      },
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`);
    }

    return await response.json();
  }

  // Add a new message to an existing thread
  async addMessage(threadId: string, data: AddMessageRequest): Promise<ThreadResponse> {
    const response = await fetch(`${this.apiEndpoint}/messages/${threadId}`, {
      method: 'POST',
      headers: {
        'Content-Type': 'application/json',
      },
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`);
    }

    return await response.json();
  }
}

// Export a singleton instance
export const chatApi = new ChatApiService();
