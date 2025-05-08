// This is a server-side API route that acts as a proxy to avoid CORS issues
import { NextResponse } from "next/server"

// Base URL for the external API
const API_BASE_URL = "https://add7-90-156-197-67.ngrok-free.app/website/ai-chat"

export async function POST(request: Request) {
  try {
    const { path, method, body } = await request.json()

    // Construct the full URL
    const url = `${API_BASE_URL}${path}`

    console.log(`Proxying ${method || "POST"} request to: ${url}`)

    // Make the request to the external API
    const response = await fetch(url, {
      method: method || "POST",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
      body: body ? JSON.stringify(body) : undefined,
    })

    // Check if the response is ok
    if (!response.ok) {
      console.error(`API responded with status: ${response.status}`)
      const text = await response.text()
      console.error(`Response body: ${text}`)
      return NextResponse.json(
        {
          error: `API responded with status: ${response.status}`,
          details: text,
        },
        { status: response.status },
      )
    }

    // Try to parse the response as JSON, but handle text responses too
    let data
    const contentType = response.headers.get("content-type")
    if (contentType && contentType.includes("application/json")) {
      data = await response.json()
    } else {
      const text = await response.text()
      try {
        // Try to parse as JSON anyway in case the content-type is wrong
        data = JSON.parse(text)
      } catch (e) {
        // If it's not JSON, return the text as a message
        data = { message: text }
      }
    }

    // Return the response
    return NextResponse.json(data)
  } catch (error) {
    console.error("Proxy error:", error)
    return NextResponse.json(
      {
        error: "Failed to proxy request",
        details: error instanceof Error ? error.message : String(error),
      },
      { status: 500 },
    )
  }
}

export async function GET(request: Request) {
  try {
    // Get the URL from the query parameters
    const { searchParams } = new URL(request.url)
    const path = searchParams.get("path")

    if (!path) {
      return NextResponse.json({ error: "Path parameter is required" }, { status: 400 })
    }

    // Construct the full URL
    const url = `${API_BASE_URL}${path}`

    console.log(`Proxying GET request to: ${url}`)

    // Make the request to the external API
    const response = await fetch(url, {
      method: "GET",
      headers: {
        "Content-Type": "application/json",
        Accept: "application/json",
      },
    })

    // Check if the response is ok
    if (!response.ok) {
      console.error(`API responded with status: ${response.status}`)
      const text = await response.text()
      console.error(`Response body: ${text}`)
      return NextResponse.json(
        {
          error: `API responded with status: ${response.status}`,
          details: text,
        },
        { status: response.status },
      )
    }

    // Try to parse the response as JSON, but handle text responses too
    let data
    const contentType = response.headers.get("content-type")
    if (contentType && contentType.includes("application/json")) {
      data = await response.json()
    } else {
      const text = await response.text()
      try {
        // Try to parse as JSON anyway in case the content-type is wrong
        data = JSON.parse(text)
      } catch (e) {
        // If it's not JSON, return the text as a message
        data = { message: text }
      }
    }

    // Return the response
    return NextResponse.json(data)
  } catch (error) {
    console.error("Proxy error:", error)
    return NextResponse.json(
      {
        error: "Failed to proxy request",
        details: error instanceof Error ? error.message : String(error),
      },
      { status: 500 },
    )
  }
}
