# Tools in gollem

Tools are your own custom built-in functions that LLMs can use to perform specific actions in your application. This guide explains how to create and use tools with gollem.

## Creating a Tool

To create a tool, you need to implement the `Tool` interface:

```go
type Tool interface {
    Spec() ToolSpec
    Run(ctx context.Context, args map[string]any) (map[string]any, error)
}
```

Here's an example of a simple tool:

```go
type HelloTool struct{}

func (t *HelloTool) Spec() gollem.ToolSpec {
    return gollem.ToolSpec{
        Name:        "hello",
        Description: "Returns a greeting",
        Parameters: map[string]*gollem.Parameter{
            "name": {
                Type:        gollem.TypeString,
                Description: "Name of the person to greet",
            },
        },
        Required: []string{"name"},
    }
}

func (t *HelloTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
    return map[string]any{
        "message": fmt.Sprintf("Hello, %s!", args["name"]),
    }, nil
}
```

## Tool Specification

The `ToolSpec` defines the tool's interface:

- `Name`: Unique identifier for the tool
- `Description`: Human-readable description of what the tool does
- `Parameters`: Map of parameter names to their specifications
- `Required`: List of required parameter names (For Object type)

Each parameter specification includes:
- `Type`: Parameter type (string, number, boolean, etc.)
- `Description`: Human-readable description
- `Title`: Optional user-friendly name for the parameter
- `Required`: Optional boolean indicating if the parameter is required
- `RequiredFields`: List of required field names when Type is Object
- `Enum`: Optional list of allowed values
- `Properties`: Map of properties when Type is Object
- `Items`: Specification for array items when Type is Array
- `Minimum`/`Maximum`: Number constraints
- `MinLength`/`MaxLength`: String length constraints
- `Pattern`: Regular expression pattern for string validation
- `MinItems`/`MaxItems`: Array size constraints
- `Default`: Default value for the parameter

> [!CAUTION]
> Note that not all parameters are supported by every LLM, as parameter support varies between different LLM providers.

## Using Tools

To use tools with your agent:

```go
agent := gollem.New(client,
    gollem.WithTools(&HelloTool{}),
)

// Execute with automatic session management
err := agent.Execute(ctx, "Say hello to Alice")
if err != nil {
    panic(err)
}
```

You can add multiple tools:

```go
agent := gollem.New(client,
    gollem.WithTools(&HelloTool{}, &CalculatorTool{}, &WeatherTool{}),
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
    gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
        return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
            fmt.Printf("⚡ Executing: %s\n", req.Tool.Name)
            return next(ctx, req)
        }
    }),
)

// Execute multiple interactions with tools
err := agent.Execute(ctx, "Say hello to Bob and then calculate 15 + 27")
if err != nil {
    panic(err)
}
```

## Tool Monitoring with Middleware

gollem provides comprehensive middleware for monitoring tool execution:

```go
agent := gollem.New(client,
    gollem.WithTools(&HelloTool{}, &CalculatorTool{}),

    // Monitor and control tool execution
    gollem.WithToolMiddleware(func(next gollem.ToolHandler) gollem.ToolHandler {
        return func(ctx context.Context, req *gollem.ToolExecRequest) (*gollem.ToolExecResponse, error) {
            fmt.Printf("⚡ Executing tool: %s with args: %v\n", req.Tool.Name, req.Tool.Arguments)

            // Implement access control
            if !isToolAllowed(req.Tool.Name) {
                return &gollem.ToolExecResponse{
                    Error: fmt.Errorf("tool %s not allowed", req.Tool.Name),
                }, nil
            }

            // Execute tool
            resp, err := next(ctx, req)

            // Handle response
            if resp.Error != nil {
                fmt.Printf("❌ Tool %s failed: %v\n", req.Tool.Name, resp.Error)

                // Decide whether to continue or abort
                if isCriticalTool(req.Tool.Name) {
                    return resp, err // Abort execution
                }
            } else {
                fmt.Printf("✅ Tool %s completed: %v\n", req.Tool.Name, resp.Result)
            }

            return resp, err
        }
    }),
)
```


