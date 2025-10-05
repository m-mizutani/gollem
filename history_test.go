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
					{FunctionResponse: &genai.FunctionResponse{Name: "", Response: map[string]any{"content": `{"result":8}`}}},
					{FunctionResponse: &genai.FunctionResponse{Name: "", Response: map[string]any{"content": `{"result":20}`}}},
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
					{FunctionResponse: &genai.FunctionResponse{Name: "", Response: map[string]any{"content": `{"time":"14:30:00","timezone":"UTC"}`}}},
				},
			},
			{Role: "model", Parts: []*genai.Part{{Text: "It's currently 14:30 UTC."}}},
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
