package gollem_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
	openaiSDK "github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

func TestOpenAIToClaudeConversion(t *testing.T) {
	type testCase struct {
		name             string
		messages         []openaiSDK.ChatCompletionMessage
		expectedMessages []anthropic.MessageParam
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// OpenAI → History
			historyFromOpenAI, err := openai.NewHistory(tc.messages)
			gt.NoError(t, err)

			// History → Claude
			claudeMsgs, err := claude.ToMessages(historyFromOpenAI)
			gt.NoError(t, err)

			// Verify Claude messages
			gt.Equal(t, tc.expectedMessages, claudeMsgs)
		}
	}

	t.Run("text messages with all fields", runTest(testCase{
		name: "text messages with all fields",
		messages: []openaiSDK.ChatCompletionMessage{
			{Role: "user", Content: "Hello", Name: "user1"},
			{Role: "assistant", Content: "Hi there!", Name: "assistant1"},
		},
		expectedMessages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi there!")),
		},
	}))

	t.Run("tool calls with all fields", runTest(testCase{
		name: "tool calls with all fields",
		messages: []openaiSDK.ChatCompletionMessage{
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
		},
		expectedMessages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather in Tokyo and London?")),
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock("call_123", map[string]interface{}{
					"location": "Tokyo",
					"unit":     "celsius",
				}, "get_weather"),
				anthropic.NewToolUseBlock("call_456", map[string]interface{}{
					"location": "London",
					"unit":     "celsius",
				}, "get_weather"),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("call_123", `{"condition":"sunny","humidity":60,"temperature":25}`, false),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("call_456", `{"condition":"rainy","humidity":80,"temperature":15}`, false),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("Tokyo is sunny at 25°C. London is rainy at 15°C.")),
		},
	}))

	t.Run("multi-content with images", runTest(testCase{
		name: "multi-content with images",
		messages: []openaiSDK.ChatCompletionMessage{
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
		},
		expectedMessages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("What's in this image?"),
				anthropic.NewImageBlockBase64("image/png", "iVBORw0KGgo="),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("I see a cat in the image.")),
		},
	}))

	t.Run("system message", runTest(testCase{
		name: "system message",
		messages: []openaiSDK.ChatCompletionMessage{
			{Role: "system", Content: "You are a helpful assistant."},
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi! How can I help you?"},
		},
		expectedMessages: []anthropic.MessageParam{
			// Claude merges system message into first user message with "\n\n" separator
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("You are a helpful assistant.\n\n"),
				anthropic.NewTextBlock("Hello"),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi! How can I help you?")),
		},
	}))

	t.Run("function calls", runTest(testCase{
		name: "function calls",
		messages: []openaiSDK.ChatCompletionMessage{
			{Role: "user", Content: "What's 2+2?"},
			{
				Role: "assistant",
				FunctionCall: &openaiSDK.FunctionCall{
					Name:      "calculate",
					Arguments: `{"expression":"2+2"}`,
				},
			},
			{
				Role:    "function",
				Name:    "calculate",
				Content: `{"result":4}`,
			},
			{Role: "assistant", Content: "The answer is 4."},
		},
		expectedMessages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What's 2+2?")),
			// Claude converts function call to tool use
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock("call_calculate_0", map[string]interface{}{"expression": "2+2"}, "calculate"),
			),
			// Claude converts function response to tool result
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("call_calculate_0", `{"result":4}`, false),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("The answer is 4.")),
		},
	}))
}

