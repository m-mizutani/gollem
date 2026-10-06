package trace_test

import (
	"encoding/json"
	"testing"

	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/gt"
)

func TestLLMResponseFinishReasonJSON(t *testing.T) {
	type testCase struct {
		resp     trace.LLMResponse
		expected string
	}
	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			raw, err := json.Marshal(tc.resp)
			gt.NoError(t, err).Required()
			gt.Equal(t, tc.expected, string(raw))
		}
	}

	t.Run("empty reason is omitted", runTest(testCase{
		resp:     trace.LLMResponse{Texts: []string{"ok"}},
		expected: `{"texts":["ok"]}`,
	}))
	t.Run("reason is written without texts", runTest(testCase{
		resp:     trace.LLMResponse{FinishReason: "refusal"},
		expected: `{"finish_reason":"refusal"}`,
	}))
}

func TestLLMResponseRefusalJSON(t *testing.T) {
	type testCase struct {
		resp     trace.LLMResponse
		expected string
	}
	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			raw, err := json.Marshal(tc.resp)
			gt.NoError(t, err).Required()
			gt.Equal(t, tc.expected, string(raw))
		}
	}

	t.Run("nil refusal is omitted", runTest(testCase{
		resp:     trace.LLMResponse{FinishReason: "end_turn"},
		expected: `{"finish_reason":"end_turn"}`,
	}))
	t.Run("refusal is written with every value", runTest(testCase{
		resp: trace.LLMResponse{
			FinishReason: "refusal",
			Refusal: &trace.Refusal{
				Reason:      "refusal",
				Categories:  []string{"cyber"},
				Explanation: "declined",
			},
		},
		expected: `{"finish_reason":"refusal","refusal":{"reason":"refusal","categories":["cyber"],"explanation":"declined"}}`,
	}))
	t.Run("empty values of a refusal are omitted", runTest(testCase{
		resp: trace.LLMResponse{
			FinishReason: "content_filter",
			Refusal:      &trace.Refusal{Reason: "content_filter"},
		},
		expected: `{"finish_reason":"content_filter","refusal":{"reason":"content_filter"}}`,
	}))
}
