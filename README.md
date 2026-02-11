# 🤖 gollem [![Go Reference](https://pkg.go.dev/badge/github.com/m-mizutani/gollem.svg)](https://pkg.go.dev/github.com/m-mizutani/gollem) [![Test](https://github.com/m-mizutani/gollem/actions/workflows/test.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/test.yml) [![Lint](https://github.com/m-mizutani/gollem/actions/workflows/lint.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/lint.yml) [![Gosec](https://github.com/m-mizutani/gollem/actions/workflows/gosec.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/gosec.yml) [![Trivy](https://github.com/m-mizutani/gollem/actions/workflows/trivy.yml/badge.svg)](https://github.com/m-mizutani/gollem/actions/workflows/trivy.yml)

GO for Large LanguagE Model (GOLLEM)

<p align="center">
  <img src="./doc/images/logo.png" height="128" />
</p>

`gollem` provides:
- **Common interface** to query prompt to Large Language Model (LLM) services
  - GenerateContent: Generate text content from prompt
  - GenerateEmbedding: Generate embedding vector from text (OpenAI and Gemini)
- **Framework for building agentic applications** of LLMs with
  - Tools by MCP (Model Context Protocol) server and your built-in tools
  - Automatic session management for continuous conversations
  - Portable conversational memory with history for stateless/distributed applications
  - Intelligent memory management with automatic history compaction
  - Middleware system for monitoring, logging, and controlling agent behavior

## Supported LLMs

- [x] **Gemini** (see [models](https://ai.google.dev/gemini-api/docs/models?hl=ja))
- [x] **Anthropic Claude** (see [models](https://docs.anthropic.com/en/docs/about-claude/models/all-models))
  - Direct access via Anthropic API
  - Via Google Vertex AI (see [LLM Provider Configuration](doc/llm.md#claude-vertex-ai))
- [x] **OpenAI** (see [models](https://platform.openai.com/docs/models))

## Install

```bash
go get github.com/m-mizutani/gollem
```

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

	// Create LLM client
	client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create session for one-time query
	session, err := client.NewSession(ctx)
	if err != nil {
		panic(err)
	}

	// Generate content
	result, err := session.GenerateContent(ctx, gollem.Text("Hello, how are you?"))
	if err != nil {
		panic(err)
	}

	fmt.Println(result.Texts)
}
```

## Features

### Agent Framework

Build conversational agents with automatic session management and tool integration. [Learn more →](doc/getting-started.md)

```go
agent := gollem.New(client,
	gollem.WithTools(&GreetingTool{}),
	gollem.WithSystemPrompt("You are a helpful assistant."),
)

// Session is managed automatically across calls
agent.Execute(ctx, "Hello!")
agent.Execute(ctx, "What did I just say?") // remembers context
```

### Tool Integration

Define custom tools for LLMs to call, or connect external tools via MCP. [Tools →](doc/tools.md) | [MCP →](doc/mcp.md)

```go
// Custom tool - implement Spec() and Run()
type SearchTool struct{}

func (t *SearchTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "search",
		Description: "Search the database",
		Parameters:  map[string]*gollem.Parameter{
			"query": {Type: gollem.TypeString, Description: "Search query"},
		},
	}
}

func (t *SearchTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"results": doSearch(args["query"].(string))}, nil
}

// MCP server - connect external tool servers
mcpClient, _ := mcp.NewStdio(ctx, "./mcp-server", []string{})
agent := gollem.New(client,
	gollem.WithTools(&SearchTool{}),
	gollem.WithToolSets(mcpClient),
)
```

### Multimodal Input

Send images and PDFs alongside text prompts. [Learn more →](doc/llm.md#pdf-input-support)

```go
img, _ := gollem.NewImage(imageBytes)
pdf, _ := gollem.NewPDFFromReader(file)

result, _ := session.GenerateContent(ctx, img, pdf, gollem.Text("Describe these."))
```

### Structured Output

Constrain LLM responses to a JSON Schema. [Learn more →](doc/schema.md)

```go
schema, _ := gollem.ToSchema(UserProfile{})
session, _ := client.NewSession(ctx,
	gollem.WithSessionContentType(gollem.ContentTypeJSON),
	gollem.WithSessionResponseSchema(schema),
)
resp, _ := session.GenerateContent(ctx, gollem.Text("Extract: John, 30, john@example.com"))
// resp.Texts[0] is valid JSON matching the schema
```

### Middleware

Monitor, log, and control agent behavior with composable middleware. [Learn more →](doc/middleware.md)

```go
agent := gollem.New(client,
	gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
		return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
			log.Printf("Tool called: %s", req.Tool.Name)
			return next(ctx, req)
		}
	}),
)
```

### Strategy Pattern

Swap execution strategies: simple, ReAct, or Plan & Execute. [Learn more →](doc/strategy.md)

```go
import "github.com/m-mizutani/gollem/strategy/planexec"

agent := gollem.New(client,
	gollem.WithStrategy(planexec.New(client)),
	gollem.WithTools(&SearchTool{}, &AnalysisTool{}),
)
```

### Tracing

Observe agent execution with pluggable backends (in-memory, OpenTelemetry). [Learn more →](doc/tracing.md)

```go
import "github.com/m-mizutani/gollem/trace"

rec := trace.New(trace.WithRepository(trace.NewFileRepository("./traces")))
agent := gollem.New(client, gollem.WithTrace(rec))
```

<p align="center">
  <img width="860" src="https://github.com/user-attachments/assets/6b9d77e0-d580-4c08-b7c8-3b2b6cd733eb" />
</p>

### History Management

Portable conversation history for stateless/distributed applications. [Learn more →](doc/history.md)

```go
// Export history for persistence
history := agent.Session().History()
data, _ := json.Marshal(history)

// Restore in another process
var restored gollem.History
json.Unmarshal(data, &restored)
agent := gollem.New(client, gollem.WithHistory(&restored))
```

## Examples

See the [examples](https://github.com/m-mizutani/gollem/tree/main/examples) directory for complete working examples:

- **[Simple](examples/simple)**: Minimal example for getting started
- **[Query](examples/query)**: Simple LLM query without conversation state
- **[Basic](examples/basic)**: Simple agent with custom tools
- **[Chat](examples/chat)**: Interactive chat application
- **[MCP](examples/mcp)**: Integration with MCP servers
- **[Tools](examples/tools)**: Custom tool development
- **[JSON Schema](examples/json_schema)**: Structured output with JSON Schema validation
- **[Embedding](examples/embedding)**: Text embedding generation
- **[Tracing](examples/tracing)**: Agent execution tracing with file persistence

## Documentation

- **[Getting Started Guide](doc/getting-started.md)**
- **[Tool Development](doc/tools.md)**
- **[MCP Integration](doc/mcp.md)**
- **[Structured Output with JSON Schema](doc/schema.md)**
- **[Middleware System](doc/middleware.md)**
- **[Strategy Pattern](doc/strategy.md)**
- **[Tracing](doc/tracing.md)**
- **[History Management](doc/history.md)**
- **[LLM Provider Configuration](doc/llm.md)**
- **[Debugging](doc/debugging.md)**
- **[API Reference](https://pkg.go.dev/github.com/m-mizutani/gollem)**

## License

Apache 2.0 License. See [LICENSE](LICENSE) for details.
