# Tracing

gollem provides a tracing system for observing agent execution. The `trace` package follows the `slog.Handler` pattern: a `Handler` interface defines the contract, and concrete implementations provide different backends.

## Overview

During agent execution, gollem emits lifecycle events:

- **Agent Execute**: The root span wrapping an `agent.Execute()` call
- **LLM Call**: Each request/response to the LLM provider
- **Tool Exec**: Each tool invocation with arguments and results
- **Sub Agent**: Child agent invocations via `SubAgent`
- **Event**: Custom events emitted by strategies (e.g., plan creation, reflection)

These events form a tree structure rooted at the agent execution span.

## Handler Interface

All trace backends implement the `trace.Handler` interface:

```go
type Handler interface {
    StartAgentExecute(ctx context.Context) context.Context
    EndAgentExecute(ctx context.Context, err error)

    StartLLMCall(ctx context.Context) context.Context
    EndLLMCall(ctx context.Context, data *LLMCallData, err error)

    StartToolExec(ctx context.Context, toolName string, args map[string]any) context.Context
    EndToolExec(ctx context.Context, result map[string]any, err error)

    StartSubAgent(ctx context.Context, name string) context.Context
    EndSubAgent(ctx context.Context, err error)

    AddEvent(ctx context.Context, kind string, data any)

    Finish(ctx context.Context) error
}
```

Each `Start*` method returns a new `context.Context` that carries span state. The corresponding `End*` method receives this context to close the span. `Finish` is called after execution completes to perform any final cleanup (e.g., persisting data).

## Enabling Tracing

Pass a `Handler` to the agent via `gollem.WithTrace()`:

```go
agent := gollem.New(client, gollem.WithTrace(handler))
```

## Built-in Handlers

### Recorder (`trace.New()`)

The in-memory recorder collects trace data into a tree of `Span` structs. It is useful for debugging, testing, and persisting traces.

```go
rec := trace.New()
agent := gollem.New(client, gollem.WithTrace(rec))

result, err := agent.Execute(ctx, gollem.Text("Hello"))

// Access trace data
tr := rec.Trace()
fmt.Printf("Trace ID: %s\n", tr.TraceID)
fmt.Printf("Root span: %s (%s)\n", tr.RootSpan.Name, tr.RootSpan.Kind)
for _, child := range tr.RootSpan.Children {
    fmt.Printf("  - %s (%s) duration=%s\n", child.Name, child.Kind, child.Duration)
}
```

#### Persisting Traces with Repository

Traces can be saved automatically by providing a `Repository`:

```go
rec := trace.New(
    trace.WithRepository(trace.NewFileRepository("./traces")),
    trace.WithMetadata(trace.TraceMetadata{
        Model:    "gpt-4",
        Strategy: "planexec",
        Labels:   map[string]string{"env": "production", "user": "alice"},
    }),
)

agent := gollem.New(client, gollem.WithTrace(rec))
result, err := agent.Execute(ctx, gollem.Text("Analyze the logs"))
// Trace is automatically saved to ./traces/{trace_id}.json on Finish
```

`FileRepository` writes each trace as a JSON file. You can implement the `Repository` interface for custom storage (database, cloud storage, etc.):

```go
type Repository interface {
    Save(ctx context.Context, trace *Trace) error
}
```

#### Trace Data Structure

The recorded trace has this structure:

```
Trace
├── TraceID
├── Metadata (model, strategy, labels)
├── StartedAt / EndedAt
└── RootSpan (agent_execute)
    ├── LLM Call span (input/output tokens, model, request/response)
    ├── Tool Exec span (tool name, args, result)
    ├── Sub Agent span
    │   ├── LLM Call span
    │   └── Tool Exec span
    └── Event span (strategy-defined events)
```

Each span contains:
- `SpanID`, `ParentID` for the tree structure
- `Kind`: `agent_execute`, `llm_call`, `tool_exec`, `sub_agent`, `event`
- `StartedAt`, `EndedAt`, `Duration` for timing
- `Status`: `ok` or `error`
- Kind-specific data (`LLMCallData`, `ToolExecData`, `EventData`)

