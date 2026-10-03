package openai_test

import (
	"encoding/json"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/internal/historytest"
	"github.com/gollem-dev/gollem/llm/openai"
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

	t.Run("PDF data URL", runTest(testCase{
		name: "PDF data URL",
		messages: []openaiSDK.ChatCompletionMessage{
			{
				Role: "user",
				MultiContent: []openaiSDK.ChatMessagePart{
					{Type: "text", Text: "Analyze this PDF"},
					{
						Type: "image_url",
						ImageURL: &openaiSDK.ChatMessageImageURL{
							URL: "data:application/pdf;base64,JVBER" + "i0xLjQgdGVzdA==",
						},
					},
				},
			},
			{
				Role:    "assistant",
				Content: "This PDF contains test data.",
			},
		},
	}))

	t.Run("reasoning content", func(t *testing.T) {
		// Test reasoning content conversion (OpenAI → gollem)
		history, err := openai.NewHistory([]openaiSDK.ChatCompletionMessage{
			{
				Role:    "user",
				Content: "Help me solve this problem",
			},
			{
				Role:             "assistant",
				ReasoningContent: "Let me think through this step by step...",
				Content:          "Here's the solution",
			},
		})
		gt.NoError(t, err)

		// Find assistant message with reasoning content
		var assistantMsg *gollem.Message
		for i := range history.Messages {
			if history.Messages[i].Role == gollem.RoleAssistant {
				assistantMsg = &history.Messages[i]
				break
			}
		}

		gt.NotNil(t, assistantMsg)
		gt.Equal(t, 2, len(assistantMsg.Contents))

		// First content should be reasoning
		reasoningContent := assistantMsg.Contents[0]
		gt.Equal(t, gollem.MessageContentTypeThinking, reasoningContent.Type)

		thinking, err := reasoningContent.GetThinkingContent()
		gt.NoError(t, err)
		gt.Equal(t, "Let me think through this step by step...", thinking.Text)

		// Second content should be text
		textContent := assistantMsg.Contents[1]
		gt.Equal(t, gollem.MessageContentTypeText, textContent.Type)

		text, err := textContent.GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, "Here's the solution", text.Text)

		// Test round-trip conversion (gollem → OpenAI)
		restored, err := openai.ToMessages(history)
		gt.NoError(t, err)

		gt.Equal(t, 2, len(restored))
		gt.Equal(t, "Let me think through this step by step...", restored[1].ReasoningContent)
		gt.Equal(t, "Here's the solution", restored[1].Content)
	})

	// Legacy function calls are converted to tool calls internally,
	// so round-trip conversion will not preserve the original function format.
	// This is expected behavior in v3.
}

// OpenAI carries tool arguments as a JSON string, so this pins the exact bytes the request
// contains: an integer wider than float64 must not come back rounded.
func TestOpenAIHistoryPreservesWideIntegers(t *testing.T) {
	const wide = "9007199254740993"

	messages := []openaiSDK.ChatCompletionMessage{
		{
			Role: "assistant",
			ToolCalls: []openaiSDK.ToolCall{{
				ID:       "call_1",
				Type:     "function",
				Function: openaiSDK.FunctionCall{Name: "lookup", Arguments: `{"id":` + wide + `}`},
			}},
		},
		{Role: "tool", ToolCallID: "call_1", Name: "lookup", Content: `{"account":` + wide + `}`},
	}

	history, err := openai.NewHistory(messages)
	gt.NoError(t, err)

	restored, err := openai.ToMessages(history)
	gt.NoError(t, err)

	gt.Equal(t, `{"id":`+wide+`}`, restored[0].ToolCalls[0].Function.Arguments)
	gt.Equal(t, `{"account":`+wide+`}`, restored[1].Content)
}

// A tool result that is a JSON object followed by prose is not a JSON object. It has to
// fall back to being carried as raw text, not be truncated to the leading object.
func TestOpenAIToolContentWithTrailingTextIsKeptWhole(t *testing.T) {
	content := `{"ok":true} and a note about the result`

	history, err := openai.NewHistory([]openaiSDK.ChatCompletionMessage{
		{Role: "tool", ToolCallID: "call_1", Name: "check", Content: content},
	})
	gt.NoError(t, err)

	resp, err := history.Messages[0].Contents[0].GetToolResponseContent()
	gt.NoError(t, err)
	gt.Equal(t, content, gt.Cast[string](t, resp.Response["content"]))
}

