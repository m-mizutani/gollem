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

To use tools with your LLM:

```go
s := gollem.New(client,
    gollem.WithTools(&HelloTool{}),
)
```

You can add multiple tools:

```go
s := gollem.New(client,
    gollem.WithTools(&HelloTool{}, &CalculatorTool{}, &WeatherTool{}),
)
```

## Best Practices

1. **Clear Descriptions**: Provide clear and concise descriptions for tools and parameters to help the LLM understand their purpose and usage
2. **Validate Input**: Always validate that parameters passed by the LLM match the specified types in your tool specification
3. **Error Handling**: When errors occur, return clear error messages that explain both what went wrong and how to fix it. The Error() message will be passed directly to the LLM, so include actionable guidance
4. **Nested Results**: For nested tool results with multiple levels of maps, always use `map[string]any`. Other types may cause errors with some LLM SDKs like Gemini

## Next Steps

- Learn about [MCP server integration](mcp.md) for external tool integration
- Check out [practical examples](examples.md) of tool usage
