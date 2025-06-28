package main

import (
	"context"
	"fmt"
	"log"
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

	// Create MCP client (HTTP) with custom client info
	// StreamableHTTP is now implemented with the official SDK
	httpClient, err := mcp.NewStreamableHTTP(ctx, "http://localhost:8080",
		mcp.WithStreamableHTTPClientInfo("gollem-mcp-http-client", "1.0.0"))
	if err != nil {
		fmt.Printf("⚠️  Could not connect to HTTP MCP server: %v\n", err)
		httpClient = nil
	}
	if httpClient != nil {
		defer httpClient.Close()
	}

	// Create MCP client (SSE) with custom client info
	// SSE is also implemented with the official SDK
	sseClient, err := mcp.NewSSE(ctx, "http://localhost:8081",
		mcp.WithSSEClientInfo("gollem-mcp-sse-client", "1.0.0"))
	if err != nil {
		fmt.Printf("⚠️  Could not connect to SSE MCP server: %v\n", err)
		sseClient = nil
	}
	if sseClient != nil {
		defer sseClient.Close()
	}

	// Create MCP client (Stdio) with custom client info
	stdioClient, err := mcp.NewStdio(ctx, "./mcp-server", []string{},
		mcp.WithEnvVars([]string{"MCP_ENV=test"}),
		mcp.WithStdioClientInfo("gollem-mcp-stdio-client", "1.0.0"))
	if err != nil {
		log.Fatalf("Failed to create Stdio client: %v", err)
	}
	defer stdioClient.Close()

	// Create gollem agent with MCP tools
	var toolSets []gollem.ToolSet
	if httpClient != nil {
		toolSets = append(toolSets, httpClient)
	}
	if sseClient != nil {
		toolSets = append(toolSets, sseClient)
	}
	toolSets = append(toolSets, stdioClient)

	agent := gollem.New(client,
		gollem.WithToolSets(toolSets...),
		gollem.WithSystemPrompt("You are a helpful assistant with access to various MCP tools for file operations and other tasks."),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
		gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
			fmt.Printf("⚡ Using MCP tool: %s\n", tool.Name)
			return nil
		}),
	)

	fmt.Println("🔧 MCP Integration Example")
	fmt.Println("💡 This agent has access to MCP tools from multiple servers")

	// Execute task with MCP tools
	task := "Hello, I want to use MCP tools. Please show me what tools are available and help me with file operations."
	fmt.Printf("📝 Task: %s\n\n", task)

	if err := agent.Execute(ctx, task); err != nil {
		log.Fatalf("❌ Error executing task: %v", err)
	}

	fmt.Println("\n✅ MCP integration example completed!")
}
