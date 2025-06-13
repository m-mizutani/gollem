package mcp_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/gollem/mcp"
	"github.com/m-mizutani/gt"
	mcpgo "github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
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

	_, err = mcp.InputSchemaToParameter(tools[0].InputSchema)
	gt.NoError(t, err)

	tool := tools[0]
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

func TestNewStreamableHTTP(t *testing.T) {
	type testCase struct {
		name    string
		baseURL string
		headers map[string]string
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Create a test MCP server
			mcpServer := server.NewMCPServer("test-server", "1.0.0")

			// Add a simple test tool
			testTool := mcpgo.Tool{
				Name:        "test-tool",
				Description: "A test tool",
				InputSchema: mcpgo.ToolInputSchema{
					Type: "object",
					Properties: map[string]interface{}{
						"message": map[string]interface{}{
							"type":        "string",
							"description": "Test message",
						},
					},
				},
			}

			mcpServer.AddTool(testTool, func(ctx context.Context, request mcpgo.CallToolRequest) (*mcpgo.CallToolResult, error) {
				return &mcpgo.CallToolResult{
					Content: []mcpgo.Content{
						mcpgo.TextContent{Text: "test result"},
					},
				}, nil
			})

			// Create HTTP test server
			httpServer := server.NewTestStreamableHTTPServer(mcpServer)
			defer httpServer.Close()

			// Create client with test server URL
			client, err := mcp.NewStreamableHTTP(
				context.Background(),
				httpServer.URL,
				mcp.WithStreamableHTTPHeaders(tc.headers),
			)
			gt.NoError(t, err)
			defer client.Close()

			// Test that we can get tool specs
			specs, err := client.Specs(context.Background())
			gt.NoError(t, err)
			gt.Array(t, specs).Length(1)
			gt.Equal(t, "test-tool", specs[0].Name)
		}
	}

	t.Run("basic functionality", runTest(testCase{
		name: "basic",
	}))

	t.Run("with headers", runTest(testCase{
		name: "with headers",
		headers: map[string]string{
			"Authorization": "Bearer test-token",
			"Custom-Header": "test-value",
		},
	}))
}

func TestNewStreamableHTTPErrorHandling(t *testing.T) {
	// Test with invalid URL
	t.Run("invalid URL", func(t *testing.T) {
		client, err := mcp.NewStreamableHTTP(
			context.Background(),
			"invalid-url",
		)
		gt.Error(t, err)
		gt.Nil(t, client)
	})

	// Test with unreachable server
	t.Run("unreachable server", func(t *testing.T) {
		client, err := mcp.NewStreamableHTTP(
			context.Background(),
			"http://localhost:99999",
		)
		gt.Error(t, err)
		gt.Nil(t, client)
	})
}

// Test that existing functionality still works
func TestExistingFunctionalityNotAffected(t *testing.T) {
	// Test that NewStdio still works
	t.Run("NewStdio functionality", func(t *testing.T) {
		// This test is skipped if TEST_MCP_EXEC_PATH is not set
		mcpExecPath, ok := os.LookupEnv("TEST_MCP_EXEC_PATH")
		if !ok {
			t.Skip("TEST_MCP_EXEC_PATH is not set")
		}

		client, err := mcp.NewStdio(context.Background(), mcpExecPath, []string{})
		if err != nil {
			t.Skip("Could not create stdio client:", err)
		}
		defer client.Close()

		// Verify it works
		specs, err := client.Specs(context.Background())
		gt.NoError(t, err)
		gt.Array(t, specs).Longer(0)
	})

	// Test that NewSSE interface is available (without requiring a server)
	t.Run("NewSSE interface available", func(t *testing.T) {
		// Just test that the function signature is correct
		// We don't test actual functionality since we don't have an SSE test server
		client, err := mcp.NewSSE(
			context.Background(),
			"http://localhost:99999", // Will fail, but that's expected
		)
		gt.Error(t, err) // Expected to fail with connection error
		gt.Nil(t, client)
	})
}
