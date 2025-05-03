# MCP Server Integration

gollem supports integration with MCP (Model Context Protocol) servers, allowing you to extend LLM capabilities with external tools and resources.

## What is MCP?

MCP is a protocol that enables LLMs to interact with external tools and resources through a standardized interface. It provides a way to:

- Define and expose custom tools
- Manage external resources
- Customize LLM prompts

## Connecting to an MCP Server

To connect your gollem application to an MCP server, you can use either HTTP SSE or stdio transport:

```go
// Using HTTP SSE transport
mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8080")
if err != nil {
    panic(err)
}
defer mcpClient.Close()

s := gollem.New(client,
    gollem.WithToolSets(mcpClient),
)

// Using stdio transport
mcpClient, err := mcp.NewStdio(context.Background(), "/path/to/mcp/server", []string{"--arg1", "value1"})
if err != nil {
    panic(err)
}
defer mcpClient.Close()

s := gollem.New(client,
    gollem.WithToolSets(mcpClient),
)
```

## Options

1. **Environment Variables**: Set environment variables for the MCP client
```go
mcpClient, err := mcp.NewStdio(context.Background(), "/path/to/mcp/server", []string{},
    mcp.WithEnvVars([]string{"MCP_ENV=test"}),
)
```

2. **HTTP Headers**: Set custom HTTP headers for SSE transport
```go
mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8080",
    mcp.WithHeaders(map[string]string{
        "Authorization": "Bearer token",
    }),
)
```

## Next Steps

- Learn more about [tool creation](tools.md)
- Check out [practical examples](examples.md) of MCP integration
- Review the [getting started guide](getting-started.md) for basic usage
- Understand [history management](history.md) for conversation context
- Explore the [complete documentation](README.md)