package main

import (
	"context"
	"log"
	"os"

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/gpt"
	"github.com/m-mizutani/gollam/mcp"
)

func main() {
	ctx := context.Background()

	// Create GPT client
	client, err := gpt.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create MCP client (SSE)
	sseClient, err := mcp.NewSSE(ctx, "http://localhost:8080")
	if err != nil {
		log.Fatalf("Failed to create SSE client: %v", err)
	}
	defer sseClient.Close()

	// Create MCP client (Stdio)
	stdioClient, err := mcp.NewStdio(ctx, "./mcp-server", []string{}, mcp.WithEnvVars([]string{"MCP_ENV=test"}))
	if err != nil {
		log.Fatalf("Failed to create Stdio client: %v", err)
	}
	defer stdioClient.Close()

	// Create gollam instance with MCP tools
	g := gollam.New(client,
		gollam.WithToolSets(sseClient, stdioClient),
	)

	// Send a message
	if _, err := g.Instruct(ctx, "Hello, I want to use MCP tools."); err != nil {
		panic(err)
	}
}
