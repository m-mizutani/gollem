package gollem_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
)

func createMockClient(resp any) *mock.LLMClientMock {
	return &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			return &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
					raw, err := json.Marshal(resp)
					if err != nil {
						return nil, err
					}

					return &gollem.Response{
						Texts: []string{string(raw)},
					}, nil
				},
			}, nil
		},
		GenerateEmbeddingFunc: func(ctx context.Context, dimension int, input []string) ([][]float64, error) {
			return nil, nil
		},
	}
}

func TestDefaultFacilitator_Spec(t *testing.T) {
	mockClient := createMockClient(map[string]any{
		"action": "continue",
		"reason": "Need to analyze more data",
	})
	facilitator := gollem.NewDefaultFacilitator(mockClient)

	spec := facilitator.Spec()

	gt.Equal(t, spec.Name, gollem.DefaultFacilitatorName)
	gt.Equal(t, spec.Description, "Call this tool when you have gathered all necessary information, completed all required actions, and already provided the final answer to the user's original request. This signals that your work on the current request is finished.")
	gt.NotNil(t, spec.Parameters["summary"])
	gt.Equal(t, spec.Parameters["summary"].Type, gollem.TypeString)
}

func TestDefaultFacilitator_Run(t *testing.T) {
	mockClient := createMockClient(map[string]any{
		"action": "continue",
		"reason": "Need to analyze more data",
	})
	facilitator := gollem.NewDefaultFacilitator(mockClient)

	// Initially not completed
	gt.False(t, facilitator.IsCompleted())

	args := map[string]any{
		"summary": "Task completed successfully",
	}

	result, err := facilitator.Run(context.Background(), args)
	gt.NoError(t, err)
	gt.Nil(t, result)

	// Should be completed after Run()
	gt.True(t, facilitator.IsCompleted())
}

func TestDefaultFacilitator_Facilitate(t *testing.T) {
	type testCase struct {
		name         string
		mockResponse string
		expected     *gollem.Facilitation
		expectError  bool
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			mockSession := &mock.SessionMock{
				GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
					return &gollem.Response{
						Texts: []string{tc.mockResponse},
					}, nil
				},
			}

			mockClient := &mock.LLMClientMock{
				NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
					return mockSession, nil
				},
				GenerateEmbeddingFunc: func(ctx context.Context, dimension int, input []string) ([][]float64, error) {
					return nil, nil
				},
			}

			facilitator := gollem.NewDefaultFacilitator(mockClient)
			history := &gollem.History{}

			result, err := facilitator.Facilitate(context.Background(), history)

			if tc.expectError {
				gt.Error(t, err)
				return
			}

			gt.NoError(t, err)
			gt.Equal(t, result.Action, tc.expected.Action)
			gt.Equal(t, result.Reason, tc.expected.Reason)
			gt.Equal(t, result.NextStep, tc.expected.NextStep)
			gt.Equal(t, result.Completion, tc.expected.Completion)
		}
	}

	t.Run("valid continue response", runTest(testCase{
		name: "valid continue response",
		mockResponse: `{
			"action": "continue",
			"reason": "Need to analyze more data",
			"next_step": "Process remaining files"
		}`,
		expected: &gollem.Facilitation{
			Action:   gollem.ActionContinue,
			Reason:   "Need to analyze more data",
			NextStep: "Process remaining files",
		},
		expectError: false,
	}))

	t.Run("valid complete response", runTest(testCase{
		name: "valid complete response",
		mockResponse: `{
			"action": "complete",
			"reason": "Analysis finished successfully",
			"completion": "Found 5 security issues in the codebase"
		}`,
		expected: &gollem.Facilitation{
			Action:     gollem.ActionComplete,
			Reason:     "Analysis finished successfully",
			Completion: "Found 5 security issues in the codebase",
		},
		expectError: false,
	}))

	t.Run("invalid JSON", runTest(testCase{
		name:         "invalid JSON",
		mockResponse: `{"action": "continue", "reason":}`,
		expected:     nil,
		expectError:  true,
	}))

	// Add validation-specific test cases
	t.Run("continue without next_step", runTest(testCase{
		name: "continue without next_step",
		mockResponse: `{
			"action": "continue",
			"reason": "Need to analyze more data"
		}`,
		expected:    nil,
		expectError: true,
	}))

	t.Run("complete without completion", runTest(testCase{
		name: "complete without completion",
		mockResponse: `{
			"action": "complete",
			"reason": "Analysis finished successfully"
		}`,
		expected:    nil,
		expectError: true,
	}))

	t.Run("invalid action", runTest(testCase{
		name: "invalid action",
		mockResponse: `{
			"action": "invalid",
			"reason": "Some reason"
		}`,
		expected:    nil,
		expectError: true,
	}))

	t.Run("empty action", runTest(testCase{
		name: "empty action",
		mockResponse: `{
			"action": "",
			"reason": "Some reason"
		}`,
		expected:    nil,
		expectError: true,
	}))

	t.Run("continue with empty next_step string", runTest(testCase{
		name: "continue with empty next_step string",
		mockResponse: `{
			"action": "continue",
			"reason": "Need to analyze more data",
			"next_step": ""
		}`,
		expected:    nil,
		expectError: true,
	}))

	t.Run("complete with empty completion string", runTest(testCase{
		name: "complete with empty completion string",
		mockResponse: `{
			"action": "complete",
			"reason": "Analysis finished successfully",
			"completion": ""
		}`,
		expected:    nil,
		expectError: true,
	}))

	t.Run("continue with whitespace-only next_step", runTest(testCase{
		name: "continue with whitespace-only next_step",
		mockResponse: `{
			"action": "continue",
			"reason": "Need to analyze more data",
			"next_step": "   "
		}`,
		expected: &gollem.Facilitation{
			Action:   gollem.ActionContinue,
			Reason:   "Need to analyze more data",
			NextStep: "   ",
		},
		expectError: false, // Current implementation only checks for empty string, not whitespace
	}))
}

