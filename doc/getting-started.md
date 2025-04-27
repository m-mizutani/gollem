# Getting Started with gollam

gollam is a Go framework for building applications with Large Language Models (LLMs). This guide will help you get started with the framework.

## Installation

Install gollam using Go modules:

```bash
go get github.com/m-mizutani/gollam
```

## Basic Usage

Here's a simple example of how to use gollam with OpenAI's GPT model:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/m-mizutani/gollam"
    "github.com/m-mizutani/gollam/llm/gpt"
    "github.com/m-mizutani/gollam/mcp"
)

func main() {
    // Create GPT client
    client, err := gpt.New(context.Background(), os.Getenv("OPENAI_API_KEY"))
    if err != nil {
        panic(err)
    }

    // Create MCP client
    mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8080")
    if err != nil {
        panic(err)
    }
    defer mcpClient.Close()

    // Create gollam instance
    s := gollam.New(client,
        gollam.WithToolSets(mcpClient),
        gollam.WithMsgCallback(func(ctx context.Context, msg string) error {
            fmt.Println(msg)
            return nil
        }),
    )

    // Send a message to the LLM
    if err := s.Order(context.Background(), "Hello, how are you?"); err != nil {
        panic(err)
    }
}
```

This code uses the OpenAI GPT model to receive a message from the user and send it to the LLM. Here, we are not specifying a Tool or MCP server, so the LLM is expected to return only a message.

For information on how to integrate with Tools and MCP servers, please refer to [tools](tools.md) and [mcp](mcp.md) documents.

## Supported LLM Providers

gollam supports multiple LLM providers:

- Gemini
- Anthropic (Claude)
- OpenAI (GPT)

Each provider has its own client implementation in the `llm` package. See the respective documentation for configuration options.

## Key Concepts

1. **LLM Client**: The interface to communicate with LLM providers
2. **Tools**: Custom functions that LLMs can use to perform actions
3. **MCP Server**: External tool integration through Model Context Protocol
4. **Natural Language Interface**: Interact with your application using natural language

## Next Steps

- Learn how to create and use [custom tools](tools.md)
- Explore [MCP server integration](mcp.md)
- Check out [practical examples](examples.md)
