package mcp

import (
	"github.com/iota-uz/iota-sdk/pkg/mcp/handlers"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// ConfigureMCPServer sets up the MCP server with all handlers
func ConfigureMCPServer() *server.MCPServer {
	// Create MCP server
	s := server.NewMCPServer(
		"IOTA SDK MCP Server",
		"1.0.0",
		server.WithResourceCapabilities(true, true),
		server.WithLogging(),
	)

	// Add get_definition tool
	definitionTool := mcp.NewTool("get_definition",
		mcp.WithDescription("Get the definition of a Go type, function, or interface"),
		mcp.WithString("path",
			mcp.Required(),
			mcp.Description("The import path followed by the symbol name, e.g., github.com/iota-uz/iota-sdk/pkg/repo.Insert"),
		),
	)

	// Add get_definition handler
	s.AddTool(definitionTool, handlers.GetDefinition)

	return s
}

// CreateSSEServer creates a new SSE server for the MCP server
func CreateSSEServer(mcp *server.MCPServer) *server.SSEServer {
	return server.NewSSEServer(mcp,
		server.WithBaseURL("http://localhost"),
		server.WithBasePath("/mcp"),
		server.WithSSEEndpoint("/events"),
		server.WithMessageEndpoint("/message"),
	)
}