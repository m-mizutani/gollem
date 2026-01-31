package trace

import "context"

// Handler is the interface for trace backends.
// Implementations receive lifecycle events during agent execution
// and can record, export, or forward them as needed.
type Handler interface {
	// StartAgentExecute starts the root agent execution span.
	StartAgentExecute(ctx context.Context) context.Context
	// EndAgentExecute ends the root agent execution span.
	EndAgentExecute(ctx context.Context, err error)

	// StartLLMCall starts an LLM call span.
	StartLLMCall(ctx context.Context) context.Context
	// EndLLMCall ends an LLM call span with the given data.
	EndLLMCall(ctx context.Context, data *LLMCallData, err error)

	// StartToolExec starts a tool execution span.
	StartToolExec(ctx context.Context, toolName string, args map[string]any) context.Context
	// EndToolExec ends a tool execution span with the result.
	EndToolExec(ctx context.Context, result map[string]any, err error)

	// StartSubAgent starts a sub-agent span.
	StartSubAgent(ctx context.Context, name string) context.Context
	// EndSubAgent ends a sub-agent span.
	EndSubAgent(ctx context.Context, err error)

	// AddEvent adds an event to the current span.
	AddEvent(ctx context.Context, kind string, data any)

	// Finish completes the trace and performs any final operations.
	Finish(ctx context.Context) error
}
