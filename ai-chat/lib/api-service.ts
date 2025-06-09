import { ApiError } from './errors';

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
  private locale = 'ru';

  setApiEndpoint(endpoint: string) {
    this.apiEndpoint = endpoint;
  }

  setLocale(locale: string) {
    this.locale = locale;
  }

  private getHeaders(): HeadersInit {
    return {
      'Content-Type': 'application/json',
      'Accept-Language': this.locale,
    };
  }

  async createThread(data: CreateThreadRequest): Promise<ThreadResponse> {
    const response = await fetch(`${this.apiEndpoint}/messages`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      // Create ApiError with the response data
      throw new ApiError(
        errorData?.message || `API error: ${response.status}`,
        errorData?.code || 'UNKNOWN_ERROR',
        errorData || { status: response.status }
      );
    }

    return await response.json();
  }

  // Get all messages for a thread
  async getMessages(threadId: string): Promise<MessagesResponse> {
    const response = await fetch(`${this.apiEndpoint}/messages/${threadId}`, {
      method: 'GET',
      headers: this.getHeaders(),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      // Create ApiError with the response data
      throw new ApiError(
        errorData?.message || `API error: ${response.status}`,
        errorData?.code || 'UNKNOWN_ERROR',
        errorData || { status: response.status }
      );
    }

    return await response.json();
  }

  // Add a new message to an existing thread
  async addMessage(threadId: string, data: AddMessageRequest): Promise<ThreadResponse> {
    const response = await fetch(`${this.apiEndpoint}/messages/${threadId}`, {
      method: 'POST',
      headers: this.getHeaders(),
      body: JSON.stringify(data),
    });

    if (!response.ok) {
      const errorData = await response.json().catch(() => null);
      // Create ApiError with the response data
      throw new ApiError(
        errorData?.message || `API error: ${response.status}`,
        errorData?.code || 'UNKNOWN_ERROR',
        errorData || { status: response.status }
      );
    }

    return await response.json();
  }
}

// Export a singleton instance
export const chatApi = new ChatApiService();
