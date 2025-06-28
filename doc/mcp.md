# MCP Server Integration

gollem supports integration with MCP (Model Context Protocol) servers, allowing you to extend LLM capabilities with external tools and resources.

## What is MCP?

MCP is a protocol that enables LLMs to interact with external tools and resources through a standardized interface. It provides a way to:

- Define and expose custom tools
- Manage external resources
- Customize LLM prompts

## Transport Options

gollem supports three types of MCP transport:

1. **Stdio Transport** (`NewStdio`) - For local MCP servers running as child processes
2. **StreamableHTTP Transport** (`NewStreamableHTTP`) - For remote MCP servers via HTTP streaming
3. **SSE Transport** (`NewSSE`) - ⚠️ **DEPRECATED** - For remote MCP servers via Server-Sent Events

### Stdio Transport (Recommended for Local Servers)

Use this for MCP servers that run as local executables:

```go
mcpClient, err := mcp.NewStdio(context.Background(), "/path/to/mcp/server", []string{"--arg1", "value1"},
    mcp.WithStdioClientInfo("my-app", "1.0.0"),
    mcp.WithEnvVars([]string{"MCP_ENV=production"}))
if err != nil {
    panic(err)
}
defer mcpClient.Close()
```

### StreamableHTTP Transport (Recommended for Remote Servers)

Use this for remote MCP servers accessible via HTTP streaming:

```go
mcpClient, err := mcp.NewStreamableHTTP(context.Background(), "http://localhost:8080",
    mcp.WithStreamableHTTPClientInfo("my-app", "1.0.0"),
    mcp.WithStreamableHTTPHeaders(map[string]string{
        "Authorization": "Bearer token",
    }))
if err != nil {
    panic(err)
}
defer mcpClient.Close()
```

### SSE Transport (Deprecated)

⚠️ **DEPRECATED**: SSE transport is deprecated and will be removed in future versions. Use StreamableHTTP transport instead.

```go
// DEPRECATED: Use NewStreamableHTTP instead
mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8081",
    mcp.WithSSEClientInfo("my-app", "1.0.0"),
    mcp.WithSSEHeaders(map[string]string{
        "Authorization": "Bearer token",
    }))
if err != nil {
    panic(err)
}
defer mcpClient.Close()
```

## Complete Example

Here's a complete example using multiple transport types:

```go
ctx := context.Background()

// Create OpenAI client
client, err := openai.New(ctx, os.Getenv("OPENAI_API_KEY"))
if err != nil {
    panic(err)
}

// Local MCP server via Stdio
mcpLocal, err := mcp.NewStdio(ctx, "./mcp-server", []string{},
    mcp.WithStdioClientInfo("gollem-app", "1.0.0"),
    mcp.WithEnvVars([]string{"MCP_ENV=development"}))
if err != nil {
    panic(err)
}
defer mcpLocal.Close()

// Remote MCP server via StreamableHTTP (recommended)
mcpRemote, err := mcp.NewStreamableHTTP(ctx, "http://localhost:8080",
    mcp.WithStreamableHTTPClientInfo("gollem-remote", "1.0.0"))
if err != nil {
    fmt.Printf("⚠️  Could not connect to HTTP MCP server: %v\n", err)
    mcpRemote = nil
}
if mcpRemote != nil {
    defer mcpRemote.Close()
}

// Create gollem agent with MCP tools
var toolSets []gollem.ToolSet
toolSets = append(toolSets, mcpLocal)
if mcpRemote != nil {
    toolSets = append(toolSets, mcpRemote)
}

agent := gollem.New(client,
    gollem.WithToolSets(toolSets...),
    gollem.WithMessageHook(func(ctx context.Context, msg string) error {
        fmt.Printf("🤖 %s\n", msg)
        return nil
    }),
    gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.FunctionCall) error {
        fmt.Printf("⚡ Using MCP tool: %s\n", tool.Name)
        return nil
    }),
)

// Execute with MCP tools
err = agent.Execute(ctx, "Use the available MCP tools to help me with my task")
if err != nil {
    panic(err)
}
```

## Configuration Options

### Client Information

Customize client name and version for all transport types (defaults to "gollem" and ""):

```go
// For Stdio transport
mcpClient, err := mcp.NewStdio(context.Background(), "/path/to/mcp/server", []string{},
    mcp.WithStdioClientInfo("my-application", "1.2.3"),
)

// For StreamableHTTP transport
mcpClient, err := mcp.NewStreamableHTTP(context.Background(), "http://localhost:8080",
    mcp.WithStreamableHTTPClientInfo("my-web-app", "2.0.0"),
)

// For SSE transport (deprecated)
mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8081",
    mcp.WithSSEClientInfo("my-legacy-app", "1.5.0"), // DEPRECATED
)
```

### Environment Variables (Stdio Only)

Set environment variables for the MCP client:

```go
mcpClient, err := mcp.NewStdio(context.Background(), "/path/to/mcp/server", []string{},
    mcp.WithEnvVars([]string{"MCP_ENV=test", "DEBUG=true"}),
)
```

### HTTP Headers (HTTP Transports)

Set custom HTTP headers for HTTP-based transports:

```go
// For StreamableHTTP transport
mcpClient, err := mcp.NewStreamableHTTP(context.Background(), "http://localhost:8080",
    mcp.WithStreamableHTTPHeaders(map[string]string{
        "Authorization": "Bearer token",
        "User-Agent": "MyApp/1.0",
    }),
)

// For SSE transport (deprecated)
mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8081",
    mcp.WithSSEHeaders(map[string]string{ // DEPRECATED
        "Authorization": "Bearer token",
        "User-Agent": "MyApp/1.0",
    }),
)
```

### Custom HTTP Client

Provide custom HTTP client for HTTP-based transports:

```go
customClient := &http.Client{
    Timeout: 30 * time.Second,
}

// For StreamableHTTP transport
mcpClient, err := mcp.NewStreamableHTTP(context.Background(), "http://localhost:8080",
    mcp.WithStreamableHTTPClient(customClient),
)

// For SSE transport (deprecated)
mcpClient, err := mcp.NewSSE(context.Background(), "http://localhost:8081",
    mcp.WithSSEClient(customClient), // DEPRECATED
)
```

### Combining Options

You can combine multiple options:

```go
mcpClient, err := mcp.NewStdio(context.Background(), "/path/to/mcp/server", []string{},
    mcp.WithStdioClientInfo("my-app", "1.0.0"),
    mcp.WithEnvVars([]string{"MCP_ENV=production", "LOG_LEVEL=info"}),
)
```

## Next Steps

- Learn more about [tool creation](tools.md)
- Check out [practical examples](examples.md) of MCP integration
- Review the [getting started guide](getting-started.md) for basic usage
- Understand [history management](history.md) for conversation context
- Explore the [complete documentation](README.md)