package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gpt"
	"github.com/m-mizutani/gollem/mcp"
)

type MyTool struct{}

func (t *MyTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "my_tool",
		Description: "Returns a greeting",
		Parameters: map[string]*gollem.Parameter{
			"name": {
				Type:        gollem.TypeString,
				Description: "Name of the person to greet",
			},
		},
	}
}

func (t *MyTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name is required")
	}
	return map[string]any{"message": fmt.Sprintf("Hello, %s!", name)}, nil
}
func main() {
	ctx := context.Background()

	// Create GPT client
	client, err := gpt.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create MCP client with local server
	mcpLocal, err := mcp.NewStdio(ctx, "./mcp-server", []string{}, mcp.WithEnvVars([]string{"MCP_ENV=test"}))
	if err != nil {
		panic(err)
	}
	defer mcpLocal.Close()

	// Create MCP client with remote server
	mcpRemote, err := mcp.NewSSE(ctx, "http://localhost:8080")
	if err != nil {
		panic(err)
	}
	defer mcpRemote.Close()

	// Create gollem instance
	agent := gollem.New(client,
		// Not only MCP servers,
		gollem.WithToolSets(mcpLocal, mcpRemote),
		// But also you can use your own built-in tools
		gollem.WithTools(&MyTool{}),
		// You can customize the callback function for each message and tool call.
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
	)

	var history *gollem.History
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()

		newHistory, err := agent.Prompt(ctx, scanner.Text(), gollem.WithHistory(history))
		if err != nil {
			panic(err)
		}
		history = newHistory
	}
}
