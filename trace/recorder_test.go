package trace_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/m-mizutani/gollem/trace"
	"github.com/m-mizutani/gt"
)

func TestRecorderContextPropagation(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	// Before setting, RecorderFrom returns nil
	gt.Value(t, trace.HandlerFrom(ctx)).Nil()

	// After setting, RecorderFrom returns the recorder
	ctx = trace.WithHandler(ctx, rec)
	gt.Value(t, trace.HandlerFrom(ctx)).NotNil()
	gt.Equal[trace.Handler](t, trace.HandlerFrom(ctx), rec)
}

func TestRecorderAgentExecuteSpan(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()
	ctx = trace.WithHandler(ctx, rec)

	// Start agent execute
	agentCtx := rec.StartAgentExecute(ctx)

	// Current span should exist
	span := trace.CurrentSpanFrom(agentCtx)
	gt.Value(t, span).NotNil()
	gt.Equal(t, span.Kind, trace.SpanKindAgentExecute)
	gt.Equal(t, span.Name, "agent_execute")

	// Trace should be initialized
	tr := rec.Trace()
	gt.Value(t, tr).NotNil()
	gt.Equal(t, tr.RootSpan, span)

	// End agent execute
	rec.EndAgentExecute(agentCtx, nil)
	gt.Equal(t, span.Status, trace.SpanStatusOK)
	gt.B(t, span.Duration > 0).True()
}

func TestRecorderAgentExecuteWithError(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	rec.EndAgentExecute(agentCtx, errors.New("execution failed"))

	span := trace.CurrentSpanFrom(agentCtx)
	gt.Equal(t, span.Status, trace.SpanStatusError)
	gt.Equal(t, span.Error, "execution failed")
}

func TestRecorderLLMCallSpan(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	llmCtx := rec.StartLLMCall(agentCtx)

	llmSpan := trace.CurrentSpanFrom(llmCtx)
	gt.Value(t, llmSpan).NotNil()
	gt.Equal(t, llmSpan.Kind, trace.SpanKindLLMCall)

	rec.EndLLMCall(llmCtx, &trace.LLMCallData{
		InputTokens:  100,
		OutputTokens: 50,
		Model:        "test-model",
		Request: &trace.LLMRequest{
			SystemPrompt: "You are helpful",
			Messages:     []trace.Message{{Role: "user", Content: "hello"}},
		},
		Response: &trace.LLMResponse{
			Texts: []string{"Hi!"},
		},
	}, nil)

	gt.Equal(t, llmSpan.LLMCall.InputTokens, 100)
	gt.Equal(t, llmSpan.LLMCall.OutputTokens, 50)
	gt.Equal(t, llmSpan.LLMCall.Model, "test-model")
	gt.Equal(t, llmSpan.Status, trace.SpanStatusOK)

	// LLM span should be a child of the agent execute span
	rootSpan := rec.Trace().RootSpan
	gt.Equal(t, len(rootSpan.Children), 1)
	gt.Equal(t, rootSpan.Children[0], llmSpan)
}

func TestRecorderToolExecSpan(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	toolCtx := rec.StartToolExec(agentCtx, "search", map[string]any{"query": "test"})

	toolSpan := trace.CurrentSpanFrom(toolCtx)
	gt.Value(t, toolSpan).NotNil()
	gt.Equal(t, toolSpan.Kind, trace.SpanKindToolExec)
	gt.Equal(t, toolSpan.ToolExec.ToolName, "search")
	gt.Equal(t, toolSpan.ToolExec.Args["query"], "test")

	result := map[string]any{"found": true}
	rec.EndToolExec(toolCtx, result, nil)

	gt.Equal(t, toolSpan.ToolExec.Result["found"], true)
	gt.Equal(t, toolSpan.Status, trace.SpanStatusOK)
}

func TestRecorderToolExecWithError(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	toolCtx := rec.StartToolExec(agentCtx, "search", map[string]any{})

	rec.EndToolExec(toolCtx, nil, errors.New("tool failed"))

	toolSpan := trace.CurrentSpanFrom(toolCtx)
	gt.Equal(t, toolSpan.Status, trace.SpanStatusError)
	gt.Equal(t, toolSpan.ToolExec.Error, "tool failed")
}

func TestRecorderSubAgentSpan(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	subCtx := rec.StartSubAgent(agentCtx, "child_agent")

	subSpan := trace.CurrentSpanFrom(subCtx)
	gt.Value(t, subSpan).NotNil()
	gt.Equal(t, subSpan.Kind, trace.SpanKindSubAgent)
	gt.Equal(t, subSpan.Name, "child_agent")

	// LLM call inside sub agent
	llmCtx := rec.StartLLMCall(subCtx)
	rec.EndLLMCall(llmCtx, &trace.LLMCallData{
		InputTokens: 10,
	}, nil)

	// LLM span should be a child of the sub agent span
	gt.Equal(t, len(subSpan.Children), 1)
	gt.Equal(t, subSpan.Children[0].Kind, trace.SpanKindLLMCall)

	rec.EndSubAgent(subCtx, nil)
	gt.Equal(t, subSpan.Status, trace.SpanStatusOK)
}

