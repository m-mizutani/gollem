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
  - Diverse agent behavior control mechanisms (Hooks, Facilitators)

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

	// Create agent with tools and hooks
	agent := gollem.New(client,
		gollem.WithTools(&GreetingTool{}),
		gollem.WithSystemPrompt("You are a helpful assistant."),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
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
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
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

### Manual History Management (Legacy)

For backward compatibility, the `Prompt` method is still available but deprecated:

```go
// Legacy approach (not recommended)
history1, err := agent.Prompt(ctx, "Hello")
history2, err := agent.Prompt(ctx, "Continue", gollem.WithHistory(history1))
```

### Tool Integration

gollem supports two types of tools:

1. **Built-in Tools**: Custom Go functions you implement
2. **MCP Tools**: External tools via Model Context Protocol servers

```go
agent := gollem.New(client,
	gollem.WithTools(&MyCustomTool{}),           // Built-in tools
	gollem.WithToolSets(mcpClient),              // MCP tools
	gollem.WithSystemPrompt("You are helpful."), // System instructions
)
```

### Response Modes

Choose between blocking and streaming responses:

```go
agent := gollem.New(client,
	gollem.WithResponseMode(gollem.ResponseModeStreaming), // Real-time streaming
	gollem.WithMessageHook(func(ctx context.Context, msg string) error {
		fmt.Print(msg) // Print tokens as they arrive
		return nil
	}),
)
```

### Comprehensive Hook System

gollem provides a powerful hook system for monitoring, logging, and controlling agent behavior. Hooks are callback functions that are invoked at specific points during execution.

#### Available Hooks

**1. MessageHook** - Called when the LLM generates text
```go
gollem.WithMessageHook(func(ctx context.Context, msg string) error {
	// Log or process each message from the LLM
	log.Printf("🤖 LLM: %s", msg)
	
	// Stream to UI, save to database, etc.
	if err := streamToUI(msg); err != nil {
		return err // Abort execution on error
	}
	return nil
})
```

**2. ToolRequestHook** - Called before executing any tool
```go
gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
	// Monitor tool usage, implement access control
	log.Printf("⚡ Executing tool: %s with args: %v", tool.Name, tool.Arguments)
	
	// Implement security checks
	if !isToolAllowed(tool.Name) {
		return fmt.Errorf("tool %s not allowed", tool.Name)
	}
	
	// Rate limiting, usage tracking, etc.
	return trackToolUsage(tool.Name)
})
```

**3. ToolResponseHook** - Called after successful tool execution
```go
gollem.WithToolResponseHook(func(ctx context.Context, tool gollem.FunctionCall, response map[string]any) error {
	// Log successful tool executions
	log.Printf("✅ Tool %s completed: %v", tool.Name, response)
	
	// Post-process results, update metrics
	return updateToolMetrics(tool.Name, response)
})
```

**4. ToolErrorHook** - Called when tool execution fails
```go
gollem.WithToolErrorHook(func(ctx context.Context, err error, tool gollem.FunctionCall) error {
	// Handle tool failures
	log.Printf("❌ Tool %s failed: %v", tool.Name, err)
	
	// Decide whether to continue or abort
	if isCriticalTool(tool.Name) {
		return err // Abort execution
	}
	
	// Log and continue for non-critical tools
	logToolError(tool.Name, err)
	return nil // Continue execution
})
```

**5. LoopHook** - Called at the start of each conversation loop
```go
gollem.WithLoopHook(func(ctx context.Context, loop int, input []gollem.Input) error {
	// Monitor conversation progress
	log.Printf("🔄 Loop %d starting with %d inputs", loop, len(input))
	
	// Implement custom loop limits or conditions
	if loop > customLimit {
		return fmt.Errorf("custom loop limit exceeded")
	}
	
	// Track conversation metrics
	return updateLoopMetrics(loop, input)
})
```

#### Practical Hook Examples

**Real-time Streaming to WebSocket:**
```go
agent := gollem.New(client,
	gollem.WithMessageHook(func(ctx context.Context, msg string) error {
		// Stream LLM responses to WebSocket clients
		return websocketBroadcast(msg)
	}),
	gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
		// Notify clients about tool execution
		return websocketSend(fmt.Sprintf("Executing: %s", tool.Name))
	}),
)
```

**Comprehensive Logging and Monitoring:**
```go
agent := gollem.New(client,
	gollem.WithMessageHook(func(ctx context.Context, msg string) error {
		logger.Info("LLM response", "message", msg, "length", len(msg))
		metrics.IncrementCounter("llm_messages_total")
		return nil
	}),
	gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
		logger.Info("Tool execution started", 
			"tool", tool.Name, 
			"args", tool.Arguments,
			"request_id", ctx.Value("request_id"))
		metrics.IncrementCounter("tool_executions_total", "tool", tool.Name)
		return nil
	}),
	gollem.WithToolResponseHook(func(ctx context.Context, tool gollem.FunctionCall, response map[string]any) error {
		logger.Info("Tool execution completed", 
			"tool", tool.Name, 
			"response_size", len(response))
		metrics.RecordDuration("tool_execution_duration", tool.Name, time.Since(startTime))
		return nil
	}),
	gollem.WithToolErrorHook(func(ctx context.Context, err error, tool gollem.FunctionCall) error {
		logger.Error("Tool execution failed", 
			"tool", tool.Name, 
			"error", err)
		metrics.IncrementCounter("tool_errors_total", "tool", tool.Name)

		// Continue execution for non-critical errors
		if !isCriticalError(err) {
			return nil
		}
		return err
	}),
)
```

**Security and Access Control:**
```go
agent := gollem.New(client,
	gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
		userID := ctx.Value("user_id").(string)

		// Check permissions
		if !hasPermission(userID, tool.Name) {
			return fmt.Errorf("user %s not authorized for tool %s", userID, tool.Name)
		}

		// Rate limiting
		if isRateLimited(userID, tool.Name) {
			return fmt.Errorf("rate limit exceeded for user %s", userID)
		}

		return nil
	}),
)
```

**Error Recovery and Fallbacks:**
```go
agent := gollem.New(client,
	gollem.WithToolErrorHook(func(ctx context.Context, err error, tool gollem.FunctionCall) error {
		// Implement fallback mechanisms
		switch tool.Name {
		case "external_api":
			// Use cached data as fallback
			if cachedData := getFromCache(tool.Arguments); cachedData != nil {
				log.Printf("Using cached data for %s", tool.Name)
				return nil // Continue with cached data
			}
		case "file_operation":
			// Retry with different parameters
			if retryCount < maxRetries {
				return retryWithBackoff(tool, retryCount)
			}
		}

		// Log error and continue for non-critical tools
		logError(tool.Name, err)
		return nil
	}),
	gollem.WithLoopLimit(10),    // Prevent infinite loops
	gollem.WithRetryLimit(3),    // Retry failed operations
)
```

### Plan Mode - Goal-Oriented Agent

Plan mode enables goal-oriented task execution with intelligent planning and adaptive execution. The agent breaks down complex goals into structured steps and can intelligently skip redundant tasks based on previous execution results.

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

	// Create agent with plan mode configuration
	agent := gollem.New(client,
		gollem.WithTools(&SearchTool{}, &AnalysisTool{}, &ReportTool{}),
		gollem.WithPlanExecutionMode(gollem.PlanExecutionModeBalanced), // Default: intelligent skipping
		gollem.WithSkipConfidenceThreshold(0.8), // Skip tasks with 80%+ confidence
		gollem.WithSkipConfirmationHook(func(ctx context.Context, plan *gollem.Plan, decision gollem.SkipDecision) bool {
			// Custom skip confirmation logic
			fmt.Printf("🤔 Skip decision (confidence: %.2f): %s\n", decision.Confidence, decision.SkipReason)
			return decision.Confidence >= 0.8 // Auto-approve high confidence skips
		}),
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
		}),
	)

	// Create and execute a plan
	plan, err := agent.Plan(ctx, "Research and analyze the latest trends in AI for 2024, then create a comprehensive report")
	if err != nil {
		panic(err)
	}

	// Execute the plan (plan automatically handles step-by-step execution)
	result, err := plan.Execute(ctx)
	if err != nil {
		panic(err)
	}

	// Get plan progress
	todos := plan.GetToDos()
	completed := 0
	for _, todo := range todos {
		if todo.Status == "Completed" {
			completed++
		}
	}
	fmt.Printf("✅ Plan completed: %s\n", result)
	fmt.Printf("📊 Executed %d out of %d steps\n", completed, len(todos))
}
```

**Key Features:**
- **Intelligent Planning**: Automatically breaks down complex goals into manageable steps
- **Adaptive Execution**: Skips redundant tasks based on previous results and confidence levels
- **Execution Modes**: Complete (no skipping), Balanced (default, smart skipping), Efficient (aggressive skipping)
- **Transparency**: Detailed reasoning for all skip decisions with confidence scores

### Facilitator - Conversation Flow Control

Facilitators control the conversation flow and determine when conversations should continue or end. gollem includes a default facilitator, but you can implement custom ones:

```go
// Custom facilitator example
type CustomFacilitator struct {
	completed bool
}

