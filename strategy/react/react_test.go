package react_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gollem/strategy/react"
	"github.com/m-mizutani/gt"
)

// Helper function to check if string contains substring
func containsString(s, substr string) bool {
	return strings.Contains(s, substr)
}

func TestReactStrategy(t *testing.T) {
	ctx := context.Background()

	t.Run("initial iteration adds thought prompt", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{}
		strategy := react.New()
		handler := strategy(mockClient)

		initInput := []gollem.Input{
			gollem.Text("solve this problem"),
		}

		state := &gollem.StrategyState{
			InitInput: initInput,
			Iteration: 0,
		}

		result, _, err := handler(ctx, state)
		gt.NoError(t, err)

		// Should have thought prompt + initial input
		gt.Equal(t, 2, len(result))

		// First element should be the thought prompt
		thoughtText := result[0].String()
		if !containsString(thoughtText, "step-by-step") {
			t.Errorf("Expected thought prompt to contain 'step-by-step', got: %s", thoughtText)
		}

		// Second element should be the original input
		gt.Equal(t, initInput[0].String(), result[1].String())
	})

	t.Run("processes tool results with reflection", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{}
		strategy := react.New()
		handler := strategy(mockClient)

		toolResponse := gollem.FunctionResponse{
			ID:   "call-123",
			Name: "search",
			Data: map[string]any{"result": "found"},
		}

		state := &gollem.StrategyState{
			InitInput: []gollem.Input{gollem.Text("search for info")},
			NextInput: []gollem.Input{toolResponse},
			Iteration: 1,
			LastResponse: &gollem.Response{
				FunctionCalls: []*gollem.FunctionCall{
					{ID: "call-123", Name: "search", Arguments: map[string]any{"query": "test"}},
				},
			},
		}

		result, _, err := handler(ctx, state)
		gt.NoError(t, err)

		// Should have reflection prompt + tool response
		gt.Equal(t, 2, len(result))

		// First element should be reflection prompt
		reflectionText := result[0].String()
		if !containsString(reflectionText, "Based on the previous result") {
			t.Errorf("Expected reflection prompt to contain 'Based on the previous result', got: %s", reflectionText)
		}
		if !containsString(reflectionText, "result from search") {
			t.Errorf("Expected reflection prompt to contain 'result from search', got: %s", reflectionText)
		}

		// Second element should be the tool response
		fr, ok := result[1].(gollem.FunctionResponse)
		gt.True(t, ok)
		gt.Equal(t, toolResponse.ID, fr.ID)
	})

	t.Run("evaluates completion when no pending input", func(t *testing.T) {
		mockSession := &mock.SessionMock{
			GenerateContentFunc: func(ctx context.Context, i ...gollem.Input) (*gollem.Response, error) {
				// Return task complete response
				completionCheck := map[string]any{
					"is_complete": true,
					"reason":      "Task has been completed successfully",
				}
				jsonBytes, _ := json.Marshal(completionCheck)
				return &gollem.Response{
					Texts: []string{string(jsonBytes)},
				}, nil
			},
		}

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, sessionOptions ...gollem.SessionOption) (gollem.Session, error) {
				return mockSession, nil
			},
		}

		strategy := react.New()
		handler := strategy(mockClient)

		state := &gollem.StrategyState{
			InitInput: []gollem.Input{gollem.Text("task")},
			NextInput: []gollem.Input{}, // No pending input
			Iteration: 2,
			LastResponse: &gollem.Response{
				Texts: []string{"Task done"},
			},
		}

		result, resp, err := handler(ctx, state)
		gt.NoError(t, err)

		// Should return nil when task is complete
		gt.V(t, result).Nil()

		// Should return ExecuteResponse with completion message
		gt.NotNil(t, resp)
		gt.Equal(t, "Task completed: Task has been completed successfully", resp.String())

		// Verify session was created with correct options
		gt.Equal(t, 1, len(mockClient.NewSessionCalls()))

		// Verify GenerateContent was called
		gt.Equal(t, 1, len(mockSession.GenerateContentCalls()))
	})

	t.Run("continues when task not complete", func(t *testing.T) {
		mockSession := &mock.SessionMock{
			GenerateContentFunc: func(ctx context.Context, i ...gollem.Input) (*gollem.Response, error) {
				// Return task not complete response
				completionCheck := map[string]any{
					"is_complete": false,
					"reason":      "Need more information",
					"next_action": "Search for additional details",
				}
				jsonBytes, _ := json.Marshal(completionCheck)
				return &gollem.Response{
					Texts: []string{string(jsonBytes)},
				}, nil
			},
		}

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, sessionOptions ...gollem.SessionOption) (gollem.Session, error) {
				return mockSession, nil
			},
		}

		strategy := react.New()
		handler := strategy(mockClient)

		state := &gollem.StrategyState{
			InitInput: []gollem.Input{gollem.Text("task")},
			NextInput: []gollem.Input{},
			Iteration: 2,
			LastResponse: &gollem.Response{
				Texts: []string{"Working on it"},
			},
		}

		result, _, err := handler(ctx, state)
		gt.NoError(t, err)

		// Should return next action
		gt.Equal(t, 1, len(result))
		if !containsString(result[0].String(), "Search for additional details") {
			t.Errorf("Expected result to contain 'Search for additional details', got: %s", result[0].String())
		}
	})

	t.Run("handles error in tool response", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{}
		strategy := react.New()
		handler := strategy(mockClient)

		errorResponse := gollem.FunctionResponse{
			ID:    "call-456",
			Name:  "failing_tool",
			Error: fmt.Errorf("tool execution failed"),
		}

		state := &gollem.StrategyState{
			InitInput: []gollem.Input{gollem.Text("execute task")},
			NextInput: []gollem.Input{errorResponse},
			Iteration: 1,
			LastResponse: &gollem.Response{
				FunctionCalls: []*gollem.FunctionCall{
					{ID: "call-456", Name: "failing_tool", Arguments: map[string]any{}},
				},
			},
		}

		result, _, err := handler(ctx, state)
		gt.NoError(t, err)

		// Should have reflection prompt + error response
		gt.Equal(t, 2, len(result))

		// Reflection should mention error
		reflectionText := result[0].String()
		if !containsString(reflectionText, "error from failing_tool") {
			t.Errorf("Expected reflection prompt to contain 'error from failing_tool', got: %s", reflectionText)
		}
	})

	t.Run("custom prompts are applied", func(t *testing.T) {
		customThought := "Custom thinking prompt"
		customReflection := "Custom reflection: %s"
		customFinish := "Custom completion check"

		mockSession := &mock.SessionMock{
			GenerateContentFunc: func(ctx context.Context, i ...gollem.Input) (*gollem.Response, error) {
				// Verify custom finish prompt is used
				inputStr := i[0].String()
				if !containsString(inputStr, customFinish) {
					t.Errorf("Expected custom finish prompt in input")
				}
				return &gollem.Response{Texts: []string{"COMPLETE"}}, nil
			},
		}

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, sessionOptions ...gollem.SessionOption) (gollem.Session, error) {
				return mockSession, nil
			},
		}

		strategy := react.New(
			react.WithThoughtPrompt(customThought),
			react.WithReflectionPrompt(customReflection),
			react.WithFinishPrompt(customFinish),
		)
		handler := strategy(mockClient)

		// Test custom thought prompt
		state := &gollem.StrategyState{
			InitInput: []gollem.Input{gollem.Text("test")},
			Iteration: 0,
		}

		result, _, err := handler(ctx, state)
		gt.NoError(t, err)
		gt.Equal(t, customThought, result[0].String())

		// Test custom reflection prompt
		state = &gollem.StrategyState{
			InitInput: []gollem.Input{gollem.Text("test")},
			NextInput: []gollem.Input{
				gollem.FunctionResponse{
					ID:   "id",
					Name: "tool",
					Data: map[string]any{},
				},
			},
			Iteration: 1,
		}

		result, _, err = handler(ctx, state)
		gt.NoError(t, err)
		if !containsString(result[0].String(), "Custom reflection:") {
			t.Errorf("Expected result to contain 'Custom reflection:', got: %s", result[0].String())
		}

		// Test custom finish prompt (already tested in mockSession above)
		state = &gollem.StrategyState{
			InitInput:    []gollem.Input{gollem.Text("test")},
			NextInput:    []gollem.Input{},
			Iteration:    2,
			LastResponse: &gollem.Response{Texts: []string{"text"}},
		}

		result, _, err = handler(ctx, state)
		gt.NoError(t, err)
		// Should continue with task when no explicit completion
		gt.NotNil(t, result)
	})

	t.Run("handles multiple tool results", func(t *testing.T) {
		mockClient := &mock.LLMClientMock{}
		strategy := react.New()
		handler := strategy(mockClient)

		toolResponses := []gollem.Input{
			gollem.FunctionResponse{
				ID:   "call-1",
				Name: "tool1",
				Data: map[string]any{"result": "data1"},
			},
			gollem.FunctionResponse{
				ID:   "call-2",
				Name: "tool2",
				Data: map[string]any{"result": "data2"},
			},
		}

		state := &gollem.StrategyState{
			InitInput: []gollem.Input{gollem.Text("multi-tool task")},
			NextInput: toolResponses,
			Iteration: 1,
			LastResponse: &gollem.Response{
				FunctionCalls: []*gollem.FunctionCall{
					{ID: "call-1", Name: "tool1", Arguments: map[string]any{}},
					{ID: "call-2", Name: "tool2", Arguments: map[string]any{}},
				},
			},
		}

		result, _, err := handler(ctx, state)
		gt.NoError(t, err)

		// Should have reflection prompt + both tool responses
		gt.Equal(t, 3, len(result))

		// Reflection should mention multiple tools
		reflectionText := result[0].String()
		if !containsString(reflectionText, "2 tool results") {
			t.Errorf("Expected reflection prompt to contain '2 tool results', got: %s", reflectionText)
		}

		// Both tool responses should be included
		fr1, ok := result[1].(gollem.FunctionResponse)
		gt.True(t, ok)
		gt.Equal(t, "call-1", fr1.ID)

		fr2, ok := result[2].(gollem.FunctionResponse)
		gt.True(t, ok)
		gt.Equal(t, "call-2", fr2.ID)
	})

	t.Run("fallback when JSON parsing fails", func(t *testing.T) {
		mockSession := &mock.SessionMock{
			GenerateContentFunc: func(ctx context.Context, i ...gollem.Input) (*gollem.Response, error) {
				// Return non-JSON response
				return &gollem.Response{
					Texts: []string{"Task is COMPLETE"},
				}, nil
			},
		}

		mockClient := &mock.LLMClientMock{
			NewSessionFunc: func(ctx context.Context, sessionOptions ...gollem.SessionOption) (gollem.Session, error) {
				return mockSession, nil
			},
		}

		strategy := react.New()
		handler := strategy(mockClient)

		state := &gollem.StrategyState{
			InitInput:    []gollem.Input{gollem.Text("task")},
			NextInput:    []gollem.Input{},
			Iteration:    2,
			LastResponse: &gollem.Response{Texts: []string{"text"}},
		}

		result, _, err := handler(ctx, state)
		gt.NoError(t, err)

		// Should continue when JSON parsing fails (no assumptions)
		gt.NotNil(t, result)
	})
}
