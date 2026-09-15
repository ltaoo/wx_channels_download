// Package mcp provides a business-agnostic Model Context Protocol engine.
//
// NewServer builds a JSON-RPC server whose tools are plugged in with AddTool,
// so the engine owns the MCP surface over stdio and Streamable HTTP without
// depending on application code. It imports only the standard library.
package mcp
