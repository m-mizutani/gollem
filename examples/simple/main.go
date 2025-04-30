package main

import (
	"bufio"
	"context"
	"fmt"
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

	// Create MCP client with local server
	mcpLocal, err := mcp.NewStdio(ctx, "./mcp-server", []string{"arg1", "arg2"})
	if err != nil {
		panic(err)
	}
	defer mcpLocal.Close()

	// Create gollam instance
	agent := gollam.New(client,
		gollam.WithToolSets(mcpLocal),
		gollam.WithMsgCallback(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
	)

	fmt.Print("> ")
	scanner := bufio.NewScanner(os.Stdin)
	scanner.Scan()

	if _, err = agent.Instruct(ctx, scanner.Text()); err != nil {
		panic(err)
	}
}