func TestRecorderAddEvent(t *testing.T) {
	type testEvent struct {
		Goal string `json:"goal"`
	}

	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	rec.AddEvent(agentCtx, "plan_created", &testEvent{Goal: "implement feature"})

	rootSpan := rec.Trace().RootSpan
	gt.Equal(t, len(rootSpan.Children), 1)
	gt.Equal(t, rootSpan.Children[0].Kind, trace.SpanKindEvent)
	gt.Equal(t, rootSpan.Children[0].Event.Kind, "plan_created")

	data, ok := rootSpan.Children[0].Event.Data.(*testEvent)
	gt.B(t, ok).True()
	gt.Equal(t, data.Goal, "implement feature")

	// Event span should have zero duration
	gt.Equal(t, rootSpan.Children[0].StartedAt, rootSpan.Children[0].EndedAt)
}

func TestRecorderChildOrdering(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)

	// Add children in order: LLM, Event, Tool, Event, LLM
	llm1 := rec.StartLLMCall(agentCtx)
	rec.EndLLMCall(llm1, &trace.LLMCallData{InputTokens: 10}, nil)

	rec.AddEvent(agentCtx, "plan_created", nil)

	tool1 := rec.StartToolExec(agentCtx, "search", nil)
	rec.EndToolExec(tool1, nil, nil)

	rec.AddEvent(agentCtx, "task_completed", nil)

	llm2 := rec.StartLLMCall(agentCtx)
	rec.EndLLMCall(llm2, &trace.LLMCallData{InputTokens: 20}, nil)

	rootSpan := rec.Trace().RootSpan
	gt.Equal(t, len(rootSpan.Children), 5)
	gt.Equal(t, rootSpan.Children[0].Kind, trace.SpanKindLLMCall)
	gt.Equal(t, rootSpan.Children[1].Kind, trace.SpanKindEvent)
	gt.Equal(t, rootSpan.Children[1].Event.Kind, "plan_created")
	gt.Equal(t, rootSpan.Children[2].Kind, trace.SpanKindToolExec)
	gt.Equal(t, rootSpan.Children[3].Kind, trace.SpanKindEvent)
	gt.Equal(t, rootSpan.Children[3].Event.Kind, "task_completed")
	gt.Equal(t, rootSpan.Children[4].Kind, trace.SpanKindLLMCall)
}

func TestRecorderFinish(t *testing.T) {
	dir := t.TempDir()
	repo := trace.NewFileRepository(dir)
	rec := trace.New(
		trace.WithRepository(repo),
		trace.WithMetadata(trace.TraceMetadata{
			Model:    "test-model",
			Strategy: "simple",
			Labels:   map[string]string{"env": "test"},
		}),
	)

	ctx := context.Background()
	agentCtx := rec.StartAgentExecute(ctx)
	rec.EndAgentExecute(agentCtx, nil)

	err := rec.Finish(ctx)
	gt.NoError(t, err)

	// Verify trace was saved
	tr := rec.Trace()
	gt.Value(t, tr).NotNil()
	gt.Equal(t, tr.Metadata.Model, "test-model")
	gt.Equal(t, tr.Metadata.Strategy, "simple")
}

func TestRecorderFinishWithoutRepo(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	rec.EndAgentExecute(agentCtx, nil)

	// Finish without repo should not error
	err := rec.Finish(ctx)
	gt.NoError(t, err)
}

func TestRecorderNoOpWhenNilContext(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	// These should all be no-ops since there's no current span
	llmCtx := rec.StartLLMCall(ctx)
	gt.Equal(t, llmCtx, ctx)

	toolCtx := rec.StartToolExec(ctx, "search", nil)
	gt.Equal(t, toolCtx, ctx)

	subCtx := rec.StartSubAgent(ctx, "child")
	gt.Equal(t, subCtx, ctx)

	// End methods should not panic with nil spans
	rec.EndLLMCall(ctx, nil, nil)
	rec.EndToolExec(ctx, nil, nil)
	rec.EndSubAgent(ctx, nil)
	rec.AddEvent(ctx, "test", nil)
}

func TestRecorderConcurrentAccess(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()
	agentCtx := rec.StartAgentExecute(ctx)

	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			llmCtx := rec.StartLLMCall(agentCtx)
			rec.EndLLMCall(llmCtx, &trace.LLMCallData{InputTokens: 1}, nil)
		}()
	}
	wg.Wait()

	rootSpan := rec.Trace().RootSpan
	gt.Equal(t, len(rootSpan.Children), 10)
}

func TestRecorderWithCustomTraceID(t *testing.T) {
	customID := "my-request-12345"
	rec := trace.New(trace.WithTraceID(customID))
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	rec.EndAgentExecute(agentCtx, nil)

	tr := rec.Trace()
	gt.Value(t, tr).NotNil()
	gt.Equal(t, tr.TraceID, customID)
}

