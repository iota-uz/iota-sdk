// Package mcp is the shared engine behind every AI tool surface of an IOTA SDK
// application: the external MCP gateway and the built-in chat call the same
// registry through the same Engine, so a tool is registered once and runs with
// one schema, one authorization path and one failure contract everywhere.
//
// The package holds contracts only and stays free of transport dependencies;
// the MCP protocol adapter lives in the separate mcpgateway Go module.
package mcp
