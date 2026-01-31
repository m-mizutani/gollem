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
  - Via Google Vertex AI (see [Claude on Vertex AI](#claude-via-vertex-ai))
- [x] **OpenAI** (see [models](https://platform.openai.com/docs/models))

## Quick Start

### Install

```bash
go get github.com/m-mizutani/gollem
```

### Examples

#### Simple LLM Query

For basic text generation without tools or conversation history:

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

#### Agent with Automatic Session Management (Recommended)

For building conversational agents with tools and automatic history management:

```go
package main

import (
	"bufio"
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
)

// Custom tool example
type GreetingTool struct{}

func (t *GreetingTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "greeting",
		Description: "Returns a personalized greeting",
		Parameters: map[string]*gollem.Parameter{
			"name": {
				Type:        gollem.TypeString,
				Description: "Name of the person to greet",
			},
		},
		Required: []string{"name"},
	}
}

func (t *GreetingTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	name, ok := args["name"].(string)
	if !ok {
		return nil, fmt.Errorf("name is required")
	}
	return map[string]any{
		"message": fmt.Sprintf("Hello, %s! Nice to meet you!", name),
	}, nil
}

func main() {
	ctx := context.Background()

	// Create LLM client
	client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create agent with tools and middleware
	agent := gollem.New(client,
		gollem.WithTools(&GreetingTool{}),
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

	// Interactive conversation loop
	scanner := bufio.NewScanner(os.Stdin)
	for {
		fmt.Print("> ")
		if !scanner.Scan() {
			break
		}
		
		// Execute handles session management automatically
		if err := agent.Execute(ctx, scanner.Text()); err != nil {
			fmt.Printf("Error: %v\n", err)
		}
		
		// Access conversation history if needed
		history := agent.Session().History()
		fmt.Printf("(Conversation has %d messages)\n", history.ToCount())
	}
}
```

#### Generate Embedding

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollem/llm/openai"
)

func main() {
	ctx := context.Background()

	// Create OpenAI client
	client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Generate embeddings
	embeddings, err := client.GenerateEmbedding(ctx, 1536, []string{
		"Hello, world!",
		"This is a test",
	})
	if err != nil {
		panic(err)
	}

	fmt.Printf("Generated %d embeddings with %d dimensions each\n", 
		len(embeddings), len(embeddings[0]))
}
```

#### Agent with MCP Server Integration

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
	ctx := context.Background()

	// Create OpenAI client
	client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Create MCP clients with custom client info
	// Stdio transport for local MCP servers
	mcpLocal, err := mcp.NewStdio(ctx, "./mcp-server", []string{},
		mcp.WithStdioClientInfo("my-app", "1.0.0"),
		mcp.WithEnvVars([]string{"MCP_ENV=production"}))
	if err != nil {
		panic(err)
	}
	defer mcpLocal.Close()

	// StreamableHTTP transport for remote MCP servers (recommended)
	mcpRemote, err := mcp.NewStreamableHTTP(ctx, "http://localhost:8080",
		mcp.WithStreamableHTTPClientInfo("my-app-client", "1.0.0"))
	if err != nil {
		panic(err)
	}
	defer mcpRemote.Close()

	// Note: NewSSE is also available but deprecated in favor of NewStreamableHTTP
	// See doc/mcp.md for complete transport options and configuration

	// Create agent with MCP tools
	agent := gollem.New(client,
		gollem.WithToolSets(mcpLocal, mcpRemote),
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

	// Execute task with MCP tools
	err = agent.Execute(ctx, "Please help me with file operations using available tools.")
	if err != nil {
		panic(err)
	}
}
```

## Key Features

### Automatic Session Management

The `Execute` method provides automatic session management:
- **Continuous conversations**: No need to manually manage history
- **Session persistence**: Conversation context is maintained across multiple calls
- **Simplified API**: Just call `Execute` repeatedly for ongoing conversations

```go
agent := gollem.New(client, gollem.WithTools(tools...))

// First interaction
err := agent.Execute(ctx, "Hello, I'm working on a project.")

// Follow-up (automatically remembers previous context)
err = agent.Execute(ctx, "Can you help me with the next step?")

// Access full conversation history
history := agent.Session().History()
```

### Manual Session Management (Advanced)

For advanced use cases where you need direct session control:

```go
// Create session manually
session, err := client.NewSession(ctx)
if err != nil {
	panic(err)
}

// Use session directly
result, err := session.GenerateContent(ctx, gollem.Text("Hello"))
```

### Tool Integration

gollem supports multiple types of tools:

1. **Built-in Tools**: Custom Go functions you implement
2. **MCP Tools**: External tools via Model Context Protocol servers
3. **SubAgents**: Specialized child agents that can be invoked as tools

```go
agent := gollem.New(client,
	gollem.WithTools(&MyCustomTool{}),           // Built-in tools
	gollem.WithToolSets(mcpClient),              // MCP tools
	gollem.WithSubAgents(reviewerAgent),         // SubAgents
	gollem.WithSystemPrompt("You are helpful."), // System instructions
)
```

### Response Modes

Choose between blocking and streaming responses:

```go
agent := gollem.New(client,
	gollem.WithResponseMode(gollem.ResponseModeStreaming), // Real-time streaming
	gollem.WithContentStreamMiddleware(func(next gollem.ContentStreamHandler) gollem.ContentStreamHandler {
		return func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
			ch, err := next(ctx, req)
			if err != nil {
				return nil, err
			}
			// Print tokens as they arrive
			outCh := make(chan *gollem.ContentResponse)
			go func() {
				defer close(outCh)
				for resp := range ch {
					if len(resp.Texts) > 0 {
						for _, text := range resp.Texts {
							fmt.Print(text)
						}
					}
					outCh <- resp
				}
			}()
			return outCh, nil
		}
	}),
)
```

### Structured Output with JSON Schema

gollem supports structured output using JSON Schema to ensure LLM responses conform to a specific format. This is useful for data extraction, form filling, and structured data generation.

```go
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
)

func main() {
	ctx := context.Background()
	client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
	if err != nil {
		panic(err)
	}

	// Define response schema
	schema := &gollem.Parameter{
		Title:       "UserProfile",
		Description: "Structured user profile information",
		Type:        gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"name": {
				Type:        gollem.TypeString,
				Description: "Full name of the user",
			},
			"age": {
				Type:        gollem.TypeInteger,
				Description: "Age in years",
			},
			"email": {
				Type:        gollem.TypeString,
				Description: "Email address",
			},
		},
		Required: []string{"name", "email"},
	}

	// Create session with JSON content type and schema
	session, err := client.NewSession(ctx,
		gollem.WithSessionContentType(gollem.ContentTypeJSON),
		gollem.WithSessionResponseSchema(schema),
	)
	if err != nil {
		panic(err)
	}

	// Generate structured content
	resp, err := session.GenerateContent(ctx,
		gollem.Text("Extract user info: John Doe, 30 years old, john@example.com"))
	if err != nil {
		panic(err)
	}

	// Response will be valid JSON matching the schema
	var profile map[string]any
	json.Unmarshal([]byte(resp.Texts[0]), &profile)
	fmt.Printf("Name: %s, Email: %s\n", profile["name"], profile["email"])
}
```

**Key features:**
- **Type safety**: Define expected structure with `gollem.Parameter` types (`TypeString`, `TypeInteger`, `TypeObject`, `TypeArray`, etc.)
- **Validation**: Specify constraints like `Required`, `Minimum`, `Maximum`, `Pattern`, `Enum`
- **Nested structures**: Support for nested objects and arrays
- **Provider agnostic**: Works with OpenAI, Claude, and Gemini
- **Guaranteed format**: LLM output always conforms to the schema

#### Creating Schemas from Go Structs

Instead of manually constructing schema objects, you can automatically generate them from Go structs using field tags:

```go
type UserProfile struct {
	Name     string `json:"name" description:"User's full name" required:"true"`
	Email    string `json:"email" description:"Email address" pattern:"^[a-zA-Z0-9._%+-]+@..." required:"true"`
	Age      int    `json:"age" description:"Age in years" min:"0" max:"150"`
	Role     string `json:"role" description:"User role" enum:"admin,user,guest"`
}

// Generate schema from struct
schema, err := gollem.ToSchema(UserProfile{})
if err != nil {
	panic(err)
}
schema.Title = "UserProfile"
schema.Description = "Structured user profile information"

// Use with session as before
session, err := client.NewSession(ctx,
	gollem.WithSessionContentType(gollem.ContentTypeJSON),
	gollem.WithSessionResponseSchema(schema),
)
```

**Supported struct tags:**
- `json:"field_name"` - Field name (use `json:"-"` to ignore)
- `description:"text"` - Field description
- `enum:"value1,value2,value3"` - Enum values
- `min:"0"`, `max:"100"` - Numeric constraints
- `minLength:"1"`, `maxLength:"255"` - String length
- `pattern:"^[a-z]+$"` - Regex pattern
- `minItems:"1"`, `maxItems:"10"` - Array constraints
- `required:"true"` - Required field

See [examples/json_schema](examples/json_schema) for a complete working example.

### Middleware System

gollem provides a powerful middleware system for monitoring, logging, and controlling agent behavior. Middleware functions wrap the core handlers to add cross-cutting concerns.

#### Available Middleware Types

**1. ContentBlockMiddleware** - Wraps synchronous content generation
```go
gollem.WithContentBlockMiddleware(func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
	return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
		// Pre-processing: log request, validate inputs
		log.Printf("📝 Generating content with %d inputs", len(req.Inputs))

		// Execute core handler
		resp, err := next(ctx, req)

		// Post-processing: log response, track metrics
		if err == nil && len(resp.Texts) > 0 {
			for _, text := range resp.Texts {
				log.Printf("🤖 LLM: %s", text)
			}
			metrics.IncrementCounter("llm_messages_total")
		}

		return resp, err
	}
})
```

**2. ContentStreamMiddleware** - Wraps streaming content generation
```go
gollem.WithContentStreamMiddleware(func(next gollem.ContentStreamHandler) gollem.ContentStreamHandler {
	return func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
		// Execute core handler
		ch, err := next(ctx, req)
		if err != nil {
			return nil, err
		}

		// Wrap response channel for processing
		outCh := make(chan *gollem.ContentResponse)
		go func() {
			defer close(outCh)
			for resp := range ch {
				// Process each streaming chunk
				if len(resp.Texts) > 0 {
					for _, text := range resp.Texts {
						fmt.Print(text) // Stream to UI
					}
				}
				outCh <- resp
			}
		}()

		return outCh, nil
	}
})
```

**3. ToolMiddleware** - Wraps tool execution
```go
gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
	return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
		// Pre-execution: security checks, logging
		log.Printf("⚡ Executing tool: %s", req.Tool.Name)

		// Implement access control
		if !isToolAllowed(req.Tool.Name) {
			return &gollem.ToolExecResponse{
				Error: fmt.Errorf("tool %s not allowed", req.Tool.Name),
			}, nil
		}

		// Execute tool
		resp, err := next(ctx, req)

		// Post-execution: metrics, error handling
		if resp.Error != nil {
			log.Printf("❌ Tool %s failed: %v", req.Tool.Name, resp.Error)
			metrics.IncrementCounter("tool_errors_total", "tool", req.Tool.Name)
		} else {
			log.Printf("✅ Tool %s completed in %dms", req.Tool.Name, resp.Duration)
			metrics.RecordDuration("tool_execution_duration", req.Tool.Name, resp.Duration)
		}

		return resp, err
	}
})
```

#### Practical Middleware Examples

**Real-time Streaming to WebSocket:**
```go
agent := gollem.New(client,
	gollem.WithContentStreamMiddleware(func(next gollem.ContentStreamHandler) gollem.ContentStreamHandler {
		return func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
			ch, err := next(ctx, req)
			if err != nil {
				return nil, err
			}

			outCh := make(chan *gollem.ContentResponse)
			go func() {
				defer close(outCh)
				for resp := range ch {
					// Broadcast to WebSocket clients
					if len(resp.Texts) > 0 {
						for _, text := range resp.Texts {
							websocketBroadcast(text)
						}
					}
					outCh <- resp
				}
			}()
			return outCh, nil
		}
	}),
	gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
		return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
			// Notify clients about tool execution
			websocketSend(fmt.Sprintf("Executing: %s", req.Tool.Name))
			return next(ctx, req)
		}
	}),
)
```

**Comprehensive Logging and Monitoring:**
```go
agent := gollem.New(client,
	gollem.WithContentBlockMiddleware(func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
		return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
			resp, err := next(ctx, req)
			if err == nil {
				logger.Info("LLM response",
					"texts", len(resp.Texts),
					"input_tokens", resp.InputToken,
					"output_tokens", resp.OutputToken)
				metrics.IncrementCounter("llm_messages_total")
			}
			return resp, err
		}
	}),
	gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
		return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
			logger.Info("Tool execution started",
				"tool", req.Tool.Name,
				"args", req.Tool.Arguments,
				"request_id", ctx.Value("request_id"))

			resp, err := next(ctx, req)

			if resp.Error != nil {
				logger.Error("Tool execution failed",
					"tool", req.Tool.Name,
					"error", resp.Error)
				metrics.IncrementCounter("tool_errors_total", "tool", req.Tool.Name)
			} else {
				logger.Info("Tool execution completed",
					"tool", req.Tool.Name,
					"duration_ms", resp.Duration)
				metrics.RecordDuration("tool_execution_duration", req.Tool.Name, resp.Duration)
			}

			return resp, err
		}
	}),
)
```

**Security and Access Control:**
```go
agent := gollem.New(client,
	gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
		return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
			userID := ctx.Value("user_id").(string)

			// Check permissions
			if !hasPermission(userID, req.Tool.Name) {
				return &gollem.ToolExecResponse{
					Error: fmt.Errorf("user %s not authorized for tool %s", userID, req.Tool.Name),
				}, nil
			}

			// Rate limiting
			if isRateLimited(userID, req.Tool.Name) {
				return &gollem.ToolExecResponse{
					Error: fmt.Errorf("rate limit exceeded for user %s", userID),
				}, nil
			}

			return next(ctx, req)
		}
	}),
)
```

**Error Recovery and Fallbacks:**
```go
agent := gollem.New(client,
	gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
		return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
			resp, err := next(ctx, req)

			// Implement fallback mechanisms
			if resp.Error != nil {
				switch req.Tool.Name {
				case "external_api":
					// Use cached data as fallback
					if cachedData := getFromCache(req.Tool.Arguments); cachedData != nil {
						log.Printf("Using cached data for %s", req.Tool.Name)
						return &gollem.ToolExecResponse{
							Result: cachedData,
						}, nil
					}
				case "file_operation":
					// Retry with backoff
					if retryCount < maxRetries {
						time.Sleep(backoffDuration(retryCount))
						return next(ctx, req) // Retry
					}
				}
			}

			return resp, err
		}
	}),
	gollem.WithLoopLimit(10), // Prevent infinite loops
)
```

#### Built-in Middleware

**Automatic History Compaction (`compacter`)**

The compacter middleware automatically handles token limit errors by compressing conversation history using LLM summarization. When a token limit error is detected, it summarizes the oldest messages (default 70%) and retries the request.

```go
import "github.com/m-mizutani/gollem/middleware/compacter"

agent := gollem.New(client,
	gollem.WithContentBlockMiddleware(
		compacter.NewContentBlockMiddleware(
			client,
			compacter.WithCompactRatio(0.7),     // Compress oldest 70% of history (default)
			compacter.WithMaxRetries(3),         // Max retry attempts (default)
			compacter.WithCompactionHook(func(ctx context.Context, event *compacter.CompactionEvent) {
				// Observability: track compaction events
				log.Printf("Compacted: %d -> %d chars, tokens: in=%d out=%d",
					event.OriginalDataSize,
					event.CompactedDataSize,
					event.InputTokens,
					event.OutputTokens,
				)
			}),
		),
	),
	gollem.WithContentStreamMiddleware(
		compacter.NewContentStreamMiddleware(client),
	),
)
```

**Features:**
- **Automatic Recovery**: Detects `ErrTagTokenExceeded` and automatically compacts history
- **LLM-based Summarization**: Uses the same LLM client to generate high-quality summaries that preserve important context
- **Character-based Compression**: Compacts based on character count (default 70% of oldest messages)
- **Configurable**: Customize compression ratio, max retries, and summary prompt
- **Observability**: Hook for monitoring compaction events with metrics:
  - `OriginalDataSize` / `CompactedDataSize`: Character counts before/after
  - `InputTokens` / `OutputTokens`: Actual LLM token usage for summarization
  - `Summary`: Generated summary text
  - `Attempt`: Retry attempt number

**Example with Custom Settings:**
```go
middleware := compacter.NewContentBlockMiddleware(
	client,
	compacter.WithCompactRatio(0.8),  // Compress 80% of history
	compacter.WithMaxRetries(5),       // Allow up to 5 retries
	compacter.WithSummaryPrompt(`Summarize the conversation concisely, preserving:
- Key decisions and conclusions
- Important facts and context
- Action items and next steps`),
	compacter.WithLogger(logger),      // Custom logger
)
```

### Strategy Pattern for Agent Behavior

gollem uses the Strategy pattern to customize how agents process tasks and make decisions. Strategies control the core execution logic, from simple request-response to complex planning workflows.

#### Built-in Strategies

**1. Default Strategy** - Simple request-response pattern
```go
// Used automatically when no strategy is specified
agent := gollem.New(client,
	gollem.WithTools(&MyTool{}),
)
```

**2. React Strategy** - ReAct (Reasoning + Acting) pattern with step-by-step reasoning
```go
import "github.com/m-mizutani/gollem/strategy/react"

strategy := react.New(client)
agent := gollem.New(client,
	gollem.WithStrategy(strategy),
	gollem.WithTools(&MyTool{}),
)
```

**3. Plan & Execute Strategy** - Goal-oriented task planning and execution with context-aware planning
```go
import "github.com/m-mizutani/gollem/strategy/planexec"

strategy := planexec.New(client)
agent := gollem.New(client,
	gollem.WithStrategy(strategy),
	gollem.WithSystemPrompt("You are an expert data analyst. All outputs must be HIPAA compliant."),
	gollem.WithHistory(history), // Use conversation history
	gollem.WithTools(&SearchTool{}, &AnalysisTool{}),
)
```

The Plan & Execute strategy uses a three-phase approach with context embedding:
- **Planning**: Creates task breakdown with system prompt and history context. Important constraints and requirements are embedded into the Plan structure (`context_summary` and `constraints` fields), making the plan self-contained.
- **Execution**: Executes tasks sequentially using the main session with full context (system prompt and history).
- **Reflection**: After each task completion, evaluates results using ONLY the Plan's embedded information (goal, context summary, constraints) without accessing the original system prompt or history. This ensures consistent evaluation criteria and enables stateless reflection.

**External Plan Generation**: You can generate and reuse plans separately from execution:
```go
import "github.com/m-mizutani/gollem/strategy/planexec"

// Generate a plan separately
plan, err := planexec.GeneratePlan(ctx, client,
	[]gollem.Input{gollem.Text("Analyze security logs")},
	tools,                       // Available tools
	"Focus on OWASP Top 10",    // System prompt
	nil,                         // History (optional)
)

// Save plan for later or review
planData, _ := json.Marshal(plan)
savePlan(planData)

// Later: load and execute with pre-generated plan
var savedPlan *planexec.Plan
json.Unmarshal(planData, &savedPlan)

strategy := planexec.New(client, planexec.WithPlan(savedPlan))
agent := gollem.New(client, gollem.WithStrategy(strategy), gollem.WithTools(tools...))
resp, err := agent.Execute(ctx, gollem.Text("Analyze security logs"))
```

This enables use cases like:
- **Plan Review**: Generate plan, review tasks, then execute
- **Plan Caching**: Reuse plans for similar requests
- **Plan Modification**: Adjust tasks before execution
- **Parallel Planning**: Generate plans with one model, execute with another

#### Custom Strategy Implementation

Implement the `Strategy` interface for custom agent behavior:

```go
type Strategy interface {
	Init(ctx context.Context, inputs []Input) error
	Handle(ctx context.Context, state *StrategyState) ([]Input, *ExecuteResponse, error)
	Tools(ctx context.Context) ([]Tool, error)
}
```

**Example: Custom Strategy**
```go
type myStrategy struct {
	client gollem.LLMClient
}

func (s *myStrategy) Init(ctx context.Context, inputs []Input) error {
	// Initialize strategy state
	return nil
}

func (s *myStrategy) Handle(ctx context.Context, state *StrategyState) ([]Input, *ExecuteResponse, error) {
	// Custom decision logic
	resp, err := state.GenerateContent(ctx, state.Inputs...)
	if err != nil {
		return nil, nil, err
	}

	// Return next inputs and optional completion response
	return nil, &gollem.ExecuteResponse{Message: resp.Texts[0]}, nil
}

func (s *myStrategy) Tools(ctx context.Context) ([]Tool, error) {
	// Return available tools for this strategy
	return []Tool{}, nil
}

// Use custom strategy
agent := gollem.New(client,
	gollem.WithStrategy(&myStrategy{client: client}),
)
```

**Key Features:**
- **Pluggable Architecture**: Swap strategies without changing agent code
- **Built-in Patterns**: ReAct, Plan & Execute, or default simple execution
- **Custom Logic**: Implement your own decision-making algorithms
- **State Management**: Full access to conversation state and history

### Tracing

gollem provides a tracing system for observing agent execution. The `trace.Handler` interface (inspired by `slog.Handler`) lets you plug in different backends for recording agent lifecycle events: LLM calls, tool executions, sub-agent invocations, and custom strategy events.

#### In-Memory Recorder

`trace.New()` creates a built-in recorder that collects trace data into an in-memory tree structure. Useful for debugging, testing, and persistence via `Repository`. You can also set a custom trace ID with `trace.WithTraceID(id)` to correlate with external systems.

```go
import (
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/trace"
)

// Create a recorder that saves traces to files
rec := trace.New(
	trace.WithRepository(trace.NewFileRepository("./traces")),
	trace.WithMetadata(trace.TraceMetadata{
		Labels: map[string]string{"env": "production"},
	}),
)

agent := gollem.New(client, gollem.WithTrace(rec))
result, err := agent.Execute(ctx, gollem.Text("Hello"))

// Inspect the trace programmatically
tr := rec.Trace()
fmt.Printf("Trace %s: %d child spans\n", tr.TraceID, len(tr.RootSpan.Children))
```

#### OpenTelemetry Integration

The `trace/otel` package bridges gollem's trace events to OpenTelemetry spans, integrating with any OTel-compatible backend (Jaeger, Zipkin, OTLP, etc.).

```go
import (
	"github.com/m-mizutani/gollem"
	traceOtel "github.com/m-mizutani/gollem/trace/otel"
)

// Uses the global TracerProvider
agent := gollem.New(client, gollem.WithTrace(traceOtel.New()))

// Or with an explicit TracerProvider
agent = gollem.New(client, gollem.WithTrace(
	traceOtel.New(traceOtel.WithTracerProvider(tp)),
))
```

For more details including combining multiple handlers with `trace.Multi()`, see the [Tracing documentation](doc/tracing.md).

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

For detailed documentation and advanced usage:

- **[Getting Started Guide](doc/getting-started.md)**
- **[Tool Development](doc/tools.md)**
- **[MCP Integration](doc/mcp.md)**
- **[Structured Output with JSON Schema](doc/schema.md)** - Define response formats and extract structured data
- **[Tracing](doc/tracing.md)** - Agent execution tracing and observability (in-memory, OpenTelemetry)
- **[LLM Provider Configuration](doc/llm.md)** - Detailed configuration options for each LLM provider
- **[API Reference](https://pkg.go.dev/github.com/m-mizutani/gollem)**

## Claude via Vertex AI

gollem supports accessing Claude models through Google Vertex AI, allowing you to use Claude within Google Cloud's infrastructure with unified billing and enhanced security features.

### Setup

```go
package main

import (
	"context"
	"fmt"
	"os"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
)

func main() {
	ctx := context.Background()

	// Create Vertex AI Claude client
	client, err := claude.NewWithVertex(ctx, "your-region", "your-project-id",
		claude.WithVertexModel("claude-sonnet-4@20250514"), // Default model
		claude.WithVertexSystemPrompt("You are a helpful assistant."),
	)
	if err != nil {
		panic(err)
	}

	// Use the same agent interface as other providers
	agent := gollem.New(client,
		gollem.WithTools(&YourCustomTool{}),
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

	// Execute tasks normally
	err = agent.Execute(ctx, "Hello! Can you help me with my project?")
	if err != nil {
		panic(err)
	}
}
```

### Authentication

Vertex AI Claude client uses Google Cloud credentials:

```bash
# Option 1: Service account key file
export GOOGLE_APPLICATION_CREDENTIALS="path/to/service-account-key.json"

# Option 2: gcloud CLI authentication
gcloud auth application-default login

# Option 3: Use workload identity in GKE/Cloud Run (automatic)
```

### Benefits of Vertex AI Integration

- **Unified Google Cloud billing** and cost management
- **Enterprise security** with VPC, private endpoints, and audit logs
- **Regional deployment** for data residency requirements
- **Vertex AI MLOps** integration for monitoring and management
- **Consistent API** - same gollem interface across all providers

## Debugging

### Logging LLM Requests and Responses

You can enable detailed logging for debugging purposes by setting environment variables:

**Prompt Logging:**
- `GOLLEM_LOGGING_CLAUDE_PROMPT=true` - Log all prompts sent to Claude
- `GOLLEM_LOGGING_OPENAI_PROMPT=true` - Log all prompts sent to OpenAI
- `GOLLEM_LOGGING_GEMINI_PROMPT=true` - Log all prompts sent to Gemini

**Response Logging:**
- `GOLLEM_LOGGING_CLAUDE_RESPONSE=true` - Log all responses from Claude
- `GOLLEM_LOGGING_OPENAI_RESPONSE=true` - Log all responses from OpenAI
- `GOLLEM_LOGGING_GEMINI_RESPONSE=true` - Log all responses from Gemini

These will output the raw prompts and responses to the standard logger, which is useful for debugging conversation flow, tool usage, and token consumption.

## License

Apache 2.0 License. See [LICENSE](LICENSE) for details.
