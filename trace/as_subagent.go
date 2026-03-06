package trace

import "context"

// AsSubAgent creates a Handler that wraps a parent Handler,
// mapping StartAgentExecute/EndAgentExecute to StartChildAgent/EndChildAgent.
//
// This is useful when running multiple gollem Agents that should appear
// as child agents in a single trace tree. The child agents are recorded
// with SpanKindAgentExecute, distinguishing them from gollem-internal
// sub-agents (SpanKindSubAgent).
//
// Pass the returned Handler to gollem.WithTrace() when creating the child agent.
//
// Example:
//
//	recorder := trace.New(trace.WithRepository(repo))
//	// ... recorder is used as trace handler for the root agent ...
//
//	// For each child agent:
//	childHandler := trace.AsSubAgent(recorder, "task-1")
//	childAgent := gollem.New(llmClient,
//	    gollem.WithTrace(childHandler),
//	    gollem.WithToolSets(tools...),
//	)
//	resp, err := childAgent.Execute(ctx, inputs...)
func AsSubAgent(parent Handler, name string) Handler {
	return &asSubAgentHandler{parent: parent, name: name}
}

type asSubAgentHandler struct {
	parent Handler
	name   string
}

func (h *asSubAgentHandler) StartAgentExecute(ctx context.Context) context.Context {
	return h.parent.StartChildAgent(ctx, h.name)
}

func (h *asSubAgentHandler) EndAgentExecute(ctx context.Context, err error) {
	h.parent.EndChildAgent(ctx, err)
}

func (h *asSubAgentHandler) StartLLMCall(ctx context.Context) context.Context {
	return h.parent.StartLLMCall(ctx)
}

func (h *asSubAgentHandler) EndLLMCall(ctx context.Context, data *LLMCallData, err error) {
	h.parent.EndLLMCall(ctx, data, err)
}

func (h *asSubAgentHandler) StartToolExec(ctx context.Context, toolName string, args map[string]any) context.Context {
	return h.parent.StartToolExec(ctx, toolName, args)
}

func (h *asSubAgentHandler) EndToolExec(ctx context.Context, result map[string]any, err error) {
	h.parent.EndToolExec(ctx, result, err)
}

func (h *asSubAgentHandler) StartSubAgent(ctx context.Context, name string) context.Context {
	return h.parent.StartSubAgent(ctx, name)
}

func (h *asSubAgentHandler) EndSubAgent(ctx context.Context, err error) {
	h.parent.EndSubAgent(ctx, err)
}

func (h *asSubAgentHandler) StartChildAgent(ctx context.Context, name string) context.Context {
	return h.parent.StartChildAgent(ctx, name)
}

func (h *asSubAgentHandler) EndChildAgent(ctx context.Context, err error) {
	h.parent.EndChildAgent(ctx, err)
}

func (h *asSubAgentHandler) AddEvent(ctx context.Context, kind string, data any) {
	h.parent.AddEvent(ctx, kind, data)
}

func (h *asSubAgentHandler) Finish(_ context.Context) error {
	// no-op: parent owns the Finish lifecycle
	return nil
}
