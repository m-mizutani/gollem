package openai_test

import (
	"encoding/json"
	"testing"

	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
	openaiSDK "github.com/sashabaranov/go-openai"
)

// normalizeToolMessages normalizes JSON content in tool/function messages
// to handle JSON key reordering during round-trip conversion
func normalizeToolMessages(messages []openaiSDK.ChatCompletionMessage) []openaiSDK.ChatCompletionMessage {
	result := make([]openaiSDK.ChatCompletionMessage, len(messages))
	copy(result, messages)

	for i := range result {
		msg := &result[i]
		if (msg.Role == "tool" || msg.Role == "function") && msg.Content != "" {
			// Try to parse and re-stringify JSON to normalize key order
			var parsed interface{}
			if err := json.Unmarshal([]byte(msg.Content), &parsed); err == nil {
				if normalized, err := json.Marshal(parsed); err == nil {
					msg.Content = string(normalized)
				}
			}
		}
	}

	return result
}

func TestOpenAIMessageRoundTrip(t *testing.T) {
	type testCase struct {
		name     string
		messages []openaiSDK.ChatCompletionMessage
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Convert OpenAI messages to gollem.History
			history, err := openai.NewHistory(tc.messages)
			gt.NoError(t, err)

			// Convert back to OpenAI messages
			restored, err := openai.ToMessages(history)
			gt.NoError(t, err)

			// Normalize JSON content in tool/function messages before comparison
			// (JSON key ordering is not guaranteed to be preserved)
			normalizedOrig := normalizeToolMessages(tc.messages)
			normalizedRest := normalizeToolMessages(restored)

			// Compare normalized messages
			gt.Equal(t, normalizedOrig, normalizedRest)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		messages: []openaiSDK.ChatCompletionMessage{
			{
				Role:    "system",
				Content: "You are a helpful assistant.",
			},
			{
				Role:    "user",
				Content: "Hello",
			},
			{
				Role:    "assistant",
				Content: "Hi, how can I help you?",
			},
		},
	}))

	t.Run("tool calls and responses", runTest(testCase{
		name: "tool calls and responses",
		messages: []openaiSDK.ChatCompletionMessage{
			{
				Role:    "user",
				Content: "What's the weather?",
			},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []openaiSDK.ToolCall{
					{
						ID:   "call_abc123",
						Type: "function",
						Function: openaiSDK.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"location":"Tokyo"}`,
						},
					},
				},
			},
			{
				Role:       "tool",
				Content:    `{"temperature":25,"condition":"sunny"}`,
				ToolCallID: "call_abc123",
			},
			{
				Role:    "assistant",
				Content: "The weather in Tokyo is sunny with a temperature of 25°C.",
			},
		},
	}))

	// Legacy function calls are converted to tool calls internally,
	// so round-trip conversion will not preserve the original function format.
	// This is expected behavior in v3.
}
