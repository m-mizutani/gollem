# gollem Documentation

gollem is a Go framework for building applications with Large Language Models (LLMs). This documentation provides comprehensive guides and examples to help you get started and make the most of gollem.

## Documentation Index

- [Getting Started](getting-started.md) - Quick start guide and basic usage
- [Tools](tools.md) - Creating and using custom tools
- [MCP Server Integration](mcp.md) - Integrating with Model Context Protocol servers
- [History](history.md) - Managing conversation history and context
- [Examples](examples.md) - Practical examples and use cases

## Key Features

- **Multiple LLM Support**: Works with various LLM providers including OpenAI, Claude, and Gemini
- **Automatic Session Management**: Built-in conversation history management with the `Execute` method
- **Custom Tools**: Create and integrate your own tools for LLMs to use
- **MCP Integration**: Connect with external tools and resources through Model Context Protocol
- **Middleware System**: Monitor and control agent behavior with powerful middleware functions
- **Streaming Support**: Real-time response streaming for better user experience
- **Error Handling**: Robust error handling with retry mechanisms and graceful degradation

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "os"

    "github.com/m-mizutani/gollem"
    "github.com/m-mizutani/gollem/llm/openai"
)

func main() {
    ctx := context.Background()

    // Create OpenAI client
    client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
    if err != nil {
        panic(err)
    }

    // Create agent with automatic session management
    agent := gollem.New(client,
        gollem.WithSystemPrompt("You are a helpful assistant."),
        gollem.WithContentBlockMiddleware(func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
            return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
                resp, err := next(ctx, req)
                if err == nil && len(resp.Texts) > 0 {
                    for _, text := range resp.Texts {
                        fmt.Printf("🤖 %s\n", text)
                    }
                }
                return resp, err
            }
        }),
    )

    // Execute with automatic history management
    err = agent.Execute(ctx, "Hello, how can you help me?")
    if err != nil {
        panic(err)
    }

    // Continue conversation - history is managed automatically
    err = agent.Execute(ctx, "Tell me about Go programming best practices")
    if err != nil {
        panic(err)
    }
}
```

## Architecture Overview

gollem provides two main approaches for LLM interaction:

### Agent Approach (Recommended)
- **Execute Method**: Automatic session and history management
- **Conversational**: Perfect for building chatbots and interactive applications
- **Stateful**: Maintains conversation context automatically

### Direct Session Approach
- **Manual Control**: Direct access to LLM sessions
- **Stateless**: Suitable for one-off queries
- **Flexible**: Full control over session lifecycle

## Advanced Features

### Middleware System
Monitor and control every aspect of agent behavior:
- **ContentBlockMiddleware**: Process LLM responses in real-time
- **ContentStreamMiddleware**: Handle streaming responses
- **ToolMiddleware**: Monitor and control tool execution with pre/post processing

### Tool Integration
- **Custom Tools**: Implement your own tools with the `Tool` interface
- **Tool Sets**: Group related tools together
- **MCP Integration**: Connect to external MCP servers
- **Validation**: Built-in parameter validation and type checking

### Error Handling
- **Retry Logic**: Automatic retry for transient failures
- **Loop Limits**: Prevent infinite conversation loops
- **Graceful Degradation**: Continue operation when non-critical tools fail
- **Context Cancellation**: Proper timeout and cancellation support



## Support and Community

- **GitHub**: [github.com/m-mizutani/gollem](https://github.com/m-mizutani/gollem)
- **Issues**: Report bugs and request features on GitHub
- **Documentation**: This documentation is continuously updated with new features and examples


