# Getting Started with gollem

gollem is a Go framework for building applications with Large Language Models (LLMs). This guide will help you get started with the framework.

## Installation

Install gollem using Go modules:

```bash
go get github.com/m-mizutani/gollem
```

## Basic Usage

Here's a simple example of how to use gollem with OpenAI's OpenAI model:

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/m-mizutani/gollem"
    "github.com/m-mizutani/gollem/llm/openai"
    "github.com/m-mizutani/gollem/mcp"
)

func main() {
    // Create OpenAI client
    client, err := OpenAI.New(context.Background(), os.Getenv("OPENAI_API_KEY"))
    if err != nil {
        panic(err)
    }

    // Create MCP client
    mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8080")
    if err != nil {
        panic(err)
    }
    defer mcpClient.Close()

    // Create gollem instance
    s := gollem.New(client,
        gollem.WithToolSets(mcpClient),
        gollem.WithMessageHook(func(ctx context.Context, msg string) error {
            fmt.Println(msg)
            return nil
        }),
    )

    // Send a message to the LLM
    if err := s.Prompt(context.Background(), "Hello, how are you?"); err != nil {
        panic(err)
    }
}
```

This code uses the OpenAI OpenAI model to receive a message from the user and send it to the LLM. Here, we are not specifying a Tool or MCP server, so the LLM is expected to return only a message.

For information on how to integrate with Tools and MCP servers, please refer to [tools](tools.md) and [mcp](mcp.md) documents.

## Supported LLM Providers

gollem supports multiple LLM providers:

- Gemini
- Anthropic (Claude)
- OpenAI (OpenAI)

Each provider has its own client implementation in the `llm` package. See the respective documentation for configuration options.

## Key Concepts

1. **LLM Client**: The interface to communicate with LLM providers
2. **Tools**: Custom functions that LLMs can use to perform actions
3. **MCP Server**: External tool integration through Model Context Protocol
4. **Natural Language Interface**: Interact with your application using natural language

## Error Handling

gollem provides robust error handling capabilities to help you build reliable applications:

### Error Types
- **LLM Errors**: Errors from the LLM provider (e.g., rate limits, invalid requests)
- **Tool Execution Errors**: Errors during tool execution
- **MCP Server Errors**: Errors from MCP server communication

### Best Practices
1. **Graceful Degradation**: Implement fallback mechanisms when LLM or tools fail
2. **Retry Strategies**: Use exponential backoff for transient errors
3. **Error Logging**: Log errors with appropriate context for debugging
4. **User Feedback**: Provide clear error messages to end users

Example of error handling:
```go
newHistory, err := s.Prompt(ctx, userInput, history)
if err != nil {
    // Handle errors
    log.Printf("Error: %v", err)
    return nil, fmt.Errorf("failed to process request: %w", err)
}
```

## Context Management

gollem provides a history-based context management system to maintain conversation state:

### History Object
The `History` object maintains the conversation context, including:
- Previous messages
- Tool execution results
- System prompts
- Context metadata

### Best Practices
1. **Memory Management**: Clear old history when it exceeds size limits
2. **Context Persistence**: Save important context for future sessions
3. **Context Pruning**: Remove irrelevant information to maintain focus

Example of history management:
```go
// Initialize history
var history *gollem.History

// Process user input with history
newHistory, err := s.Prompt(ctx, userInput, history)
if err != nil {
    return nil, err
}

// Update history
history = newHistory
```

## Next Steps

- Learn how to create and use [custom tools](tools.md)
- Explore [MCP server integration](mcp.md)
- Check out [practical examples](examples.md)
