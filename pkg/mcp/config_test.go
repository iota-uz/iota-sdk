package mcp

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestConfigureMCPServer(t *testing.T) {
	server := ConfigureMCPServer()
	assert.NotNil(t, server, "MCP server should not be nil")
}

func TestCreateSSEServer(t *testing.T) {
	mcp := ConfigureMCPServer()
	sseServer := CreateSSEServer(mcp)
	assert.NotNil(t, sseServer, "SSE server should not be nil")
}