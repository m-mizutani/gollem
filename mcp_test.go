package gollam_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gt"
)

func TestMCPLocalDryRun(t *testing.T) {
	mcpExecPath, ok := os.LookupEnv("TEST_MCP_EXEC_PATH")
	if !ok {
		t.Skip("TEST_MCP_EXEC_PATH is not set")
	}

	client := gollam.NewLocalMCPClient(mcpExecPath)

	err := client.Start(context.Background())
	gt.NoError(t, err)

	tools, err := client.ListTools(context.Background())
	gt.NoError(t, err)
	gt.A(t, tools).Longer(0)

	parameter, err := gollam.InputSchemaToParameter(tools[0].InputSchema)
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
