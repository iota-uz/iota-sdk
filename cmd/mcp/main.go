package main

import (
	"fmt"
	
	"github.com/iota-uz/iota-sdk/pkg/mcp"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// Create and configure MCP server
	s := mcp.ConfigureMCPServer()

	// Start the stdio server
	if err := server.ServeStdio(s); err != nil {
		fmt.Printf("Server error: %v\n", err)
	}
}