func (f *CustomFacilitator) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "complete_task",
		Description: "Mark the current task as completed",
		Parameters:  map[string]*gollem.Parameter{},
	}
}

func (f *CustomFacilitator) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	f.completed = true
	return map[string]any{"status": "completed"}, nil
}

func (f *CustomFacilitator) IsCompleted() bool {
	return f.completed
}

func (f *CustomFacilitator) ProceedPrompt() string {
	return "What should we do next?"
}

// Use custom facilitator
agent := gollem.New(client,
	gollem.WithFacilitator(&CustomFacilitator{}),
	gollem.WithTools(tools...),
)
```

### Memory Management with History Compactor

gollem provides intelligent memory management through automatic history compaction, which summarizes old conversation messages to reduce token usage while preserving context. **History compaction is enabled by default** to prevent token limit issues during long conversations.

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

	// Agent with default compaction (automatically enabled)
	agent := gollem.New(client,
		gollem.WithTools(&YourTool{}),
		// History compaction is enabled by default with standard settings
	)

	// Or customize compaction behavior
	compactor := gollem.NewHistoryCompactor(client,
		gollem.WithMaxTokens(50000),           // Start compaction at 50k tokens
		gollem.WithPreserveRecentTokens(10000), // Preserve 10k tokens of recent context
		gollem.WithCompactionSystemPrompt("Custom summarization instructions..."),
	)

	// Agent with custom compaction settings
	agentCustom := gollem.New(client,
		gollem.WithTools(&YourTool{}),
		gollem.WithHistoryCompactor(compactor), // Override default compactor
		gollem.WithCompactionHook(func(ctx context.Context, original, compacted *gollem.History) error {
			fmt.Printf("🗜️  History compacted: %d → %d messages (%.1f%% reduction)\n", 
				original.ToCount(), compacted.ToCount(),
				float64(original.ToCount()-compacted.ToCount())/float64(original.ToCount())*100)
			return nil
		}),
	)

	// Long conversation that will trigger automatic compaction
	for i := 0; i < 20; i++ {
		prompt := fmt.Sprintf("Tell me about topic %d in detail", i)
		if err := agent.Execute(ctx, prompt); err != nil {
			panic(err)
		}
		// Or use the default agent - compaction still happens automatically
		if err := agentCustom.Execute(ctx, prompt); err != nil {
			panic(err)
		}
	}
}
```