func TestClaudeToGeminiConversion(t *testing.T) {
	type testCase struct {
		name             string
		messages         []anthropic.MessageParam
		expectedMessages []*genai.Content
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Claude → History
			historyFromClaude, err := claude.NewHistory(tc.messages)
			gt.NoError(t, err)

			// History → Gemini
			geminiContents, err := gemini.ToContents(historyFromClaude)
			gt.NoError(t, err)

			// Verify Gemini contents
			gt.Equal(t, tc.expectedMessages, geminiContents)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, how are you?")),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("I'm doing well, thank you!")),
		},
		expectedMessages: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Hello, how are you?"}}},
			{Role: "model", Parts: []*genai.Part{{Text: "I'm doing well, thank you!"}}},
		},
	}))

	t.Run("tool use with multiple calls", runTest(testCase{
		name: "tool use with multiple calls",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Calculate 5+3 and 10*2")),
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock("toolu_123", map[string]interface{}{"expression": "5+3"}, "calculate"),
				anthropic.NewToolUseBlock("toolu_456", map[string]interface{}{"expression": "10*2"}, "calculate"),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("toolu_123", `{"result":8}`, false),
				anthropic.NewToolResultBlock("toolu_456", `{"result":20}`, false),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("5+3 equals 8, and 10*2 equals 20.")),
		},
		expectedMessages: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Calculate 5+3 and 10*2"}}},
			{
				Role: "model",
				Parts: []*genai.Part{
					{FunctionCall: &genai.FunctionCall{Name: "calculate", Args: map[string]any{"expression": "5+3"}}},
					{FunctionCall: &genai.FunctionCall{Name: "calculate", Args: map[string]any{"expression": "10*2"}}},
				},
			},
			{
				Role: "user",
				Parts: []*genai.Part{
					// Claude now parses JSON, so result is properly structured
					{FunctionResponse: &genai.FunctionResponse{Name: "", Response: map[string]any{"result": float64(8)}}},
					{FunctionResponse: &genai.FunctionResponse{Name: "", Response: map[string]any{"result": float64(20)}}},
				},
			},
			{Role: "model", Parts: []*genai.Part{{Text: "5+3 equals 8, and 10*2 equals 20."}}},
		},
	}))

	t.Run("mixed content blocks", runTest(testCase{
		name: "mixed content blocks",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Tell me a joke and check the time")),
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("Here's a joke: Why did the chicken cross the road?"),
				anthropic.NewToolUseBlock("toolu_789", map[string]interface{}{}, "get_current_time"),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("toolu_789", `{"time":"14:30:00","timezone":"UTC"}`, false),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("It's currently 14:30 UTC.")),
		},
		expectedMessages: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Tell me a joke and check the time"}}},
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Here's a joke: Why did the chicken cross the road?"},
					{FunctionCall: &genai.FunctionCall{Name: "get_current_time", Args: map[string]any{}}},
				},
			},
			{
				Role: "user",
				Parts: []*genai.Part{
					// Claude parses JSON response
					{FunctionResponse: &genai.FunctionResponse{Name: "", Response: map[string]any{"time": "14:30:00", "timezone": "UTC"}}},
				},
			},
			{Role: "model", Parts: []*genai.Part{{Text: "It's currently 14:30 UTC."}}},
		},
	}))

	t.Run("image content", runTest(testCase{
		name: "image content",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("Analyze this image"),
				anthropic.NewImageBlockBase64("image/jpeg", "/9j/4AAQSkZJRg=="),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("This appears to be a landscape photo.")),
		},
		expectedMessages: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Analyze this image"},
					{InlineData: &genai.Blob{MIMEType: "image/jpeg", Data: []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46}}},
				},
			},
			{Role: "model", Parts: []*genai.Part{{Text: "This appears to be a landscape photo."}}},
		},
	}))

	t.Run("error tool result", runTest(testCase{
		name: "error tool result",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Get the weather")),
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock("toolu_error", map[string]interface{}{"location": "InvalidCity"}, "get_weather"),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("toolu_error", `{"error":"City not found"}`, true),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("I couldn't find that city.")),
		},
		expectedMessages: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Get the weather"}}},
			{
				Role: "model",
				Parts: []*genai.Part{
					{FunctionCall: &genai.FunctionCall{Name: "get_weather", Args: map[string]any{"location": "InvalidCity"}}},
				},
			},
			{
				Role: "user",
				Parts: []*genai.Part{
					// Error responses are also parsed as JSON
					{FunctionResponse: &genai.FunctionResponse{Name: "", Response: map[string]any{"error": "City not found"}}},
				},
			},
			{Role: "model", Parts: []*genai.Part{{Text: "I couldn't find that city."}}},
		},
	}))
}

func TestGeminiToOpenAIConversion(t *testing.T) {
	type testCase struct {
		name             string
		contents         []*genai.Content
		expectedMessages []openaiSDK.ChatCompletionMessage
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Gemini → History
			historyFromGemini, err := gemini.NewHistory(tc.contents)
			gt.NoError(t, err)

			// History → OpenAI
			openaiMsgs, err := openai.ToMessages(historyFromGemini)
			gt.NoError(t, err)

			// Verify OpenAI messages
			gt.Equal(t, tc.expectedMessages, openaiMsgs)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Hello from Gemini"}}},
			{Role: "model", Parts: []*genai.Part{{Text: "Hello! How can I assist you?"}}},
		},
		expectedMessages: []openaiSDK.ChatCompletionMessage{
			{Role: "user", Content: "Hello from Gemini"},
			{Role: "assistant", Content: "Hello! How can I assist you?"},
		},
	}))

	t.Run("function calls with complex args", runTest(testCase{
		name: "function calls with complex args",
		contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Search for Python tutorials"}}},
			{
				Role: "model",
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{
						Name: "search",
						Args: map[string]any{
							"query":  "Python tutorials",
							"limit":  float64(10),
							"filter": map[string]any{"language": "en", "level": "beginner"},
						},
					},
				}},
			},
			{
				Role: "user",
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						Name: "search",
						Response: map[string]any{
							"results": []any{
								map[string]any{"title": "Python Basics", "url": "https://example.com/1"},
								map[string]any{"title": "Learn Python", "url": "https://example.com/2"},
							},
							"total": float64(2),
						},
					},
				}},
			},
			{Role: "model", Parts: []*genai.Part{{Text: "I found 2 Python tutorials for beginners."}}},
		},
		expectedMessages: []openaiSDK.ChatCompletionMessage{
			{Role: "user", Content: "Search for Python tutorials"},
			{
				Role: "assistant",
				ToolCalls: []openaiSDK.ToolCall{{
					ID:   "call_search_0",
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
				ToolCallID: "call_search_0",
				Name:       "search",
			},
			{Role: "assistant", Content: "I found 2 Python tutorials for beginners."},
		},
	}))

	t.Run("multiple parts in single message", runTest(testCase{
		name: "multiple parts in single message",
		contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "First part"},
					{Text: "Second part"},
				},
			},
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Response part 1"},
					{Text: "Response part 2"},
				},
			},
		},
		expectedMessages: []openaiSDK.ChatCompletionMessage{
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
		},
	}))
}

