package mcp

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewLocalMCPClient(path string) *Client {
	client := &Client{
		path: path,
	}
	return client
}

func (x *Client) Start(ctx context.Context) error {
	return x.init(ctx)
}

func (x *Client) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	return x.listTools(ctx)
}

func (x *Client) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return x.callTool(ctx, name, args)
}

var (
	InputSchemaToParameter = inputSchemaToParameter
)
