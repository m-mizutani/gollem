# 🤖 gollam

<p align="center">
  <img src="./doc/images/logo.png" height="128" />
</p>

`gollam` is a Go framework for building applications with Large Language Models (LLMs). It provides an elegant and intuitive interface to seamlessly integrate LLMs into your applications while extending their capabilities through powerful tools and actions.

## Overview

gollam empowers you to:
- Seamlessly integrate with leading LLM providers (Gemini, Anthropic, OpenAI)
- Create and leverage custom built-in tools that enhance LLM capabilities
- Connect with MCP (Model Context Protocol) servers for advanced external tool integration
- Develop sophisticated agentic applications that intelligently execute actions based on natural language inputs

## Supported LLMs

- [x] Gemini (choose model from the [document](https://ai.google.dev/gemini-api/docs/models?hl=ja))
- [x] Anthropic (choose model from the [document](https://docs.anthropic.com/en/docs/about-claude/models/all-models))
- [x] OpenAI (choose model from the [document](https://platform.openai.com/docs/models))

## Features

### Tools and Actions

- Built-in Tools: Pre-defined tools for common operations
- MCP Server Integration
  - [x] Tool: Define and expose custom tools
  - [ ] Resource: Manage external resources
  - [ ] Prompt: Customize LLM prompts

## Quick Start

### Install

```bash
go get github.com/m-mizutani/gollam
```

### Example
Here's a simple example that demonstrates how to create a custom tool and use it with an LLM:

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollam"
	"github.com/m-mizutani/gollam/llm/gpt"
)

// HelloTool is a simple tool that returns a greeting
type HelloTool struct{}

func (t *HelloTool) Spec() *gollam.ToolSpec {
	return &gollam.ToolSpec{
		Name:        "hello",
		Description: "Returns a greeting",
		Parameters: map[string]*gollam.Parameter{
			"name": {
				Name:        "name",
				Type:        gollam.TypeString,
				Description: "Name of the person to greet",
				Required:    true,
			},
		},
	}
}

func (t *HelloTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{
		"message": fmt.Sprintf("Hello, %s!", args["name"]),
	}, nil
}

func main() {
	// Create GPT client
	client, err := gpt.New(context.Background(), os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create gollam instance with the custom tool
	s := gollam.New(client,
		// Register the custom tool
		gollam.WithTools(&HelloTool{}),

		// Register MCP server
		gollam.WithMCPonSSE("http://localhost:8080"),

		// Optional: Print the message from the LLM
		gollam.WithMsgCallback(func(ctx context.Context, msg string) {
			fmt.Println(msg)
		}),
	)

	// Send a message to the LLM
	if err := s.Order(context.Background(), "Hello, my name is Taro."); err != nil {
		panic(err)
	}
}
```

## Documentation

For detailed documentation and examples, please visit our [documentation site](https://github.com/m-mizutani/gollam/tree/main/doc).

## License

Apache 2.0 License. See [LICENSE](LICENSE) for details.