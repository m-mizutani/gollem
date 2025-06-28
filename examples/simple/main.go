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

func main() {
	ctx := context.Background()

	// Create OpenAI client
	client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create MCP client with local server (with custom client info)
	mcpLocal, err := mcp.NewStdio(ctx, "./mcp-server", []string{"arg1", "arg2"},
		mcp.WithStdioClientInfo("gollem-simple-example", "1.0.0"))
	if err != nil {
		panic(err)
	}
	defer mcpLocal.Close()

	// Create gollem agent with MCP tools
	agent := gollem.New(client,
		gollem.WithToolSets(mcpLocal),
		gollem.WithSystemPrompt("You are a helpful assistant with access to MCP tools."),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
	)

	fmt.Println("🚀 Simple Gollem Agent with MCP Tools")
	fmt.Println("💡 Enter your message below:")
	fmt.Print("> ")

	scanner := bufio.NewScanner(os.Stdin)
	if scanner.Scan() {
		input := scanner.Text()

		// Execute with automatic session management
		if err := agent.Execute(ctx, input); err != nil {
			fmt.Printf("❌ Error: %v\n", err)
		} else {
			fmt.Println("✅ Task completed!")
		}
	}
}