func TestRecorderWithEmptyTraceID(t *testing.T) {
	rec := trace.New(trace.WithTraceID(""))
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	rec.EndAgentExecute(agentCtx, nil)

	tr := rec.Trace()
	gt.Value(t, tr).NotNil()
	// Empty string should fall back to auto-generated UUID
	gt.Value(t, tr.TraceID).NotEqual("")
}

func TestCaptureStackTrace(t *testing.T) {
	frames := trace.CaptureStackTrace(0)
	gt.A(t, frames).Longer(0)

	// The first frame should be this test function in this file
	gt.S(t, frames[0].Function).Contains("TestCaptureStackTrace")
	gt.S(t, frames[0].File).Contains("trace/recorder_test.go")
	gt.N(t, frames[0].Line).Greater(0)
}

func TestRecorderStackTraceDisabledByDefault(t *testing.T) {
	rec := trace.New()
	ctx := context.Background()

	agentCtx := rec.StartAgentExecute(ctx)
	llmCtx := rec.StartLLMCall(agentCtx)
	rec.EndLLMCall(llmCtx, &trace.LLMCallData{InputTokens: 10}, nil)

	toolCtx := rec.StartToolExec(agentCtx, "search", nil)
	rec.EndToolExec(toolCtx, nil, nil)

	rec.AddEvent(agentCtx, "test_event", nil)

	subCtx := rec.StartSubAgent(agentCtx, "child")
	rec.EndSubAgent(subCtx, nil)

	rec.EndAgentExecute(agentCtx, nil)

	rootSpan := rec.Trace().RootSpan
	gt.Value(t, rootSpan.StackTrace).Nil()
	for _, child := range rootSpan.Children {
		gt.Value(t, child.StackTrace).Nil()
	}
}

func TestRecorderStackTraceEnabled(t *testing.T) {
	rec := trace.New(trace.WithStackTrace())
	ctx := context.Background()

	// Each call is on a distinct line so we can verify line numbers
	agentCtx := rec.StartAgentExecute(ctx)       // agent_execute
	llmCtx := rec.StartLLMCall(agentCtx)          // llm_call
	rec.EndLLMCall(llmCtx, &trace.LLMCallData{InputTokens: 10}, nil)
	toolCtx := rec.StartToolExec(agentCtx, "search", map[string]any{"q": "test"}) // tool_exec
	rec.EndToolExec(toolCtx, nil, nil)
	rec.AddEvent(agentCtx, "test_event", nil) // event
	subCtx := rec.StartSubAgent(agentCtx, "child") // sub_agent
	rec.EndSubAgent(subCtx, nil)
	rec.EndAgentExecute(agentCtx, nil)

	rootSpan := rec.Trace().RootSpan

	// Helper to verify that a span's stack trace points to this test function and file
	verifyStackTrace := func(t *testing.T, span *trace.Span) {
		t.Helper()
		gt.A(t, span.StackTrace).Longer(0)

		topFrame := span.StackTrace[0]
		// The top frame must be this test function
		gt.S(t, topFrame.Function).Contains("TestRecorderStackTraceEnabled")
		// The top frame must point to recorder_test.go
		gt.S(t, topFrame.File).Contains("trace/recorder_test.go")
		// Line number must be positive
		gt.N(t, topFrame.Line).Greater(0)
	}

	t.Run("agent_execute span", func(t *testing.T) {
		verifyStackTrace(t, rootSpan)
	})

	// Children: llm_call, tool_exec, event, sub_agent (in order)
	gt.A(t, rootSpan.Children).Length(4)

	t.Run("llm_call span", func(t *testing.T) {
		span := rootSpan.Children[0]
		gt.V(t, span.Kind).Equal(trace.SpanKindLLMCall)
		verifyStackTrace(t, span)
	})

	t.Run("tool_exec span", func(t *testing.T) {
		span := rootSpan.Children[1]
		gt.V(t, span.Kind).Equal(trace.SpanKindToolExec)
		verifyStackTrace(t, span)
	})

	t.Run("event span", func(t *testing.T) {
		span := rootSpan.Children[2]
		gt.V(t, span.Kind).Equal(trace.SpanKindEvent)
		verifyStackTrace(t, span)
	})

	t.Run("sub_agent span", func(t *testing.T) {
		span := rootSpan.Children[3]
		gt.V(t, span.Kind).Equal(trace.SpanKindSubAgent)
		verifyStackTrace(t, span)
	})

	t.Run("each span has a different line number", func(t *testing.T) {
		// All spans should point to different lines since each Start* call is on a different line
		lines := map[int]trace.SpanKind{}
		allSpans := append([]*trace.Span{rootSpan}, rootSpan.Children...)
		for _, span := range allSpans {
			topLine := span.StackTrace[0].Line
			_, duplicate := lines[topLine]
			gt.B(t, duplicate).False()
			lines[topLine] = span.Kind
		}
	})
}
