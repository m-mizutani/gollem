package trace

import (
	"runtime"
	"time"
)

// SpanKind represents the type of a span.
type SpanKind string

const (
	SpanKindAgentExecute SpanKind = "agent_execute"
	SpanKindLLMCall      SpanKind = "llm_call"
	SpanKindToolExec     SpanKind = "tool_exec"
	SpanKindSubAgent     SpanKind = "sub_agent"
	SpanKindEvent        SpanKind = "event"
)

// SpanStatus represents the status of a span.
type SpanStatus string

const (
	SpanStatusOK    SpanStatus = "ok"
	SpanStatusError SpanStatus = "error"
)

// Trace represents the root tracing data for an agent execution.
type Trace struct {
	TraceID   string        `json:"trace_id"`
	RootSpan  *Span         `json:"root_span"`
	Metadata  TraceMetadata `json:"metadata"`
	StartedAt time.Time     `json:"started_at"`
	EndedAt   time.Time     `json:"ended_at"`
}

// TraceMetadata holds metadata for a trace.
type TraceMetadata struct {
	Model    string            `json:"model,omitempty"`
	Strategy string            `json:"strategy,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// StackFrame represents a single frame in a call stack trace.
type StackFrame struct {
	Function string `json:"function"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// Span represents a single unit of operation in the trace hierarchy.
type Span struct {
	SpanID     string        `json:"span_id"`
	ParentID   string        `json:"parent_id,omitempty"`
	Kind       SpanKind      `json:"kind"`
	Name       string        `json:"name"`
	StartedAt  time.Time     `json:"started_at"`
	EndedAt    time.Time     `json:"ended_at"`
	Duration   time.Duration `json:"duration"`
	Status     SpanStatus    `json:"status"`
	Error      string        `json:"error,omitempty"`
	Children   []*Span       `json:"children,omitempty"`
	StackTrace []StackFrame  `json:"stack_trace,omitempty"`

	// Kind-specific data (only one is non-nil based on Kind)
	LLMCall  *LLMCallData  `json:"llm_call,omitempty"`
	ToolExec *ToolExecData `json:"tool_exec,omitempty"`
	Event    *EventData    `json:"event,omitempty"`
}

// captureStackTrace captures the current call stack, skipping the specified
// number of frames plus internal frames. skip=0 captures from the caller of
// captureStackTrace.
func captureStackTrace(skip int) []StackFrame {
	const maxFrames = 32
	pcs := make([]uintptr, maxFrames)
	// skip+2: skip runtime.Callers itself and captureStackTrace
	n := runtime.Callers(skip+2, pcs)
	if n == 0 {
		return nil
	}

	frames := runtime.CallersFrames(pcs[:n])
	var result []StackFrame
	for {
		frame, more := frames.Next()
		result = append(result, StackFrame{
			Function: frame.Function,
			File:     frame.File,
			Line:     frame.Line,
		})
		if !more {
			break
		}
	}
	return result
}
