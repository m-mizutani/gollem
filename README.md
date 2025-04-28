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
		// Not only MCP servers,
		gollam.WithToolSets(mcpLocal, mcpRemote),
		// But also you can use your own built-in tools
		gollam.WithTools(&MyTool{}),
		// You can customize the callback function for each message and tool call.
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

Full example is available in the [examples/basic](https://github.com/m-mizutani/gollam/tree/main/examples/basic).

## Documentation

For detailed documentation and examples, please visit our [documentation site](https://github.com/m-mizutani/gollam/tree/main/doc).

## License

Apache 2.0 License. See [LICENSE](LICENSE) for details.