// Round-trip tests: A → B → A' should preserve A = A'
// Note: Some fields may be lost during conversion due to provider limitations:
// - OpenAI tool message Name field (Claude doesn't preserve it)
// - Tool call IDs through Gemini (Gemini regenerates IDs)
func TestOpenAIRoundTrip(t *testing.T) {
	type testCase struct {
		name     string
		messages []openaiSDK.ChatCompletionMessage
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// OpenAI → History → Claude → History → OpenAI
			historyFromOpenAI, err := openai.NewHistory(tc.messages)
			gt.NoError(t, err)

			claudeMsgs, err := claude.ToMessages(historyFromOpenAI)
			gt.NoError(t, err)

			historyFromClaude, err := claude.NewHistory(claudeMsgs)
			gt.NoError(t, err)

			restoredOpenAI, err := openai.ToMessages(historyFromClaude)
			gt.NoError(t, err)

			// A = A'
			gt.Equal(t, tc.messages, restoredOpenAI)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		messages: []openaiSDK.ChatCompletionMessage{
			{Role: "user", Content: "Hello"},
			{Role: "assistant", Content: "Hi there!"},
		},
	}))

	t.Run("tool calls", runTest(testCase{
		name: "tool calls",
		messages: []openaiSDK.ChatCompletionMessage{
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
				// Note: Name field will be lost (Claude doesn't preserve it)
			},
			{Role: "assistant", Content: "It's 25°C in Tokyo."},
		},
	}))
}

func TestClaudeRoundTrip(t *testing.T) {
	type testCase struct {
		name     string
		messages []anthropic.MessageParam
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Claude → History → Gemini → History → Claude
			historyFromClaude, err := claude.NewHistory(tc.messages)
			gt.NoError(t, err)

			geminiContents, err := gemini.ToContents(historyFromClaude)
			gt.NoError(t, err)

			historyFromGemini, err := gemini.NewHistory(geminiContents)
			gt.NoError(t, err)

			restoredClaude, err := claude.ToMessages(historyFromGemini)
			gt.NoError(t, err)

			// A = A'
			gt.Equal(t, tc.messages, restoredClaude)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi!")),
		},
	}))

	// Note: Tool IDs cannot be perfectly round-tripped because Gemini
	// regenerates tool call IDs. Text-only messages can round-trip perfectly.
}

func TestGeminiRoundTrip(t *testing.T) {
	type testCase struct {
		name     string
		contents []*genai.Content
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Gemini → History → OpenAI → History → Gemini
			historyFromGemini, err := gemini.NewHistory(tc.contents)
			gt.NoError(t, err)

			openaiMsgs, err := openai.ToMessages(historyFromGemini)
			gt.NoError(t, err)

			historyFromOpenAI, err := openai.NewHistory(openaiMsgs)
			gt.NoError(t, err)

			restoredGemini, err := gemini.ToContents(historyFromOpenAI)
			gt.NoError(t, err)

			// A = A'
			gt.Equal(t, tc.contents, restoredGemini)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Hello"}}},
			{Role: "model", Parts: []*genai.Part{{Text: "Hi!"}}},
		},
	}))

	t.Run("function calls", runTest(testCase{
		name: "function calls",
		contents: []*genai.Content{
			{Role: "user", Parts: []*genai.Part{{Text: "Search Python"}}},
			{
				Role: "model",
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{
						Name: "search",
						Args: map[string]any{"query": "Python"},
					},
				}},
			},
			{
				Role: "user",
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						Name:     "search",
						Response: map[string]any{"results": []any{"Python tutorial"}},
					},
				}},
			},
			{Role: "model", Parts: []*genai.Part{{Text: "Found Python tutorial."}}},
		},
	}))
}
