# 🤖 gollem [![Go Reference](https://pkg.go.dev/badge/github.com/m-mizutani/gollem.svg)](https://pkg.go.dev/github.com/m-mizutani/gollem) [![Test](https://github.com/m-mizutani/gollem/actions/workflows/test.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/test.yml) [![Lint](https://github.com/m-mizutani/gollem/actions/workflows/lint.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/lint.yml) [![Gosec](https://github.com/m-mizutani/gollem/actions/workflows/gosec.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/gosec.yml) [![Trivy](https://github.com/m-mizutani/gollem/actions/workflows/trivy.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/trivy.yml)

GO for Large LanguagE Model

<p align="center">
  <img src="./doc/images/logo.png" height="128" />
</p>


`gollem` provides:
- Common interface to query prompt to Large Language Model (LLM) services
- Framework for building agentic applications of LLMs with
  - Tools by MCP (Model Context Protocol) server and your built-in tools
  - Conversational memory with history

## Supported LLMs

- [x] Gemini (see [models](https://ai.google.dev/gemini-api/docs/models?hl=ja))
- [x] Anthropic (see [models](https://docs.anthropic.com/en/docs/about-claude/models/all-models))
- [x] OpenAI (see [models](https://platform.openai.com/docs/models))

## Quick Start

### Install

```bash
go get github.com/m-mizutani/gollem
```

### Example

#### Query to LLM

```go

```

#### Agentic application with MCP server

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
	mcpLocal, err := mcp.NewStdio(ctx, "/path/to/mcp-server", []string{}, mcp.WithEnvVars([]string{"MCP_ENV=test"}))
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

	// Create gollem instance
	agent := gollem.New(client,
		gollem.WithToolSets(mcpLocal, mcpRemote),
		gollem.WithTools(&MyTool{}),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
	)

	var history *gollem.History
	for {
		fmt.Print("> ")
		scanner := bufio.NewScanner(os.Stdin)
		scanner.Scan()

		newHistory, err := agent.Prompt(ctx, scanner.Text(), history)
		if err != nil {
			panic(err)
		}
		history = newHistory
	}
}
```

See the full example in [examples/basic](https://github.com/m-mizutani/gollem/tree/main/examples/basic), and more examples in [examples](https://github.com/m-mizutani/gollem/tree/main/examples).

## Documentation

For more details and examples, visit our [documentation site](https://github.com/m-mizutani/gollem/tree/main/doc).

## License

Apache 2.0 License. See [LICENSE](LICENSE) for details.