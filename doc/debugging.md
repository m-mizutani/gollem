# Debugging

## Logging LLM Requests and Responses

You can enable detailed logging using the `trace/logger` package, which implements the `trace.Handler` interface and outputs structured logs via `slog.Logger`.

### Setup

```go
import (
    "log/slog"
    "os"

    "github.com/m-mizutani/gollem"
    tracelogger "github.com/m-mizutani/gollem/trace/logger"
)

// Enable all events (default)
handler := tracelogger.New()

// Or enable specific events only
handler := tracelogger.New(
    tracelogger.WithEvents(tracelogger.LLMRequest, tracelogger.LLMResponse),
)

// Use a custom logger
handler := tracelogger.New(
    tracelogger.WithLogger(slog.New(slog.NewJSONHandler(os.Stdout, nil))),
)

// Pass to agent
agent := gollem.New(client,
    gollem.WithTraceHandler(handler),
    gollem.WithTools(tools...),
)
```

### Available Events

| Event | Description |
|---|---|
| `AgentExec` | Agent execution start/end |
| `LLMRequest` | LLM request prompts |
| `LLMResponse` | LLM response content |
| `ToolExec` | Tool execution start/end |
| `SubAgent` | Sub-agent execution start/end |
| `CustomEvent` | Custom trace events |

By default, all events are enabled. Use `WithEvents()` to enable only specific events.

### Log Output Format

Logs are structured via `slog`:

```json
{
  "level": "INFO",
  "msg": "llm_call_end",
  "elapsed_ms": 1234,
  "texts": ["Generated response text"],
  "function_calls": [
    {
      "name": "search_function",
      "args": {"query": "example"}
    }
  ]
}
```

### Benefits

- **Debugging**: Track exact prompts and responses during development
- **Monitoring**: Observe token usage and response patterns
- **Audit**: Log tool calls and function executions
- **Performance**: Analyze response times and token efficiency
- **Troubleshooting**: Capture complete interaction context for issue resolution

## Next Steps

- Learn about [tracing](tracing.md) for structured execution observability
- Review [LLM provider configuration](llm.md) for provider-specific settings
