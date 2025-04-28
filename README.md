# 🤖 gollam

<p align="center">
  <img src="./doc/images/logo.png" height="128" />
</p>

`gollam` is a Go framework for building agentic applications with Large Language Models (LLMs). It helps you integrate LLMs into your applications and extend their features with tools and actions.

## Overview

gollam lets you:
- Connect with LLM providers (Gemini, Anthropic, OpenAI)
- Use built-in tools to enhance LLM features
- Connect with MCP servers for external tools
- Build applications that run actions based on text input

## Supported LLMs

- [x] Gemini (see [models](https://ai.google.dev/gemini-api/docs/models?hl=ja))
- [x] Anthropic (see [models](https://docs.anthropic.com/en/docs/about-claude/models/all-models))
- [x] OpenAI (see [models](https://platform.openai.com/docs/models))

## Features

### Tools and Actions

- Built-in Tools: Common operation tools
- MCP Server Integration
  - [x] Tool: Custom tools
  - [ ] Resource: External resources
  - [ ] Prompt: LLM prompts

## Quick Start

### Install

```bash
go get github.com/m-mizutani/gollam
```

### Example
Here's a simple example of creating a custom tool and using it with an LLM:

```go
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

	// Create gollam instance
	s := gollam.New(client,
		gollam.WithToolSets(mcpLocal, mcpRemote),
		gollam.WithTools(&MyTool{}),
		gollam.WithMsgCallback(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
	)

	var history *gollam.History
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()

		newHistory, err := s.Order(ctx, scanner.Text(), history)
		if err != nil {
			panic(err)
		}
		history = newHistory
	}
}
```

See the full example in [examples/basic](https://github.com/m-mizutani/gollam/tree/main/examples/basic).

## Documentation

For more details and examples, visit our [documentation site](https://github.com/m-mizutani/gollam/tree/main/doc).

## License

Apache 2.0 License. See [LICENSE](LICENSE) for details.