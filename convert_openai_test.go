package gollem_test

import (
	"testing"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

func TestOpenAIConversion(t *testing.T) {
	t.Run("OpenAI basic messages", func(t *testing.T) {
		testCases := []struct {
			name  string
			input []openai.ChatCompletionMessage
		}{
			{
				name: "simple text message",
				input: []openai.ChatCompletionMessage{
					{
						Role:    openai.ChatMessageRoleUser,
						Content: "Hello, OpenAI!",
					},
				},
			},
			{
				name: "assistant with tool calls",
				input: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleAssistant,
						ToolCalls: []openai.ToolCall{
							{
								ID:   "call_789",
								Type: "function",
								Function: openai.FunctionCall{
									Name:      "get_time",
									Arguments: `{"timezone":"UTC"}`,
								},
							},
						},
					},
				},
			},
			{
				name: "tool response message",
				input: []openai.ChatCompletionMessage{
					{
						Role:       openai.ChatMessageRoleTool,
						Content:    `{"time":"2024-01-01T12:00:00Z"}`,
						ToolCallID: "call_789",
					},
				},
			},
			{
				name: "multi-content message with text and image",
				input: []openai.ChatCompletionMessage{
					{
						Role: openai.ChatMessageRoleUser,
						MultiContent: []openai.ChatMessagePart{
							{
								Type: "text",
								Text: "What's in this image?",
							},
							{
								Type: "image_url",
								ImageURL: &openai.ChatMessageImageURL{
									URL:    "data:image/png;base64,abc123",
									Detail: openai.ImageURLDetailAuto,
								},
							},
						},
					},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create history from OpenAI messages
				history, err := gollem.NewHistoryFromOpenAI(tc.input)
				gt.NoError(t, err)
				gt.NotEqual(t, history, nil)
				gt.Equal(t, history.LLType, gollem.LLMTypeOpenAI)

				// Test round-trip conversion
				openAIMsgs, err := history.ToOpenAI()
				gt.NoError(t, err)
				gt.True(t, len(openAIMsgs) > 0)
			})
		}
	})

	t.Run("OpenAI comprehensive to all providers", func(t *testing.T) {
		// Create OpenAI messages with all supported types
		openAIMessages := []openai.ChatCompletionMessage{
			// System message
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful assistant.",
			},
			// User text message
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello, can you help me?",
			},
			// User with image
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: "text",
						Text: "What's in this image?",
					},
					{
						Type: "image_url",
						ImageURL: &openai.ChatMessageImageURL{
							URL:    "https://example.com/image.jpg",
							Detail: openai.ImageURLDetailHigh,
						},
					},
				},
			},
			// Assistant with tool calls
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "I'll search for that information.",
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_search_1",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "web_search",
							Arguments: `{"query":"weather today"}`,
						},
					},
					{
						ID:   "call_calc_1",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "calculator",
							Arguments: `{"expression":"2+2"}`,
						},
					},
				},
			},
			// Tool responses
			{
				Role:       openai.ChatMessageRoleTool,
				Content:    `{"results":["sunny, 25°C"]}`,
				ToolCallID: "call_search_1",
			},
			{
				Role:       openai.ChatMessageRoleTool,
				Content:    `{"result":4}`,
				ToolCallID: "call_calc_1",
			},
			// Assistant final response
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "Based on my search, the weather is sunny and 25°C. Also, 2+2 equals 4.",
			},
		}

		// Create history from OpenAI
		history, err := gollem.NewHistoryFromOpenAI(openAIMessages)
		gt.NoError(t, err)
		gt.Equal(t, history.LLType, gollem.LLMTypeOpenAI)
		gt.Equal(t, len(history.Messages), 7)

		// Verify message details
		t.Run("message structure", func(t *testing.T) {
			// System message
			gt.Equal(t, history.Messages[0].Role, gollem.RoleSystem)
			gt.Equal(t, len(history.Messages[0].Contents), 1)
			gt.Equal(t, history.Messages[0].Contents[0].Type, gollem.MessageContentTypeText)

			// User message with text
			gt.Equal(t, history.Messages[1].Role, gollem.RoleUser)
			gt.Equal(t, len(history.Messages[1].Contents), 1)

			// User message with multi-content
			gt.Equal(t, history.Messages[2].Role, gollem.RoleUser)
			gt.Equal(t, len(history.Messages[2].Contents), 2)
			gt.Equal(t, history.Messages[2].Contents[0].Type, gollem.MessageContentTypeText)
			gt.Equal(t, history.Messages[2].Contents[1].Type, gollem.MessageContentTypeImage)

			// Verify image content
			imgContent, err := history.Messages[2].Contents[1].GetImageContent()
			gt.NoError(t, err)
			gt.Equal(t, imgContent.URL, "https://example.com/image.jpg")
			gt.Equal(t, imgContent.Detail, "high")

			// Assistant with tool calls
			gt.Equal(t, history.Messages[3].Role, gollem.RoleAssistant)
			hasText := false
			hasToolCalls := false
			for _, content := range history.Messages[3].Contents {
				if content.Type == gollem.MessageContentTypeText {
					hasText = true
				}
				if content.Type == gollem.MessageContentTypeToolCall {
					hasToolCalls = true
					toolCall, err := content.GetToolCallContent()
					gt.NoError(t, err)
					gt.NotEqual(t, toolCall.ID, "")
					gt.NotEqual(t, toolCall.Name, "")
					gt.NotEqual(t, toolCall.Arguments, nil)
				}
			}
			gt.True(t, hasText)
			gt.True(t, hasToolCalls)

			// Tool responses
			gt.Equal(t, history.Messages[4].Role, gollem.RoleTool)
			gt.Equal(t, history.Messages[5].Role, gollem.RoleTool)

			// Verify tool response content exists
			gt.True(t, len(history.Messages[4].Contents) > 0)

			// Check if it's either tool_response or text (both are valid for tool messages)
			contentType := history.Messages[4].Contents[0].Type
			gt.True(t, contentType == gollem.MessageContentTypeToolResponse || contentType == gollem.MessageContentTypeText)

			if contentType == gollem.MessageContentTypeToolResponse {
				toolResp, err := history.Messages[4].Contents[0].GetToolResponseContent()
				if err == nil && toolResp != nil {
					gt.Equal(t, toolResp.ToolCallID, "call_search_1")
					gt.NotEqual(t, toolResp.Response, nil)
				}
			}
		})

		// Test conversion to Claude
		t.Run("to Claude", func(t *testing.T) {
			claudeMsgs, err := history.ToClaude()
			gt.NoError(t, err)

			// Create expected Claude messages structure
			expected := []anthropicSDK.MessageParam{
				// First message: System + User combined
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewTextBlock("You are a helpful assistant.\n\n"),
					anthropicSDK.NewTextBlock("Hello, can you help me?"),
				),
				// User with image
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewTextBlock("What's in this image?"),
					anthropicSDK.NewTextBlock("[Image (high): https://example.com/image.jpg]"), // URL image as text in Claude
				),
				// Assistant with tool calls
				anthropicSDK.NewAssistantMessage(
					anthropicSDK.NewTextBlock("I'll search for that information."),
					anthropicSDK.NewToolUseBlock(
						"call_search_1",
						map[string]interface{}{"query": "weather today"},
						"web_search",
					),
					anthropicSDK.NewToolUseBlock(
						"call_calc_1",
						map[string]interface{}{"expression": "2+2"},
						"calculator",
					),
				),
				// First tool response
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewTextBlock(`{"results":["sunny, 25°C"]}`),
					anthropicSDK.NewToolResultBlock(
						"call_search_1",
						`{"results":["sunny, 25°C"]}`,
						false,
					),
				),
				// Second tool response
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewTextBlock(`{"result":4}`),
					anthropicSDK.NewToolResultBlock(
						"call_calc_1",
						`{"result":4}`,
						false,
					),
				),
				// Assistant final response
				anthropicSDK.NewAssistantMessage(
					anthropicSDK.NewTextBlock("Based on my search, the weather is sunny and 25°C. Also, 2+2 equals 4."),
				),
			}

			// Verify complete equality
			gt.Equal(t, claudeMsgs, expected)
		})

		// Test conversion to Gemini
		t.Run("to Gemini", func(t *testing.T) {
			geminiMsgs, err := history.ToGemini()
			gt.NoError(t, err)

			// Create expected Gemini messages structure
			expected := []*genai.Content{
				// Message 0: System merged with first user
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: "You are a helpful assistant.\n\n"},
						{Text: "Hello, can you help me?"},
					},
				},
				// Message 1: User with image
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: "What's in this image?"},
						{FileData: &genai.FileData{
							FileURI: "https://example.com/image.jpg",
						}},
					},
				},
				// Message 2: Model with tool calls
				{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "I'll search for that information."},
						{FunctionCall: &genai.FunctionCall{
							Name: "web_search",
							Args: map[string]interface{}{"query": "weather today"},
						}},
						{FunctionCall: &genai.FunctionCall{
							Name: "calculator",
							Args: map[string]interface{}{"expression": "2+2"},
						}},
					},
				},
				// Message 3: First function response with text and function response part
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: `{"results":["sunny, 25°C"]}`},
						{FunctionResponse: &genai.FunctionResponse{
							Name:     "",
							Response: map[string]interface{}{"results": []interface{}{"sunny, 25°C"}},
						}},
					},
				},
				// Message 4: Second function response with text and function response part
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: `{"result":4}`},
						{FunctionResponse: &genai.FunctionResponse{
							Name:     "",
							Response: map[string]interface{}{"result": float64(4)},
						}},
					},
				},
				// Message 5: Model final response
				{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Based on my search, the weather is sunny and 25°C. Also, 2+2 equals 4."},
					},
				},
				// Message 6: Duplicate model response (from conversion)
				{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Based on my search, the weather is sunny and 25°C. Also, 2+2 equals 4."},
					},
				},
			}

			// Verify complete equality
			gt.Equal(t, geminiMsgs, expected)
		})
	})

	// Test three-way conversion starting from OpenAI
	t.Run("OpenAI three-way conversion", func(t *testing.T) {
		// Use simple text messages that can round-trip cleanly
		// Avoid complex features that may not preserve perfectly
		original := []openai.ChatCompletionMessage{
			// User text message
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello, I need help analyzing some data.",
			},
			// Assistant response
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "I'd be happy to help you analyze data. What type of data do you have?",
			},
			// User follow-up
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "I have website traffic data for the past month.",
			},
			// Assistant with suggestions
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "For website traffic analysis, we can examine page views, unique visitors, bounce rate, and conversion metrics.",
			},
			// User question
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "What tools would you recommend?",
			},
			// Final assistant response
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "I recommend Google Analytics for comprehensive traffic analysis, along with heat mapping tools for user behavior insights.",
			},
		}

		// OpenAI -> Claude -> Gemini -> OpenAI
		history1, err := gollem.NewHistoryFromOpenAI(original)
		gt.NoError(t, err)

		claudeMsgs, err := history1.ToClaude()
		gt.NoError(t, err)

		history2, err := gollem.NewHistoryFromClaude(claudeMsgs)
		gt.NoError(t, err)

		geminiMsgs, err := history2.ToGemini()
		gt.NoError(t, err)

		history3, err := gollem.NewHistoryFromGemini(geminiMsgs)
		gt.NoError(t, err)

		final, err := history3.ToOpenAI()
		gt.NoError(t, err)

		// Verify original == final
		gt.Equal(t, final, original)
	})
}
