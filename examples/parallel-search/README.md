# Parallel Search MCP

Search the web or fetch a page through gollem's Streamable HTTP MCP client.
This example discovers tools with `Specs` and executes them with `Run`, using
`https://search.parallel.ai/mcp` without a Parallel API key or an LLM.

From the repository root:

```sh
go run ./examples/parallel-search -objective "Find the official Go release notes" -query "Go official release notes"
go run ./examples/parallel-search -url https://go.dev/doc/devel/release
```

The output is the JSON representation of gollem's tool result, including the
search sources and excerpts or the fetched page content. Requests identify the
example as `gollem-parallel-search-example/1.0` and have a 60-second HTTP timeout
within a 90-second operation deadline.

The anonymous endpoint is free for exploration and light use, with rate limits.
See the [Parallel Search MCP documentation](https://docs.parallel.ai/integrations/mcp/search-mcp)
for current limits and tool parameters. Service error content is printed as
returned by gollem's MCP client; inspect it before using the results.

For an agent, pass the connected client to
`gollem.New(llmClient, gollem.WithToolSets(client))` before closing it, as in the
[existing MCP example](../mcp). An agent still needs its own LLM configuration;
model inference is separate from the free search endpoint. Keep the custom HTTP
transport from this example to send the User-Agent on all requests.
