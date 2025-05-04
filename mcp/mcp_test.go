package mcp_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/gollem/mcp"
	"github.com/m-mizutani/gt"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
)

func TestMCPLocalDryRun(t *testing.T) {
	mcpExecPath, ok := os.LookupEnv("TEST_MCP_EXEC_PATH")
	if !ok {
		t.Skip("TEST_MCP_EXEC_PATH is not set")
	}

	client := mcp.NewLocalMCPClient(mcpExecPath)

	err := client.Start(context.Background())
	gt.NoError(t, err)

	tools, err := client.ListTools(context.Background())
	gt.NoError(t, err)
	gt.A(t, tools).Longer(0)

	parameter, err := mcp.InputSchemaToParameter(tools[0].InputSchema)
	gt.NoError(t, err)
	t.Log("parameter:", parameter)

	tool := tools[0]

	t.Log("tool:", tool)

	callTool, err := client.CallTool(context.Background(), tool.Name, map[string]any{
		"length": 10,
	})
	gt.NoError(t, err)

	t.Log("callTool:", callTool)
}

func TestMCPContentToMap(t *testing.T) {
	t.Run("when content is empty", func(t *testing.T) {
		result := mcp.MCPContentToMap([]mcpgo.Content{})
		gt.Nil(t, result)
	})

	t.Run("when text content is JSON", func(t *testing.T) {
		content := mcpgo.TextContent{Text: `{"key": "value"}`}
		result := mcp.MCPContentToMap([]mcpgo.Content{content})
		gt.Equal(t, map[string]any{"key": "value"}, result)
	})

	t.Run("when text content is not JSON", func(t *testing.T) {
		content := mcpgo.TextContent{Text: "plain text"}
		result := mcp.MCPContentToMap([]mcpgo.Content{content})
		gt.Equal(t, map[string]any{"result": "plain text"}, result)
	})

	t.Run("when multiple contents exist", func(t *testing.T) {
		contents := []mcpgo.Content{
			mcpgo.TextContent{Text: "first"},
			mcpgo.TextContent{Text: "second"},
		}
		result := mcp.MCPContentToMap(contents)
		gt.Equal(t, map[string]any{
			"content_1": "first",
			"content_2": "second",
		}, result)
	})
}