**Key Features:**
- **Intelligent Summarization**: Automatically summarizes old messages while preserving critical context
- **Configurable Thresholds**: Control when compaction triggers based on token count or LLM context limits  
- **Recent Message Preservation**: Always keeps recent messages intact for conversation continuity
- **Context-Aware**: Preserves tool calls, responses, and important conversation state
- **Multi-LLM Support**: Works with OpenAI, Claude, and Gemini models with appropriate token limits

**Compaction Options:**
- `WithMaxTokens(int)`: Set custom token threshold (default: 50,000)
- `WithPreserveRecentTokens(int)`: Recent tokens to preserve (default: 10,000)
- `WithCompactionSystemPrompt(string)`: Custom summarization instructions
- `WithCompactionPromptTemplate(string)`: Custom prompt template for summarization
- `WithHistoryCompaction(false)`: Disable automatic compaction if needed

## Examples

See the [examples](https://github.com/m-mizutani/gollem/tree/main/examples) directory for complete working examples:

- **[Simple](examples/simple)**: Minimal example for getting started
- **[Query](examples/query)**: Simple LLM query without conversation state
- **[Basic](examples/basic)**: Simple agent with custom tools
- **[Chat](examples/chat)**: Interactive chat application
- **[MCP](examples/mcp)**: Integration with MCP servers
- **[Tools](examples/tools)**: Custom tool development
- **[Embedding](examples/embedding)**: Text embedding generation
- **[Plan Mode](examples/plan_mode)**: Goal-oriented agent with intelligent task planning
- **[Memory Compaction](examples/memory_compaction)**: Automatic history compaction for long conversations
- **[Plan Compaction](examples/plan_compaction)**: History compaction during plan execution

## Documentation

For detailed documentation and advanced usage:

- **[Getting Started Guide](doc/getting-started.md)**
- **[Tool Development](doc/tools.md)**
- **[MCP Integration](doc/mcp.md)**
- **[LLM Provider Configuration](docs/llm.md)** - Detailed configuration options for each LLM provider
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
		gollem.WithMessageHook(func(ctx context.Context, msg string) error {
			fmt.Printf("🤖 %s\n", msg)
			return nil
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

### Available Models

- `claude-sonnet-4@20250514` (default) - Latest Claude Sonnet model
- `claude-haiku-3@20240307` - Fast, cost-effective model
- `claude-opus-3@20240229` - Most capable model

### Benefits of Vertex AI Integration

- **Unified Google Cloud billing** and cost management
- **Enterprise security** with VPC, private endpoints, and audit logs
- **Regional deployment** for data residency requirements
- **Vertex AI MLOps** integration for monitoring and management
- **Consistent API** - same gollem interface across all providers

## Debugging

### Logging LLM Prompts

You can enable prompt logging for debugging purposes by setting environment variables:

- `GOLLEM_LOGGING_CLAUDE_PROMPT=true` - Log all prompts sent to Claude
- `GOLLEM_LOGGING_OPENAI_PROMPT=true` - Log all prompts sent to OpenAI
- `GOLLEM_LOGGING_GEMINI_PROMPT=true` - Log all prompts sent to Gemini

These will output the raw prompts to the standard logger before sending them to the LLM provider, which is useful for debugging conversation flow and tool usage.

## License

Apache 2.0 License. See [LICENSE](LICENSE) for details.