## SubAgents

SubAgents allow a parent agent to delegate tasks to specialized child agents. SubAgents implement the `Tool` interface, so they can be invoked by the LLM just like regular tools.

### Basic Usage (Default Mode)

In default mode, the subagent accepts a single `query` parameter:

```go
// Create a specialized agent for code review
reviewAgent := gollem.New(reviewClient,
    gollem.WithSystemPrompt("You are an expert code reviewer."),
)

// Wrap it as a SubAgent
reviewSubagent := gollem.NewSubAgent(
    "code_reviewer",
    "Reviews code for best practices and potential issues",
    reviewAgent,
)

// Add to parent agent
parentAgent := gollem.New(client,
    gollem.WithSubAgents(reviewSubagent),
)

// LLM can now call: code_reviewer(query: "Review this function for security issues...")
```

### Template Mode

For more structured inputs, use `PromptTemplate` to define custom parameters:

```go
// Create a prompt template with custom parameters
template, err := gollem.NewPromptTemplate(
    `Analyze the following code focusing on {{.focus}}:

{{.code}}

Provide detailed feedback.`,
    map[string]*gollem.Parameter{
        "code":  {Type: gollem.TypeString, Description: "Code to analyze", Required: true},
        "focus": {Type: gollem.TypeString, Description: "Focus area (security, performance, etc.)", Required: true},
    },
)
if err != nil {
    panic(err)
}

// Create subagent with custom template
analyzer := gollem.NewSubAgent(
    "code_analyzer",
    "Analyzes code with specified focus area",
    analyzerAgent,
    gollem.WithPromptTemplate(template),
)

// LLM can now call: code_analyzer(code: "func main()...", focus: "security")
```

### Testing Templates

You can test your templates independently using the `Render` method:

```go
// Create template
template, err := gollem.NewPromptTemplate(
    "Hello, {{.name}}! Your task is: {{.task}}",
    map[string]*gollem.Parameter{
        "name": {Type: gollem.TypeString, Description: "User name", Required: true},
        "task": {Type: gollem.TypeString, Description: "Task description", Required: true},
    },
)
if err != nil {
    panic(err)
}

// Test rendering
result, err := template.Render(map[string]any{
    "name": "Alice",
    "task": "Review the code",
})
// result: "Hello, Alice! Your task is: Review the code"

// Inspect parameters
params := template.Parameters()
fmt.Printf("Required: %v\n", params["name"].Required) // true
```

### Default Template

Use `DefaultPromptTemplate()` to get the standard query-based template:

```go
template := gollem.DefaultPromptTemplate()
// Equivalent to: NewPromptTemplate("{{.query}}", map[string]*Parameter{"query": {...}})
```

### Nested SubAgents

SubAgents can have their own subagents for complex hierarchical workflows:

```go
// Level 3: Specialized agents
securityAgent := gollem.New(client, gollem.WithSystemPrompt("Security expert"))
perfAgent := gollem.New(client, gollem.WithSystemPrompt("Performance expert"))

securitySubagent := gollem.NewSubAgent("security_check", "Security analysis", securityAgent)
perfSubagent := gollem.NewSubAgent("perf_check", "Performance analysis", perfAgent)

// Level 2: Code review agent with specialized subagents
reviewAgent := gollem.New(client,
    gollem.WithSystemPrompt("Code reviewer"),
    gollem.WithSubAgents(securitySubagent, perfSubagent),
)
reviewSubagent := gollem.NewSubAgent("code_review", "Comprehensive code review", reviewAgent)

// Level 1: Main agent
mainAgent := gollem.New(client,
    gollem.WithSubAgents(reviewSubagent),
)
```

## Next Steps

- Learn about [MCP server integration](mcp.md) for external tool integration
- Check out [practical examples](examples.md) of tool usage
- Review the [getting started guide](getting-started.md) for basic usage
- Understand [history management](history.md) for conversation context
- Explore the [complete documentation](README.md)
