# Practical Examples

This guide provides practical examples of using gollam in various scenarios.

## Calculator Tool

A more complex example using a calculator tool:

```go
type CalculatorTool struct{}

func (t *CalculatorTool) Spec() gollam.ToolSpec {
    return gollam.ToolSpec{
        Name:        "calculator",
        Description: "Performs basic arithmetic operations",
        Parameters: map[string]*gollam.Parameter{
            "operation": {
                Name:        "operation",
                Type:        gollam.TypeString,
                Description: "The operation to perform (add, subtract, multiply, divide)",
                Required:    true,
            },
            "a": {
                Name:        "a",
                Type:        gollam.TypeNumber,
                Description: "First number",
                Required:    true,
            },
            "b": {
                Name:        "b",
                Type:        gollam.TypeNumber,
                Description: "Second number",
                Required:    true,
            },
        },
    }
}

func (t *CalculatorTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
    // Validate operation
    op, ok := args["operation"].(string)
    if !ok {
        return nil, errors.New("operation must be a string")
    }
    if op != "add" && op != "subtract" && op != "multiply" && op != "divide" {
        return nil, errors.New("invalid operation: must be one of add, subtract, multiply, divide")
    }

    // Validate first number
    a, ok := args["a"].(float64)
    if !ok {
        return nil, errors.New("first number must be a number")
    }

    // Validate second number
    b, ok := args["b"].(float64)
    if !ok {
        return nil, errors.New("second number must be a number")
    }

    // Validate division by zero
    if op == "divide" && b == 0 {
        return nil, errors.New("division by zero is not allowed")
    }

    var result float64
    switch op {
    case "add":
        result = a + b
    case "subtract":
        result = a - b
    case "multiply":
        result = a * b
    case "divide":
        result = a / b
    }

    return map[string]any{
        "result": result,
    }, nil
}
```

## Weather Tool with MCP

An example of a weather tool using MCP:

```go
type WeatherTool struct{}

func (t *WeatherTool) Spec() gollam.ToolSpec {
    return gollam.ToolSpec{
        Name:        "weather",
        Description: "Gets weather information for a location",
        Parameters: map[string]*gollam.Parameter{
            "location": {
                Type:        gollam.TypeString,
                Description: "City name or coordinates",
                Required:    true,
            },
        },
    }
}

func (t *WeatherTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
    location, ok := args["location"].(string)
    if !ok {
        return nil, fmt.Errorf("location must be a string")
    }
    // Implement weather API call here
    return map[string]any{
        "temperature": 25.5,
        "condition":   "sunny",
    }, nil
}
```

## Best Practices

1. **Error Handling**: Always handle errors properly
2. **Type Safety**: Use appropriate types and validate input
3. **Documentation**: Document your tools and examples
4. **Testing**: Write tests for your tools
5. **Security**: Implement proper security measures

## Argument Validation

Here's an example of proper argument validation:

```go
func (t *CalculatorTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
    // Validate operation
    op, ok := args["operation"].(string)
    if !ok {
        return nil, errors.New("operation must be a string")
    }
    if op != "add" && op != "subtract" && op != "multiply" && op != "divide" {
        return nil, errors.New("invalid operation: must be one of add, subtract, multiply, divide")
    }

    // Validate first number
    a, ok := args["a"].(float64)
    if !ok {
        return nil, errors.New("first number must be a number")
    }

    // Validate second number
    b, ok := args["b"].(float64)
    if !ok {
        return nil, errors.New("second number must be a number")
    }

    // Validate division by zero
    if op == "divide" && b == 0 {
        return nil, errors.New("division by zero is not allowed")
    }

    var result float64
    switch op {
    case "add":
        result = a + b
    case "subtract":
        result = a - b
    case "multiply":
        result = a * b
    case "divide":
        result = a / b
    }

    return map[string]any{
        "result": result,
    }, nil
}
```

## Next Steps

- Learn more about [tool creation](tools.md)
- Explore [MCP server integration](mcp.md)
- Check out the [getting started guide](getting-started.md)
