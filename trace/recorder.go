package trace

import (
	"context"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Option is a functional option for configuring a Recorder.
type Option func(*Recorder)

// WithRepository sets the repository for persisting trace data.
func WithRepository(repo Repository) Option {
	return func(r *Recorder) {
		r.repo = repo
	}
}

// WithMetadata sets the metadata for the trace.
func WithMetadata(meta TraceMetadata) Option {
	return func(r *Recorder) {
		r.metadata = meta
	}
}

// WithTraceID sets a custom trace ID.
// If not set or set to an empty string, a UUID v7 is generated automatically.
func WithTraceID(id string) Option {
	return func(r *Recorder) {
		r.traceID = id
	}
}

// Recorder collects tracing data during agent execution into an in-memory Trace structure.
// It implements the Handler interface and provides access to the collected Trace via Trace().
type Recorder struct {
	trace    *Trace
	mu       sync.Mutex
	repo     Repository
	metadata TraceMetadata
	traceID  string
	logger   *slog.Logger
}

// New creates a new Recorder with the given options.
func New(opts ...Option) *Recorder {
	r := &Recorder{
		logger: slog.New(slog.DiscardHandler),
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// context key types
type handlerKey struct{}
type currentSpanKey struct{}

// WithHandler stores the Handler in the context.
func WithHandler(ctx context.Context, h Handler) context.Context {
	return context.WithValue(ctx, handlerKey{}, h)
}

// HandlerFrom retrieves the Handler from the context. Returns nil if not set.
func HandlerFrom(ctx context.Context) Handler {
	h, _ := ctx.Value(handlerKey{}).(Handler)
	return h
}

// withCurrentSpan stores the current span in the context.
func withCurrentSpan(ctx context.Context, span *Span) context.Context {
	return context.WithValue(ctx, currentSpanKey{}, span)
}

// currentSpanFrom retrieves the current span from the context. Returns nil if not set.
func currentSpanFrom(ctx context.Context) *Span {
	s, _ := ctx.Value(currentSpanKey{}).(*Span)
	return s
}

// newSpanID generates a unique span ID.
func newSpanID() string {
	return uuid.New().String()
}

// StartAgentExecute starts the root agent_execute span.
func (r *Recorder) StartAgentExecute(ctx context.Context) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	spanID := newSpanID()

	span := &Span{
		SpanID:    spanID,
		Kind:      SpanKindAgentExecute,
		Name:      "agent_execute",
		StartedAt: now,
		Status:    SpanStatusOK,
	}

	traceID := r.traceID
	if traceID == "" {
		traceID = uuid.Must(uuid.NewV7()).String()
	}

	r.trace = &Trace{
		TraceID:   traceID,
		RootSpan:  span,
		Metadata:  r.metadata,
		StartedAt: now,
	}

	return withCurrentSpan(ctx, span)
}

// EndAgentExecute ends the root agent_execute span.
func (r *Recorder) EndAgentExecute(ctx context.Context, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	span := currentSpanFrom(ctx)
	if span == nil {
		return
	}

	now := time.Now()
	span.EndedAt = now
	span.Duration = now.Sub(span.StartedAt)

	if err != nil {
		span.Status = SpanStatusError
		span.Error = err.Error()
	}

	if r.trace != nil {
		r.trace.EndedAt = now
	}
}

// StartLLMCall starts an llm_call span as a child of the current span.
func (r *Recorder) StartLLMCall(ctx context.Context) context.Context {
	return r.startChildSpan(ctx, SpanKindLLMCall, "llm_call")
}

// EndLLMCall ends the llm_call span with the given data.
func (r *Recorder) EndLLMCall(ctx context.Context, data *LLMCallData, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	span := currentSpanFrom(ctx)
	if span == nil || span.Kind != SpanKindLLMCall {
		return
	}

	now := time.Now()
	span.EndedAt = now
	span.Duration = now.Sub(span.StartedAt)
	span.LLMCall = data

	if err != nil {
		span.Status = SpanStatusError
		span.Error = err.Error()
	}
}

// StartToolExec starts a tool_exec span as a child of the current span.
func (r *Recorder) StartToolExec(ctx context.Context, toolName string, args map[string]any) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()

	parent := currentSpanFrom(ctx)
	if parent == nil {
		return ctx
	}

	spanID := newSpanID()
	span := &Span{
		SpanID:    spanID,
		ParentID:  parent.SpanID,
		Kind:      SpanKindToolExec,
		Name:      toolName,
		StartedAt: time.Now(),
		Status:    SpanStatusOK,
		ToolExec: &ToolExecData{
			ToolName: toolName,
			Args:     args,
		},
	}

	parent.Children = append(parent.Children, span)
	return withCurrentSpan(ctx, span)
}

// EndToolExec ends the tool_exec span with the result.
func (r *Recorder) EndToolExec(ctx context.Context, result map[string]any, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	span := currentSpanFrom(ctx)
	if span == nil || span.Kind != SpanKindToolExec {
		return
	}

	now := time.Now()
	span.EndedAt = now
	span.Duration = now.Sub(span.StartedAt)

	if span.ToolExec != nil {
		span.ToolExec.Result = result
		if err != nil {
			span.ToolExec.Error = err.Error()
		}
	}

	if err != nil {
		span.Status = SpanStatusError
		span.Error = err.Error()
	}
}

// StartSubAgent starts a sub_agent span as a child of the current span.
func (r *Recorder) StartSubAgent(ctx context.Context, name string) context.Context {
	return r.startChildSpan(ctx, SpanKindSubAgent, name)
}

// EndSubAgent ends the sub_agent span.
func (r *Recorder) EndSubAgent(ctx context.Context, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	span := currentSpanFrom(ctx)
	if span == nil || span.Kind != SpanKindSubAgent {
		return
	}

	now := time.Now()
	span.EndedAt = now
	span.Duration = now.Sub(span.StartedAt)

	if err != nil {
		span.Status = SpanStatusError
		span.Error = err.Error()
	}
}

// AddEvent adds an event span as a child of the current span.
// kind is an arbitrary string defined by the Strategy implementation.
// data is any JSON-serializable value defined by the Strategy.
func (r *Recorder) AddEvent(ctx context.Context, kind string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()

	parent := currentSpanFrom(ctx)
	if parent == nil {
		return
	}

	now := time.Now()
	spanID := newSpanID()

	span := &Span{
		SpanID:    spanID,
		ParentID:  parent.SpanID,
		Kind:      SpanKindEvent,
		Name:      kind,
		StartedAt: now,
		EndedAt:   now,
		Duration:  0,
		Status:    SpanStatusOK,
		Event: &EventData{
			Kind: kind,
			Data: data,
		},
	}

	parent.Children = append(parent.Children, span)
}

// Finish completes the trace and persists it to the Repository.
func (r *Recorder) Finish(ctx context.Context) error {
	r.mu.Lock()
	trace := r.trace
	repo := r.repo
	r.mu.Unlock()

	if trace == nil || repo == nil {
		return nil
	}

	if err := repo.Save(ctx, trace); err != nil {
		return err
	}

	return nil
}

// Trace returns the current trace data. Returns nil if no trace is active.
func (r *Recorder) Trace() *Trace {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.trace
}

// startChildSpan is a helper to start a child span of the current span.
func (r *Recorder) startChildSpan(ctx context.Context, kind SpanKind, name string) context.Context {
	r.mu.Lock()
	defer r.mu.Unlock()

	parent := currentSpanFrom(ctx)
	if parent == nil {
		return ctx
	}

	spanID := newSpanID()
	span := &Span{
		SpanID:    spanID,
		ParentID:  parent.SpanID,
		Kind:      kind,
		Name:      name,
		StartedAt: time.Now(),
		Status:    SpanStatusOK,
	}

	parent.Children = append(parent.Children, span)
	return withCurrentSpan(ctx, span)
}
