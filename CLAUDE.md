# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Commands

### Development and Testing
- `task` or `task mock` - Generate mock files for testing (uses moq)
- `go test ./...` - Run all tests
- `go test -v ./llm/openai/` - Run tests for specific package
- `go build ./...` - Build all packages
- `go mod tidy` - Clean up dependencies

### Test Execution
Tests may require API keys for integration testing:
- OpenAI: `OPENAI_API_KEY`
- Anthropic: `ANTHROPIC_API_KEY`  
- Gemini: `GEMINI_PROJECT_ID`, `GEMINI_LOCATION`

## Architecture

**gollem** is a Go framework providing unified LLM access and agentic application building capabilities.

### Core Components

**Agent** (`gollem.go`) - Central orchestrator managing conversation loops, tool execution, and session management. Entry point: `gollem.New(llmClient, options...)`

**LLM Clients** (`llm/`) - Provider-specific implementations (OpenAI, Claude, Gemini) that all implement the `LLMClient` interface with `NewSession()` and `GenerateEmbedding()` methods.

**Session Management** (`session.go`) - Handles conversation state and message processing for each LLM interaction.

**Tool System** (`tool.go`) - Framework for LLM tool calling:
- `Tool` interface: individual tools with `Spec()` and `Run()` methods
- `ToolSet` interface: collections of tools (like MCP servers)
- JSON Schema-based parameter validation

**History Management** (`history.go`) - Cross-provider conversation history with versioning and portable serialization for stateless applications.

**MCP Integration** (`mcp/`) - Model Context Protocol support for connecting to external tool servers via stdio or HTTP SSE.

**Facilitator** (`facilitator.go`) - Controls session termination and provides the default `respond_to_user` tool for conversation completion.

### Key Interfaces

```go
type LLMClient interface {
    NewSession(ctx context.Context, options ...SessionOption) (Session, error)
    GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error)
}

type Tool interface {
    Spec() ToolSpec
    Run(ctx context.Context, args map[string]any) (map[string]any, error)
}

type ToolSet interface {
    Specs(ctx context.Context) ([]ToolSpec, error)
    Run(ctx context.Context, name string, args map[string]any) (map[string]any, error)
}
```

### LLM Provider Support

Each provider in `llm/` handles format conversion between gollem's unified interface and provider-specific APIs:
- **OpenAI**: GPT models with function calling
- **Claude**: Anthropic models with tool use  
- **Gemini**: Google Vertex AI models with function calling

### Testing Patterns

- Mock generation using `moq` for interfaces (stored in `mock/`)
- Provider-specific conversion tests (`convert_test.go` in each LLM package)
- Integration tests that use real APIs when keys are available
- Export tests for testing internal functionality

### Examples

See `examples/` directory for usage patterns:
- `basic/` - Simple agent with tools
- `chat/` - Interactive conversation
- `embedding/` - Vector generation
- `mcp/` - MCP server integration
- `tools/` - Custom tool creation