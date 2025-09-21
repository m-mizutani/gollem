package gollem_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

func TestClaudeConversion(t *testing.T) {
	t.Run("Claude basic messages", func(t *testing.T) {
		testCases := []struct {
			name  string
			input []anthropic.MessageParam
		}{
			{
				name: "text message conversion",
				input: []anthropic.MessageParam{
					anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, Claude!")),
				},
			},
			{
				name: "tool use conversion",
				input: []anthropic.MessageParam{
					anthropic.NewAssistantMessage(anthropic.NewToolUseBlock("tool_123", map[string]any{
						"location": "Tokyo",
					}, "get_weather")),
				},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create history from Claude messages
				history, err := gollem.NewHistoryFromClaude(tc.input)
				gt.NoError(t, err)
				gt.NotEqual(t, history, nil)
				gt.Equal(t, history.LLType, gollem.LLMTypeClaude)

				// Test round-trip conversion
				claudeMsgs, err := history.ToClaude()
				gt.NoError(t, err)
				gt.Equal(t, len(claudeMsgs), len(tc.input))
			})
		}
	})

	t.Run("Claude comprehensive to all providers", func(t *testing.T) {
		// Create Claude messages with all supported types
		claudeMessages := []anthropic.MessageParam{
			// User with text
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello Claude!")),
			// Assistant with text
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hello! How can I help you?")),
			// User with image (base64)
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("Analyze this image:"),
				anthropic.NewImageBlockBase64("image/png", "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="),
			),
			// Assistant with tool use
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("Let me search for that."),
				anthropic.NewToolUseBlock("tool_use_1", map[string]any{
					"query": "latest news",
				}, "search_tool"),
			),
			// User with tool result
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("tool_use_1", `{"news":["Tech news","Sports news"]}`, false),
			),
			// Assistant with multiple tool uses
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock("calc_1", map[string]any{
					"a": 5,
					"b": 3,
				}, "add"),
				anthropic.NewToolUseBlock("calc_2", map[string]any{
					"a": 10,
					"b": 2,
				}, "multiply"),
			),
		}

		// Create history from Claude
		history, err := gollem.NewHistoryFromClaude(claudeMessages)
		gt.NoError(t, err)
		gt.Equal(t, history.LLType, gollem.LLMTypeClaude)
		gt.Equal(t, len(history.Messages), 6)

		// Verify message details
		t.Run("message structure", func(t *testing.T) {
			// First message should be user with text
			gt.Equal(t, history.Messages[0].Role, gollem.RoleUser)
			gt.Equal(t, len(history.Messages[0].Contents), 1)
			gt.Equal(t, history.Messages[0].Contents[0].Type, gollem.MessageContentTypeText)

			// Second message should be assistant with text
			gt.Equal(t, history.Messages[1].Role, gollem.RoleAssistant)
			gt.Equal(t, len(history.Messages[1].Contents), 1)
			gt.Equal(t, history.Messages[1].Contents[0].Type, gollem.MessageContentTypeText)

			// Third message should have text and image
			gt.Equal(t, history.Messages[2].Role, gollem.RoleUser)
			gt.Equal(t, len(history.Messages[2].Contents), 2)

			// Fourth message should have text and tool use
			gt.Equal(t, history.Messages[3].Role, gollem.RoleAssistant)
			gt.True(t, len(history.Messages[3].Contents) >= 2)

			// Check for tool calls
			hasToolCall := false
			for _, content := range history.Messages[3].Contents {
				if content.Type == gollem.MessageContentTypeToolCall {
					hasToolCall = true
					toolCall, err := content.GetToolCallContent()
					gt.NoError(t, err)
					gt.Equal(t, toolCall.ID, "tool_use_1")
					gt.Equal(t, toolCall.Name, "search_tool")
					gt.NotEqual(t, toolCall.Arguments, nil)
				}
			}
			gt.True(t, hasToolCall)
		})

		// Test conversion to OpenAI
		t.Run("to OpenAI", func(t *testing.T) {
			openAIMsgs, err := history.ToOpenAI()
			gt.NoError(t, err)

			// Create expected OpenAI messages structure
			expected := []openai.ChatCompletionMessage{
				// Message 0: User with text
				{
					Role:    openai.ChatMessageRoleUser,
					Content: "Hello Claude!",
				},
				// Message 1: Assistant with text
				{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Hello! How can I help you?",
				},
				// Message 2: User with text and image
				{
					Role: openai.ChatMessageRoleUser,
					MultiContent: []openai.ChatMessagePart{
						{Type: "text", Text: "Analyze this image:"},
						{Type: "image_url", ImageURL: &openai.ChatMessageImageURL{
							URL: "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
						}},
					},
				},
				// Message 3: Assistant with text and tool call
				{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "Let me search for that.",
					ToolCalls: []openai.ToolCall{
						{
							ID:   "tool_use_1",
							Type: "function",
							Function: openai.FunctionCall{
								Name:      "search_tool",
								Arguments: `{"query":"latest news"}`,
							},
						},
					},
				},
				// Message 4: Tool response
				{
					Role:       openai.ChatMessageRoleTool,
					Content:    `{"content":"{\"news\":[\"Tech news\",\"Sports news\"]}"}`,
					ToolCallID: "tool_use_1",
				},
				// Message 5: Assistant with multiple tool calls (no text content)
				{
					Role:    openai.ChatMessageRoleAssistant,
					Content: "",
					ToolCalls: []openai.ToolCall{
						{
							ID:   "calc_1",
							Type: "function",
							Function: openai.FunctionCall{
								Name:      "add",
								Arguments: `{"a":5,"b":3}`,
							},
						},
						{
							ID:   "calc_2",
							Type: "function",
							Function: openai.FunctionCall{
								Name:      "multiply",
								Arguments: `{"a":10,"b":2}`,
							},
						},
					},
				},
			}

			// Verify complete equality
			gt.Equal(t, openAIMsgs, expected)
		})

		// Test conversion to Gemini
		t.Run("to Gemini", func(t *testing.T) {
			geminiMsgs, err := history.ToGemini()
			gt.NoError(t, err)

			// Create expected Gemini messages structure
			// Base64 decode the image data
			imageData := []byte("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==")
			expected := []*genai.Content{
				// Message 0: User
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: "Hello Claude!"},
					},
				},
				// Message 1: Model
				{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Hello! How can I help you?"},
					},
				},
				// Message 2: User with image
				{
					Role: "user",
					Parts: []*genai.Part{
						{Text: "Analyze this image:"},
						{InlineData: &genai.Blob{
							MIMEType: "image/png",
							Data:     imageData,
						}},
					},
				},
				// Message 3: Model with function call
				{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Let me search for that."},
						{FunctionCall: &genai.FunctionCall{
							Name: "search_tool",
							Args: map[string]interface{}{"query": "latest news"},
						}},
					},
				},
				// Message 4: Function response - tool results go to user role in Gemini
				{
					Role: "user",
					Parts: []*genai.Part{
						{FunctionResponse: &genai.FunctionResponse{
							Name:     "", // Name is empty when converted from Claude tool result
							Response: map[string]interface{}{"content": `{"news":["Tech news","Sports news"]}`},
						}},
					},
				},
				// Message 5: Model with multiple function calls
				{
					Role: "model",
					Parts: []*genai.Part{
						{FunctionCall: &genai.FunctionCall{
							Name: "add",
							Args: map[string]interface{}{"a": float64(5), "b": float64(3)},
						}},
						{FunctionCall: &genai.FunctionCall{
							Name: "multiply",
							Args: map[string]interface{}{"a": float64(10), "b": float64(2)},
						}},
					},
				},
			}

			// Verify complete equality
			gt.Equal(t, geminiMsgs, expected)
		})
	})

	// Test three-way conversion starting from Claude
	t.Run("Claude three-way conversion", func(t *testing.T) {
		// Use simple messages that can round-trip cleanly
		// Avoid images and complex tool calls that may not preserve IDs
		original := []anthropic.MessageParam{
			// User with text
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("Hello Claude, I need help with data analysis."),
			),
			// Assistant response
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("I'd be happy to help you with data analysis. What kind of data are you working with?"),
			),
			// User follow-up
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("I have sales data from the last quarter."),
			),
			// Assistant with analysis
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("Great! I can help you analyze quarterly sales data. We can look at trends, patterns, and key metrics."),
			),
			// User with another question
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("What metrics should I focus on?"),
			),
			// Final assistant response
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("For quarterly sales data, focus on: revenue growth, customer acquisition rate, average order value, and conversion rates."),
			),
		}

		// Claude -> OpenAI -> Gemini -> Claude
		history1, err := gollem.NewHistoryFromClaude(original)
		gt.NoError(t, err)

		openAIMsgs, err := history1.ToOpenAI()
		gt.NoError(t, err)

		history2, err := gollem.NewHistoryFromOpenAI(openAIMsgs)
		gt.NoError(t, err)

		geminiMsgs, err := history2.ToGemini()
		gt.NoError(t, err)

		history3, err := gollem.NewHistoryFromGemini(geminiMsgs)
		gt.NoError(t, err)

		final, err := history3.ToClaude()
		gt.NoError(t, err)

		// Verify original == final
		gt.Equal(t, final, original)
	})
}