func TestDefaultFacilitator_IsCompleted(t *testing.T) {
	mockClient := createMockClient(map[string]any{
		"action": "continue",
		"reason": "Need to analyze more data",
	})
	facilitator := gollem.NewDefaultFacilitator(mockClient)

	// Initially should not be completed
	gt.False(t, facilitator.IsCompleted())

	// After calling Run, should be completed
	_, err := facilitator.Run(context.Background(), map[string]any{"summary": "test"})
	gt.NoError(t, err)
	gt.True(t, facilitator.IsCompleted())
}

func TestFacilitation_JSONSerialization(t *testing.T) {
	resp := gollem.Facilitation{
		Action:     gollem.ActionComplete,
		Reason:     "Analysis completed",
		Completion: "Found 3 issues",
	}

	data, err := json.Marshal(resp)
	gt.NoError(t, err)

	var unmarshaled gollem.Facilitation
	err = json.Unmarshal(data, &unmarshaled)
	gt.NoError(t, err)

	gt.Equal(t, unmarshaled.Action, resp.Action)
	gt.Equal(t, unmarshaled.Reason, resp.Reason)
	gt.Equal(t, unmarshaled.Completion, resp.Completion)
}

func TestActionType_String(t *testing.T) {
	gt.Equal(t, string(gollem.ActionContinue), "continue")
	gt.Equal(t, string(gollem.ActionComplete), "complete")
}

func TestDefaultProceedPrompt(t *testing.T) {
	// Test that the prompt is not empty and contains expected JSON structure guidance
	gt.NotEqual(t, gollem.DefaultProceedPrompt, "")
	gt.True(t, strings.Contains(gollem.DefaultProceedPrompt, "JSON format"))
	gt.True(t, strings.Contains(gollem.DefaultProceedPrompt, "continue"))
	gt.True(t, strings.Contains(gollem.DefaultProceedPrompt, "complete"))
}

func TestFacilitation_Validate(t *testing.T) {
	type testCase struct {
		name         string
		facilitation gollem.Facilitation
		expectError  bool
		errorMsg     string
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			err := tc.facilitation.Validate()

			if tc.expectError {
				gt.Error(t, err)
				if tc.errorMsg != "" {
					gt.True(t, strings.Contains(err.Error(), tc.errorMsg))
				}
			} else {
				gt.NoError(t, err)
			}
		}
	}

	t.Run("valid continue", runTest(testCase{
		name: "valid continue",
		facilitation: gollem.Facilitation{
			Action:   gollem.ActionContinue,
			Reason:   "Need to process more data",
			NextStep: "Analyze remaining files",
		},
		expectError: false,
	}))

	t.Run("valid complete", runTest(testCase{
		name: "valid complete",
		facilitation: gollem.Facilitation{
			Action:     gollem.ActionComplete,
			Reason:     "Analysis finished",
			Completion: "Found 3 security issues",
		},
		expectError: false,
	}))

	t.Run("continue without next_step", runTest(testCase{
		name: "continue without next_step",
		facilitation: gollem.Facilitation{
			Action: gollem.ActionContinue,
			Reason: "Need to process more data",
			// NextStep is empty
		},
		expectError: true,
		errorMsg:    "next_step is required when action is continue",
	}))

	t.Run("complete without completion", runTest(testCase{
		name: "complete without completion",
		facilitation: gollem.Facilitation{
			Action: gollem.ActionComplete,
			Reason: "Analysis finished",
			// Completion is empty
		},
		expectError: true,
		errorMsg:    "completion is required when action is complete",
	}))

	t.Run("invalid action", runTest(testCase{
		name: "invalid action",
		facilitation: gollem.Facilitation{
			Action: "invalid",
			Reason: "Some reason",
		},
		expectError: true,
		errorMsg:    "invalid action",
	}))

	t.Run("empty action", runTest(testCase{
		name: "empty action",
		facilitation: gollem.Facilitation{
			Action: "",
			Reason: "Some reason",
		},
		expectError: true,
		errorMsg:    "invalid action",
	}))

	t.Run("continue with empty next_step string", runTest(testCase{
		name: "continue with empty next_step string",
		facilitation: gollem.Facilitation{
			Action:   gollem.ActionContinue,
			Reason:   "Need to process more data",
			NextStep: "",
		},
		expectError: true,
		errorMsg:    "next_step is required when action is continue",
	}))

	t.Run("complete with empty completion string", runTest(testCase{
		name: "complete with empty completion string",
		facilitation: gollem.Facilitation{
			Action:     gollem.ActionComplete,
			Reason:     "Analysis finished",
			Completion: "",
		},
		expectError: true,
		errorMsg:    "completion is required when action is complete",
	}))

	t.Run("continue with whitespace-only next_step", runTest(testCase{
		name: "continue with whitespace-only next_step",
		facilitation: gollem.Facilitation{
			Action:   gollem.ActionContinue,
			Reason:   "Need to process more data",
			NextStep: "   ",
		},
		expectError: false, // Current implementation only checks for empty string, not whitespace
	}))
}
