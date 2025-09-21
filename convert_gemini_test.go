package gollem_test

import (
	"testing"

	anthropicSDK "github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

func TestGeminiConversion(t *testing.T) {
	t.Run("Gemini basic messages", func(t *testing.T) {
		testCases := []struct {
			name  string
			input []*genai.Content
		}{
			{
				name: "simple text message",
				input: []*genai.Content{
					{
						Role: "user",
						Parts: []*genai.Part{
							{Text: "Hello, Gemini!"},
						},
					},
				},
			},
			{
				name: "model with function call",
				input: []*genai.Content{
					{
						Role: "model",
						Parts: []*genai.Part{
							{
								FunctionCall: &genai.FunctionCall{
									Name: "get_weather",
									Args: map[string]interface{}{
										"location": "Tokyo",
									},
								},
							},
						},
					},
				},
			},
			{
				name: "function response",
				input: []*genai.Content{
					{
						Role: "function",
						Parts: []*genai.Part{
							{
								FunctionResponse: &genai.FunctionResponse{
									Name: "get_weather",
									Response: map[string]interface{}{
										"temperature": 25,
										"condition":   "sunny",
									},
								},
							},
						},
					},
				},
			},
			{
				name: "user with inline image data",
				input: []*genai.Content{
					{
						Role: "user",
						Parts: []*genai.Part{
							{Text: "Analyze this image:"},
							{
								InlineData: &genai.Blob{
									MIMEType: "image/png",
									Data:     []byte("test image data"),
								},
							},
						},
					},
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create history from Gemini messages
				history, err := gollem.NewHistoryFromGemini(tc.input)
				gt.NoError(t, err)
				gt.NotEqual(t, history, nil)
				gt.Equal(t, history.LLType, gollem.LLMTypeGemini)

				// Test round-trip conversion
				geminiMsgs, err := history.ToGemini()
				gt.NoError(t, err)
				gt.Equal(t, len(geminiMsgs), len(tc.input))
			})
		}
	})

	t.Run("Gemini comprehensive to all providers", func(t *testing.T) {
		// Create Gemini messages with all supported types
		geminiMessages := []*genai.Content{
			// User with text
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Hello Gemini! Can you help me?"},
				},
			},
			// Model response
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Of course! I'd be happy to help."},
				},
			},
			// User with text and image
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "What's in this image?"},
					{
						InlineData: &genai.Blob{
							MIMEType: "image/jpeg",
							Data:     []byte("fake image data for testing"),
						},
					},
				},
			},
			// Model with function call
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Let me analyze that for you."},
					{
						FunctionCall: &genai.FunctionCall{
							Name: "image_analyzer",
							Args: map[string]interface{}{
								"mode":  "detailed",
								"focus": "objects",
							},
						},
					},
					{
						FunctionCall: &genai.FunctionCall{
							Name: "web_search",
							Args: map[string]interface{}{
								"query": "similar images",
							},
						},
					},
				},
			},
			// Function responses
			{
				Role: "function",
				Parts: []*genai.Part{
					{
						FunctionResponse: &genai.FunctionResponse{
							Name: "image_analyzer",
							Response: map[string]interface{}{
								"objects":    []interface{}{"cat", "dog"},
								"confidence": 0.95,
							},
						},
					},
				},
			},
			{
				Role: "function",
				Parts: []*genai.Part{
					{
						FunctionResponse: &genai.FunctionResponse{
							Name: "web_search",
							Response: map[string]interface{}{
								"results": []interface{}{
									map[string]interface{}{
										"title": "Similar pets",
										"url":   "https://example.com",
									},
								},
							},
						},
					},
				},
			},
			// Model final response
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "The image contains a cat and a dog. I found similar images online."},
				},
			},
		}

		// Create history from Gemini
		history, err := gollem.NewHistoryFromGemini(geminiMessages)
		gt.NoError(t, err)
		gt.Equal(t, history.LLType, gollem.LLMTypeGemini)
		gt.Equal(t, len(history.Messages), 7)

		// Verify message details
		t.Run("message structure", func(t *testing.T) {
			// First user message
			gt.Equal(t, history.Messages[0].Role, gollem.RoleUser)
			gt.Equal(t, len(history.Messages[0].Contents), 1)
			gt.Equal(t, history.Messages[0].Contents[0].Type, gollem.MessageContentTypeText)
			textContent, err := history.Messages[0].Contents[0].GetTextContent()
			gt.NoError(t, err)
			gt.Equal(t, textContent.Text, "Hello Gemini! Can you help me?")

			// Model response
			gt.Equal(t, history.Messages[1].Role, gollem.RoleModel)
			gt.Equal(t, len(history.Messages[1].Contents), 1)

			// User with image
			gt.Equal(t, history.Messages[2].Role, gollem.RoleUser)
			gt.Equal(t, len(history.Messages[2].Contents), 2)
			gt.Equal(t, history.Messages[2].Contents[0].Type, gollem.MessageContentTypeText)
			gt.Equal(t, history.Messages[2].Contents[1].Type, gollem.MessageContentTypeImage)

			imgContent, err := history.Messages[2].Contents[1].GetImageContent()
			gt.NoError(t, err)
			gt.Equal(t, imgContent.MediaType, "image/jpeg")
			gt.Equal(t, imgContent.Data, []byte("fake image data for testing"))

			// Model with function calls
			gt.Equal(t, history.Messages[3].Role, gollem.RoleModel)
			gt.Equal(t, len(history.Messages[3].Contents), 3) // text + 2 function calls

			// Text content
			gt.Equal(t, history.Messages[3].Contents[0].Type, gollem.MessageContentTypeText)
			textContent4, err := history.Messages[3].Contents[0].GetTextContent()
			gt.NoError(t, err)
			gt.Equal(t, textContent4.Text, "Let me analyze that for you.")

			// First function/tool call
			gt.Equal(t, history.Messages[3].Contents[1].Type, gollem.MessageContentTypeToolCall)
			toolCall1, err := history.Messages[3].Contents[1].GetToolCallContent()
			gt.NoError(t, err)
			gt.Equal(t, toolCall1.Name, "image_analyzer")
			expectedArgs1 := map[string]any{"focus": "objects", "mode": "detailed"}
			gt.Equal(t, toolCall1.Arguments, expectedArgs1)

			// Second function/tool call
			gt.Equal(t, history.Messages[3].Contents[2].Type, gollem.MessageContentTypeToolCall)
			toolCall2, err := history.Messages[3].Contents[2].GetToolCallContent()
			gt.NoError(t, err)
			gt.Equal(t, toolCall2.Name, "web_search")
			expectedArgs2 := map[string]any{"query": "similar images"}
			gt.Equal(t, toolCall2.Arguments, expectedArgs2)

			// Tool responses (function role in Gemini)
			gt.Equal(t, history.Messages[4].Role, gollem.RoleFunction)
			gt.Equal(t, len(history.Messages[4].Contents), 1)
			toolResp1, err := history.Messages[4].Contents[0].GetToolResponseContent()
			gt.NoError(t, err)
			gt.Equal(t, toolResp1.Name, "image_analyzer")
			expectedResp1 := map[string]any{"confidence": 0.95, "objects": []any{"cat", "dog"}}
			gt.Equal(t, toolResp1.Response, expectedResp1)

			gt.Equal(t, history.Messages[5].Role, gollem.RoleFunction)
			gt.Equal(t, len(history.Messages[5].Contents), 1)
			toolResp2, err := history.Messages[5].Contents[0].GetToolResponseContent()
			gt.NoError(t, err)
			gt.Equal(t, toolResp2.Name, "web_search")
			expectedResp2 := map[string]any{
				"results": []any{
					map[string]any{"title": "Similar pets", "url": "https://example.com"},
				},
			}
			gt.Equal(t, toolResp2.Response, expectedResp2)

			// Final model response
			gt.Equal(t, history.Messages[6].Role, gollem.RoleModel)
			gt.Equal(t, len(history.Messages[6].Contents), 1)
			textContent5, err := history.Messages[6].Contents[0].GetTextContent()
			gt.NoError(t, err)
			gt.Equal(t, textContent5.Text, "The image contains a cat and a dog. I found similar images online.")
		})

		// Test conversion to Claude
		t.Run("to Claude", func(t *testing.T) {
			claudeMsgs, err := history.ToClaude()
			gt.NoError(t, err)

			// Create expected Claude messages structure
			expected := []anthropicSDK.MessageParam{
				// Message 0: User
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewTextBlock("Hello Gemini! Can you help me?"),
				),
				// Message 1: Assistant
				anthropicSDK.NewAssistantMessage(
					anthropicSDK.NewTextBlock("Of course! I'd be happy to help."),
				),
				// Message 2: User with image (base64 encoded)
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewTextBlock("What's in this image?"),
					anthropicSDK.NewImageBlockBase64("image/jpeg", "fake image data for testing"),
				),
				// Message 3: Assistant with tool uses
				anthropicSDK.NewAssistantMessage(
					anthropicSDK.NewTextBlock("Let me analyze that for you."),
					anthropicSDK.NewToolUseBlock(
						"call_image_analyzer_0",
						map[string]interface{}{"mode": "detailed", "focus": "objects"},
						"image_analyzer",
					),
					anthropicSDK.NewToolUseBlock(
						"call_web_search_0",
						map[string]interface{}{"query": "similar images"},
						"web_search",
					),
				),
				// Message 4: Tool result 1
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewToolResultBlock(
						"call_image_analyzer_0",
						`{"confidence":0.95,"objects":["cat","dog"]}`,
						false,
					),
				),
				// Message 5: Tool result 2
				anthropicSDK.NewUserMessage(
					anthropicSDK.NewToolResultBlock(
						"call_web_search_0",
						`{"results":[{"title":"Similar pets","url":"https://example.com"}]}`,
						false,
					),
				),
				// Message 6: Final assistant response
				anthropicSDK.NewAssistantMessage(
					anthropicSDK.NewTextBlock("The image contains a cat and a dog. I found similar images online."),
				),
			}

			// Verify complete equality
			gt.Equal(t, claudeMsgs, expected)
		})

		// Test conversion to OpenAI
		t.Run("to OpenAI", func(t *testing.T) {
			openAIMsgs, err := history.ToOpenAI()
			gt.NoError(t, err)

			// Create expected OpenAI messages structure
			// Note: Image data is stored as raw bytes which gets converted to data URL
			expected := []openai.ChatCompletionMessage{
				// Message 0: User
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hello Gemini! Can you help me?",
				},
				// Message 1: Assistant
				{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Of course! I'd be happy to help.",
				},
				// Message 2: User with image
				{
					Role: openai.ChatMessageRoleUser,
					MultiContent: []openai.ChatMessagePart{
						{Type: "text", Text: "What's in this image?"},
						{Type: "image_url", ImageURL: &openai.ChatMessageImageURL{
							URL: "data:image/jpeg;base64,fake image data for testing", // Raw bytes stored directly
						}},
					},
				},
				// Message 3: Assistant with tool calls
				{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Let me analyze that for you.",
					ToolCalls: []openai.ToolCall{
						{
							ID:   "call_image_analyzer_0",
							Type: "function",
							Function: openai.FunctionCall{
								Name:      "image_analyzer",
								Arguments: `{"focus":"objects","mode":"detailed"}`,
							},
						},
						{
							ID:   "call_web_search_0",
							Type: "function",
							Function: openai.FunctionCall{
								Name:      "web_search",
								Arguments: `{"query":"similar images"}`,
							},
						},
					},
				},
				// Message 4: Tool response 1 (with Name field set)
				{
					Role:       openai.ChatMessageRoleTool,
					Name:       "image_analyzer",
					Content:    `{"confidence":0.95,"objects":["cat","dog"]}`,
					ToolCallID: "call_image_analyzer_0",
				},
				// Message 5: Tool response 2 (with Name field set)
				{
					Role:       openai.ChatMessageRoleTool,
					Name:       "web_search",
					Content:    `{"results":[{"title":"Similar pets","url":"https://example.com"}]}`,
					ToolCallID: "call_web_search_0",
				},
				// Message 6: Final assistant response
				{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "The image contains a cat and a dog. I found similar images online.",
				},
			}

			// Verify complete equality
			gt.Equal(t, openAIMsgs, expected)
		})
	})

	// Test three-way conversion starting from Gemini
	t.Run("Gemini three-way conversion", func(t *testing.T) {
		// Use simple messages that can round-trip cleanly
		// Text-only messages work best for three-way conversions
		original := []*genai.Content{
			// User with text
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Hello Gemini, can you help me analyze some data?"},
				},
			},
			// Model response
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Of course! I'd be happy to help you analyze data. What type of data are you working with?"},
				},
			},
			// User follow-up
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "I have customer feedback data from our surveys."},
				},
			},
			// Model with suggestion
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Great! For customer feedback analysis, we can look at sentiment, themes, and satisfaction scores."},
				},
			},
			// User asking for specifics
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "How should I categorize the feedback?"},
				},
			},
			// Final model response
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Categorize feedback by: product features, customer service, pricing, and user experience."},
				},
			},
		}

		// Gemini -> Claude -> OpenAI -> Gemini
		history1, err := gollem.NewHistoryFromGemini(original)
		gt.NoError(t, err)

		claudeMsgs, err := history1.ToClaude()
		gt.NoError(t, err)

		history2, err := gollem.NewHistoryFromClaude(claudeMsgs)
		gt.NoError(t, err)

		openAIMsgs, err := history2.ToOpenAI()
		gt.NoError(t, err)

		history3, err := gollem.NewHistoryFromOpenAI(openAIMsgs)
		gt.NoError(t, err)

		final, err := history3.ToGemini()
		gt.NoError(t, err)

		// Verify original == final
		gt.Equal(t, final, original)
	})
}
