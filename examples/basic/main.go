package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
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
		Required: []string{"name"},
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

	// Create OpenAI client
	client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create MCP client with local server (with custom client info)
	mcpLocal, err := mcp.NewStdio(ctx, "./mcp-server", []string{},
		mcp.WithEnvVars([]string{"MCP_ENV=test"}),
		mcp.WithStdioClientInfo("gollem-basic-example", "1.0.0"))
	if err != nil {
		panic(err)
	}
	defer mcpLocal.Close()

	// Create MCP client with remote server (with custom client info)
	mcpRemote, err := mcp.NewSSE(ctx, "http://localhost:8080",
		mcp.WithSSEClientInfo("gollem-remote-client", "1.0.0"))
	if err != nil {
		panic(err)
	}
	defer mcpRemote.Close()

	// Create gollem agent with automatic session management
	agent := gollem.New(client,
		// Not only MCP servers,
		gollem.WithToolSets(mcpLocal, mcpRemote),
		// But also you can use your own built-in tools
		gollem.WithTools(&MyTool{}),
		// System prompt for better context
		gollem.WithSystemPrompt("You are a helpful assistant with access to various tools."),
		// You can customize the callback function for each message and tool call.
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
	)

	fmt.Println("🚀 Gollem Agent started! Type 'quit' to exit.")
	fmt.Println("💡 The agent automatically manages conversation history.")

	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}

		input := scanner.Text()
		if input == "quit" || input == "exit" {
			fmt.Println("👋 Goodbye!")
			break
		}

		// Execute with automatic session management
		// No need to manually handle history - it's managed automatically!
		if err := agent.Execute(ctx, input); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
			continue
		}

		// Optional: Show conversation statistics
		if history := agent.Session().History(); history != nil {
			fmt.Printf("📊 (Conversation has %d messages)\n", history.ToCount())
		}
	}
}