// Cross-provider conversion tests. Each one is split at the gollem.History shared
// through internal/historytest; the test with the same name in the other provider
// package converts that History. See the historytest package documentation.

// OpenAI → History. llm/claude converts the History to Claude messages.
func TestOpenAIToClaudeConversion(t *testing.T) {
	runTest := func(messages []openaiSDK.ChatCompletionMessage) func(t *testing.T) {
		return func(t *testing.T) {
			history, err := openai.NewHistory(messages)
			gt.NoError(t, err).Required()
			historytest.Equal(t, "openai_to_claude", "openai", history)
		}
	}

	t.Run("text messages with all fields", runTest([]openaiSDK.ChatCompletionMessage{
		{Role: "user", Content: "Hello", Name: "user1"},
		{Role: "assistant", Content: "Hi there!", Name: "assistant1"},
	}))

	t.Run("tool calls with all fields", runTest([]openaiSDK.ChatCompletionMessage{
		{Role: "user", Content: "What's the weather in Tokyo and London?"},
		{
			Role: "assistant",
			ToolCalls: []openaiSDK.ToolCall{
				{
					ID:   "call_123",
					Type: "function",
					Function: openaiSDK.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"Tokyo","unit":"celsius"}`,
					},
				},
				{
					ID:   "call_456",
					Type: "function",
					Function: openaiSDK.FunctionCall{
						Name:      "get_weather",
						Arguments: `{"location":"London","unit":"celsius"}`,
					},
				},
			},
		},
		{
			Role:       "tool",
			Content:    `{"temperature":25,"condition":"sunny","humidity":60}`,
			ToolCallID: "call_123",
			Name:       "get_weather",
		},
		{
			Role:       "tool",
			Content:    `{"temperature":15,"condition":"rainy","humidity":80}`,
			ToolCallID: "call_456",
			Name:       "get_weather",
		},
		{Role: "assistant", Content: "Tokyo is sunny at 25°C. London is rainy at 15°C."},
	}))

	t.Run("multi-content with images", runTest([]openaiSDK.ChatCompletionMessage{
		{
			Role: "user",
			MultiContent: []openaiSDK.ChatMessagePart{
				{Type: "text", Text: "What's in this image?"},
				{
					Type: "image_url",
					ImageURL: &openaiSDK.ChatMessageImageURL{
						URL:    "data:image/png;base64,iVBORw0KGgo=",
						Detail: "high",
					},
				},
			},
		},
		{Role: "assistant", Content: "I see a cat in the image."},
	}))

	t.Run("system message", runTest([]openaiSDK.ChatCompletionMessage{
		{Role: "system", Content: "You are a helpful assistant."},
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi! How can I help you?"},
	}))

	t.Run("PDF content", runTest([]openaiSDK.ChatCompletionMessage{
		{
			Role: "user",
			MultiContent: []openaiSDK.ChatMessagePart{
				{Type: "text", Text: "Analyze this PDF"},
				{
					Type: "image_url",
					ImageURL: &openaiSDK.ChatMessageImageURL{
						URL: "data:application/pdf;base64,JVBERi0xLjQgdGVzdA==",
					},
				},
			},
		},
		{Role: "assistant", Content: "This PDF contains test data."},
	}))
}

// History → OpenAI. llm/gemini converts Gemini contents to the History.
func TestGeminiToOpenAIConversion(t *testing.T) {
	runTest := func(expected []openaiSDK.ChatCompletionMessage) func(t *testing.T) {
		return func(t *testing.T) {
			messages, err := openai.ToMessages(historytest.Load(t, "gemini_to_openai", "gemini"))
			gt.NoError(t, err)
			gt.Equal(t, expected, messages)
		}
	}

	t.Run("text messages", runTest([]openaiSDK.ChatCompletionMessage{
		{Role: "user", Content: "Hello from Gemini"},
		{Role: "assistant", Content: "Hello! How can I assist you?"},
	}))

	t.Run("function calls with complex args", runTest([]openaiSDK.ChatCompletionMessage{
		{Role: "user", Content: "Search for Python tutorials"},
		{
			Role: "assistant",
			ToolCalls: []openaiSDK.ToolCall{{
				ID:   "gemini-fallback-search-0",
				Type: "function",
				Function: openaiSDK.FunctionCall{
					Name:      "search",
					Arguments: `{"filter":{"language":"en","level":"beginner"},"limit":10,"query":"Python tutorials"}`,
				},
			}},
		},
		{
			Role:       "tool",
			Content:    `{"results":[{"title":"Python Basics","url":"https://example.com/1"},{"title":"Learn Python","url":"https://example.com/2"}],"total":2}`,
			ToolCallID: "gemini-fallback-search-0",
			Name:       "search",
		},
		{Role: "assistant", Content: "I found 2 Python tutorials for beginners."},
	}))

	t.Run("multiple parts in single message", runTest([]openaiSDK.ChatCompletionMessage{
		{
			Role: "user",
			MultiContent: []openaiSDK.ChatMessagePart{
				{Type: "text", Text: "First part"},
				{Type: "text", Text: "Second part"},
			},
		},
		{
			Role: "assistant",
			MultiContent: []openaiSDK.ChatMessagePart{
				{Type: "text", Text: "Response part 1"},
				{Type: "text", Text: "Response part 2"},
			},
		},
	}))

	t.Run("PDF content", runTest([]openaiSDK.ChatCompletionMessage{
		{
			Role: "user",
			MultiContent: []openaiSDK.ChatMessagePart{
				{Type: "text", Text: "Analyze this PDF"},
				{
					Type: "image_url",
					ImageURL: &openaiSDK.ChatMessageImageURL{
						URL: "data:application/pdf;base64,JVBERi0xLjQgdGVzdA==",
					},
				},
			},
		},
		{Role: "assistant", Content: "This PDF contains test data."},
	}))
}

// OpenAI → History → Claude → History → OpenAI must restore the original messages.
// This test covers both ends; llm/claude converts the first History through Claude
// messages into the second.
func TestOpenAIRoundTrip(t *testing.T) {
	runTest := func(messages []openaiSDK.ChatCompletionMessage) func(t *testing.T) {
		return func(t *testing.T) {
			history, err := openai.NewHistory(messages)
			gt.NoError(t, err).Required()
			historytest.Equal(t, "openai_round_trip", "openai", history)

			restored, err := openai.ToMessages(historytest.Load(t, "openai_round_trip", "claude"))
			gt.NoError(t, err)
			gt.Equal(t, messages, restored)
		}
	}

	t.Run("text messages", runTest([]openaiSDK.ChatCompletionMessage{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there!"},
	}))

	t.Run("tool calls", runTest([]openaiSDK.ChatCompletionMessage{
		{Role: "user", Content: "What's the weather?"},
		{
			Role: "assistant",
			ToolCalls: []openaiSDK.ToolCall{{
				ID:   "call_123",
				Type: "function",
				Function: openaiSDK.FunctionCall{
					Name:      "get_weather",
					Arguments: `{"location":"Tokyo"}`,
				},
			}},
		},
		{
			Role:       "tool",
			Content:    `{"temperature":25}`,
			ToolCallID: "call_123",
			// A Claude tool_result carries no tool name, so this used to come back empty.
			// It is now recovered from the tool_use block with the same ID.
			Name: "get_weather",
		},
		{Role: "assistant", Content: "It's 25°C in Tokyo."},
	}))

	t.Run("PDF content", runTest([]openaiSDK.ChatCompletionMessage{
		{
			Role: "user",
			MultiContent: []openaiSDK.ChatMessagePart{
				{Type: "text", Text: "Analyze this PDF"},
				{
					Type: "image_url",
					ImageURL: &openaiSDK.ChatMessageImageURL{
						URL: "data:application/pdf;base64,JVBERi0xLjQgdGVzdA==",
					},
				},
			},
		},
		{Role: "assistant", Content: "This PDF contains test data."},
	}))
}

// The OpenAI leg of Gemini → History → OpenAI → History → Gemini. llm/gemini covers
// both ends and compares the restored contents with the original.
func TestGeminiRoundTrip(t *testing.T) {
	run := func(t *testing.T) {
		messages, err := openai.ToMessages(historytest.Load(t, "gemini_round_trip", "gemini"))
		gt.NoError(t, err).Required()

		history, err := openai.NewHistory(messages)
		gt.NoError(t, err).Required()
		historytest.Equal(t, "gemini_round_trip", "openai", history)
	}

	t.Run("text messages", run)
	t.Run("function calls", run)
	t.Run("PDF content", run)
}
