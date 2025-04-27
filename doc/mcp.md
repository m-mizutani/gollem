# MCP Server Integration

gollam supports integration with MCP (Model Context Protocol) servers, allowing you to extend LLM capabilities with external tools and resources.

## What is MCP?

MCP is a protocol that enables LLMs to interact with external tools and resources through a standardized interface. It provides a way to:

- Define and expose custom tools
- Manage external resources
- Customize LLM prompts

## Connecting to an MCP Server

To connect your gollam application to an MCP server, you can use either HTTP SSE or stdio transport:

```go
// Using HTTP SSE transport
s := gollam.New(client,
    gollam.WithMCPviaSSE("http://localhost:8080"),
)

// Using stdio transport
s := gollam.New(client,
    gollam.WithMCPviaStdio("/path/to/mcp/server", []string{"--arg1", "value1"}),
)
```

## Next Steps

- Learn more about [tool creation](tools.md)
- Check out [practical examples](examples.md) of MCP integration 