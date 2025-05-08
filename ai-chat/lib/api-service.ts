// Base URL for API requests - using our own proxy to avoid CORS issues
const API_PROXY = "/api/proxy"

// Types for API requests and responses
export interface CreateThreadRequest {
  message: string
  phone: string
}

export interface ThreadResponse {
  thread_id: string
}

export interface Message {
  role: "user" | "assistant"
  message: string
}

export interface MessagesResponse {
  messages: Message[]
}

export interface AddMessageRequest {
  message: string
}

// API service for chat functionality
export const chatApi = {
  // Create a new thread with initial message and phone number
  async createThread(data: CreateThreadRequest): Promise<ThreadResponse> {
    try {
      console.log("Creating thread with data:", data)

      const response = await fetch(API_PROXY, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          path: "/message",
          method: "POST",
          body: data,
        }),
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => null)
        console.error("Error response:", errorData)
        throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`)
      }

      const result = await response.json()
      console.log("Thread created:", result)
      return result
    } catch (error) {
      console.error("Error creating thread:", error)
      throw error
    }
  },

  // Get all messages for a thread
  async getMessages(threadId: string): Promise<MessagesResponse> {
    try {
      console.log("Fetching messages for thread:", threadId)

      const response = await fetch(`${API_PROXY}?path=/messages/${threadId}`, {
        method: "GET",
        headers: {
          "Content-Type": "application/json",
        },
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => null)
        console.error("Error response:", errorData)
        throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`)
      }

      const result = await response.json()
      console.log("Messages received:", result)
      return result
    } catch (error) {
      console.error("Error getting messages:", error)
      throw error
    }
  },

  // Add a new message to an existing thread
  async addMessage(threadId: string, data: AddMessageRequest): Promise<ThreadResponse> {
    try {
      console.log("Adding message to thread:", threadId, data)

      const response = await fetch(API_PROXY, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          path: `/messages/${threadId}`,
          method: "POST",
          body: data,
        }),
      })

      if (!response.ok) {
        const errorData = await response.json().catch(() => null)
        console.error("Error response:", errorData)
        throw new Error(errorData?.error || errorData?.details || `API error: ${response.status}`)
      }

      const result = await response.json()
      console.log("Message added:", result)
      return result
    } catch (error) {
      console.error("Error adding message:", error)
      throw error
    }
  },
}
