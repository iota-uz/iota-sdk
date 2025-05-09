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
    try {
      console.log('Creating thread with data:', data);

      const response = await fetch(`${this.apiEndpoint}/messages`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => null);
        console.error('Error response:', errorData);
        throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`);
      }

      return await response.json();
    } catch (error) {
      console.error('Error creating thread:', error);
      throw error;
    }
  }

  // Get all messages for a thread
  async getMessages(threadId: string): Promise<MessagesResponse> {
    try {
      console.log('Fetching messages for thread:', threadId);

      const response = await fetch(`${this.apiEndpoint}/messages/${threadId}`, {
        method: 'GET',
        headers: {
          'Content-Type': 'application/json',
        },
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => null);
        console.error('Error response:', errorData);
        throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`);
      }

      const result = await response.json();
      console.log('Messages received:', result);
      return result;
    } catch (error) {
      console.error('Error getting messages:', error);
      throw error;
    }
  }

  // Add a new message to an existing thread
  async addMessage(threadId: string, data: AddMessageRequest): Promise<ThreadResponse> {
    try {
      console.log('Adding message to thread:', threadId, data);

      const response = await fetch(`${this.apiEndpoint}/messages/${threadId}`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
        },
        body: JSON.stringify(data),
      });

      if (!response.ok) {
        const errorData = await response.json().catch(() => null);
        console.error('Error response:', errorData);
        throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`);
      }

      const result = await response.json();
      console.log('Message added:', result);
      return result;
    } catch (error) {
      console.error('Error adding message:', error);
      throw error;
    }
  }
}

// Export a singleton instance
export const chatApi = new ChatApiService();