### OpenTelemetry Handler (`trace/otel`)

The `trace/otel` package bridges gollem's trace events to OpenTelemetry spans. This integrates with any OTel-compatible backend such as Jaeger, Zipkin, or OTLP collectors.

```go
import traceOtel "github.com/m-mizutani/gollem/trace/otel"

// Uses the global TracerProvider (set by your OTel SDK setup)
agent := gollem.New(client, gollem.WithTrace(traceOtel.New()))
```

#### Explicit TracerProvider

If you manage multiple `TracerProvider` instances or want to avoid the global:

```go
import (
    traceOtel "github.com/m-mizutani/gollem/trace/otel"
    sdkTrace "go.opentelemetry.io/otel/sdk/trace"
)

tp := sdkTrace.NewTracerProvider(
    sdkTrace.WithBatcher(exporter),
)

agent := gollem.New(client, gollem.WithTrace(
    traceOtel.New(traceOtel.WithTracerProvider(tp)),
))
```

#### Span Mapping

gollem events map to OTel spans as follows:

| gollem Event | OTel Span Name | Span Kind | Attributes |
|---|---|---|---|
| Agent Execute | `agent_execute` | Internal | - |
| LLM Call | `llm_call` | Client | `llm.model`, `llm.input_tokens`, `llm.output_tokens` |
| Tool Exec | `tool:{name}` | Internal | `tool.name`, `tool.args` |
| Sub Agent | `sub_agent:{name}` | Internal | - |
| Event | _(added as span event)_ | - | `event.data` |

Errors are recorded via `span.RecordError()`. Parent-child relationships are preserved through context propagation.

### Multi Handler (`trace.Multi()`)

`Multi` fans out events to multiple handlers. Each handler receives its own isolated context, so multiple `Recorder` instances or any combination of handlers work without interference.

```go
import (
    "github.com/m-mizutani/gollem/trace"
    traceOtel "github.com/m-mizutani/gollem/trace/otel"
)

// Record in-memory AND export to OTel
rec := trace.New(trace.WithRepository(trace.NewFileRepository("./traces")))
otelHandler := traceOtel.New()

agent := gollem.New(client,
    gollem.WithTrace(trace.Multi(rec, otelHandler)),
)
```

`Finish` collects errors from all handlers using `errors.Join`.

## Implementing a Custom Handler

To create your own trace backend, implement the `trace.Handler` interface:

```go
type myHandler struct{}

func (h *myHandler) StartAgentExecute(ctx context.Context) context.Context {
    log.Println("agent execution started")
    return ctx
}

func (h *myHandler) EndAgentExecute(ctx context.Context, err error) {
    if err != nil {
        log.Printf("agent execution failed: %v", err)
    } else {
        log.Println("agent execution completed")
    }
}

func (h *myHandler) StartLLMCall(ctx context.Context) context.Context { return ctx }
func (h *myHandler) EndLLMCall(ctx context.Context, data *trace.LLMCallData, err error) {
    if data != nil {
        log.Printf("LLM call: model=%s tokens=%d/%d", data.Model, data.InputTokens, data.OutputTokens)
    }
}

func (h *myHandler) StartToolExec(ctx context.Context, toolName string, args map[string]any) context.Context {
    return ctx
}
func (h *myHandler) EndToolExec(ctx context.Context, result map[string]any, err error) {}

func (h *myHandler) StartSubAgent(ctx context.Context, name string) context.Context { return ctx }
func (h *myHandler) EndSubAgent(ctx context.Context, err error) {}

func (h *myHandler) AddEvent(ctx context.Context, kind string, data any) {}

func (h *myHandler) Finish(ctx context.Context) error { return nil }
```

Use `Start*` methods to store span state in the context (via `context.WithValue`) and retrieve it in the corresponding `End*` methods.
