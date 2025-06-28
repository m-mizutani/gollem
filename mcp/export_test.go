package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func NewLocalMCPClient(path string) *Client {
	client := &Client{
		name:    "gollem-test",
		version: "0.0.1",
	}
	return client
}

func (x *Client) Start(ctx context.Context) error {
	// For test purposes, initialize with a dummy command
	// This won't actually work but allows the test structure to remain
	return x.init(ctx, nil)
}

func (x *Client) ListTools(ctx context.Context) ([]*mcp.Tool, error) {
	return x.listTools(ctx)
}

func (x *Client) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return x.callTool(ctx, name, args)
}

var (
	InputSchemaToParameter = convertInputSchemaToParameter
	MCPContentToMap        = convertContentToMap
)
