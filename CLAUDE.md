# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Restriction & Rules

- In principle, do not trust developers who use this library from outside
  - Do not export unnecessary methods, structs, and variables
  - Assume that exposed items will be changed. Never expose fields that would be problematic if changed
  - Use `export_test.go` for items that need to be exposed for testing purposes
- When making changes, before finishing the task, always:
  - Run `go vet ./...`, `go fmt ./...` to format the code
  - Run `golangci-lint run ./...` to check lint error
  - Run `gosec -quiet ./...` to check security issue
  - Run tests to ensure no impact on other code
- All comment and character literal in source code must be in English
- Test files should have `package {name}_test`. Do not use same package name
- Test must be included in same name test file. (e.g. test for `abc.go` must be in `abc_test.go`)
- Use named empty structure (e.g. `type ctxHogeKey struct{}` ) as private context key
- Do not create binary. If you need to run, use `go run` command instead
- When a `tmp` directory is specified, search for files within the `./tmp` directory relative to the project root.
- **Naming Convention**: In Go, package names already express the subject, so avoid redundancy in method/struct names
  - Good: `openai.NewHistory()`, `claude.ToMessages()`, `gemini.ToContents()`
  - Bad: `openai.NewHistoryFromOpenAI()`, `claude.ToClaude()`, `gemini.ToGemini()`
- **Environment Variables**: Outside of test code, NEVER use `os.Getenv()` or `os.LookupEnv()` directly. Always use `github.com/urfave/cli/v3` for environment variable access
  - This ensures proper configuration management and testability
  - Test code may use `os.Getenv()` for test setup purposes


## Commands

### Development and Testing
- `task` or `task mock` - Generate mock files for testing (uses moq)
- `go test ./...` - Run all tests (MUST run before exiting tasks)
- `go test -v ./llm/openai/` - Run tests for specific package
- `go test -v ./path/to/package` - Run specific package tests when developing
- `go build ./...` - Build all packages
- `go mod tidy` - Clean up dependencies

### Test Execution
Tests may require API keys for integration testing:
- OpenAI: `OPENAI_API_KEY`
- Anthropic: `ANTHROPIC_API_KEY`
- Gemini: `GEMINI_PROJECT_ID`, `GEMINI_LOCATION`

When debugging tests with LLM logging enabled, use `zenv -p debug go test` to enable logging environment variables:
```bash
# Enable logging for debugging
zenv -p debug go test -v ./strategy/planexec/ -run TestName

# This automatically enables GOLLEM_LOGGING_*_PROMPT and GOLLEM_LOGGING_*_RESPONSE
# as defined in .env.toml [debug] profile
```

### Code Quality
- MUST run `go test ./...` before completing any task
- Use `export_test.go` files to access internal packages for testing
- Clean up any test binaries after checking

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

**MCP Integration** (`mcp/`) - Model Context Protocol support for connecting to external tool servers via stdio or Streamable HTTP.

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
- Private methods can be exported for testing via `export_test.go` (e.g., `var IsTokenLimitError = isTokenLimitError`)

### Examples

See `examples/` directory for usage patterns:
- `basic/` - Simple agent with tools
- `chat/` - Interactive conversation
- `embedding/` - Vector generation
- `mcp/` - MCP server integration
- `tools/` - Custom tool creation
- `plan_mode/` - Plan mode agent with goal-oriented task execution
- `query/` - Simple LLM query without conversation state
- `simple/` - Minimal example

## Development Guidelines

### Error Handling
Use `github.com/m-mizutani/goerr/v2` for error wrapping:
```go
if err := validateData(t.Data); err != nil {
    return goerr.Wrap(err, "failed to validate data", goerr.Value("name", t.Name))
}
```

### Testing Framework
- Use `github.com/m-mizutani/gt` for testing (leverages Go generics)
- Use Helper Driven Testing style instead of Table Driven Tests
- All comments and literals MUST be in English

Example test pattern:
```go
type testCase struct {
    input    string
    expected string
}

runTest := func(tc testCase) func(t *testing.T) {
    return func(t *testing.T) {
        actual := someFunc(tc.input)
        gt.Equal(t, tc.expected, actual)
    }
}

t.Run("success case", runTest(testCase{
    input: "blue",
    expected: "BLUE",
}))
```

## Logging

### LLM Request and Response Logging

gollem provides detailed logging capabilities for LLM interactions to support debugging, monitoring, and observability.

#### Prompt Logging

Each LLM provider supports prompt logging through dedicated environment variables:

- **Claude**: `GOLLEM_LOGGING_CLAUDE_PROMPT`
- **OpenAI**: `GOLLEM_LOGGING_OPENAI_PROMPT`  
- **Gemini**: `GOLLEM_LOGGING_GEMINI_PROMPT`

When enabled, logs include:
- System prompts
- User messages
- Tool/function responses
- Message history context

#### Response Logging

Response logging captures LLM outputs for analysis and debugging:

- **Claude**: `GOLLEM_LOGGING_CLAUDE_RESPONSE`
- **OpenAI**: `GOLLEM_LOGGING_OPENAI_RESPONSE`
- **Gemini**: `GOLLEM_LOGGING_GEMINI_RESPONSE`

Response logs include:
- Generated text content
- Tool/function calls with IDs, names, and arguments
- Token usage statistics (input/output tokens)
- Model information and finish reasons
- Both streaming and non-streaming responses

#### Usage Examples

```bash
# Enable Claude prompt and response logging
export GOLLEM_LOGGING_CLAUDE_PROMPT=true
export GOLLEM_LOGGING_CLAUDE_RESPONSE=true

# Enable OpenAI response logging only
export GOLLEM_LOGGING_OPENAI_RESPONSE=true

# Enable all Gemini logging
export GOLLEM_LOGGING_GEMINI_PROMPT=true
export GOLLEM_LOGGING_GEMINI_RESPONSE=true
```

#### Log Structure

Logs use structured format with ctxlog scopes:

```json
{
  "level": "info",
  "scope": "claude_response",
  "model": "claude-3-sonnet-20240229",
  "stop_reason": "end_turn",
  "usage": {
    "input_tokens": 150,
    "output_tokens": 75
  },
  "content": [
    {
      "type": "text", 
      "text": "Generated response text"
    },
    {
      "type": "tool_use",
      "id": "call_123",
      "name": "search_function", 
      "input": {"query": "example"}
    }
  ]
}
```

#### Benefits

- **Debugging**: Track exact prompts and responses during development
- **Monitoring**: Observe token usage and response patterns
- **Audit**: Log tool calls and function executions
- **Performance**: Analyze response times and token efficiency
- **Troubleshooting**: Capture complete interaction context for issue resolution