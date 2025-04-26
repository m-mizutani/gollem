package gollam

import (
	"context"

	"github.com/mark3labs/mcp-go/mcp"
)

func NewLocalMCPClient(path string) *MCPClient {
	client := &MCPClient{
		path: path,
	}
	return client
}

func (x *MCPClient) Start(ctx context.Context) error {
	return x.start(ctx)
}

func (x *MCPClient) ListTools(ctx context.Context) ([]mcp.Tool, error) {
	return x.listTools(ctx)
}

func (x *MCPClient) CallTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	return x.callTool(ctx, name, args)
}

func (x *MCPClient) Close(ctx context.Context) error {
	return x.close()
}

var (
	InputSchemaToParameter = inputSchemaToParameter
)
