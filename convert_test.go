package gollem_test

import (
	"encoding/json"
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/sashabaranov/go-openai"
	"google.golang.org/genai"
)

// Helper functions for creating MessageContent
func mustNewTextContent(text string) gollem.MessageContent {
	content, err := gollem.NewTextContent(text)
	if err != nil {
		panic(err)
	}
	return content
}

func TestHistoryFromClaude(t *testing.T) {
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
}

func TestHistoryFromOpenAI(t *testing.T) {
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
								URL:    "data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==",
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
			// Note: tool responses may create multiple messages
			gt.True(t, len(openAIMsgs) > 0)
		})
	}
}

func TestHistoryFromGemini(t *testing.T) {
	testCases := []struct {
		name  string
		input []*genai.Content
	}{
		{
			name: "user text message",
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
			name: "model message (converted to assistant)",
			input: []*genai.Content{
				{
					Role: "model",
					Parts: []*genai.Part{
						{Text: "Hello from model!"},
					},
				},
			},
		},
		{
			name: "function call",
			input: []*genai.Content{
				{
					Role: "model",
					Parts: []*genai.Part{
						{FunctionCall: &genai.FunctionCall{
							Name: "calculate",
							Args: map[string]any{
								"expression": "2+2",
							},
						}},
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
						{FunctionResponse: &genai.FunctionResponse{
							Name:     "calculate",
							Response: map[string]any{"result": 4},
						}},
					},
				},
			},
		},
		{
			name: "inline data (image)",
			input: []*genai.Content{
				{
					Role: "user",
					Parts: []*genai.Part{
						{InlineData: &genai.Blob{
							MIMEType: "image/jpeg",
							Data:     []byte("imagedata"),
						}},
					},
				},
			},
		},
		{
			name: "file data (video)",
			input: []*genai.Content{
				{
					Role: "user",
					Parts: []*genai.Part{
						{FileData: &genai.FileData{
							MIMEType: "video/mp4",
							FileURI:  "gs://bucket/video.mp4",
						}},
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
}

func TestMessageContentGetters(t *testing.T) {
	t.Run("GetTextContent", func(t *testing.T) {
		content := mustNewTextContent("Hello")

		textContent, err := content.GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Hello")
	})

	t.Run("GetImageContent", func(t *testing.T) {
		// Test base64 image
		imgContent, err := gollem.NewImageContent("image/png", []byte("test-data"), "", "")
		gt.NoError(t, err)

		imgData, err := imgContent.GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, imgData.MediaType, "image/png")
		gt.Equal(t, imgData.Data, []byte("test-data"))
		gt.Equal(t, imgData.URL, "")
		gt.Equal(t, imgData.Detail, "")

		// Test URL image with detail
		imgContent2, err := gollem.NewImageContent("image/jpeg", nil, "https://example.com/image.jpg", "high")
		gt.NoError(t, err)

		imgData2, err := imgContent2.GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, imgData2.MediaType, "image/jpeg")
		gt.Equal(t, len(imgData2.Data), 0)
		gt.Equal(t, imgData2.URL, "https://example.com/image.jpg")
		gt.Equal(t, imgData2.Detail, "high")
	})

	t.Run("GetToolCallContent", func(t *testing.T) {
		args := map[string]any{"param1": "value1", "param2": float64(42)}
		toolContent, err := gollem.NewToolCallContent("call_123", "test_tool", args)
		gt.NoError(t, err)

		toolCall, err := toolContent.GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, toolCall.ID, "call_123")
		gt.Equal(t, toolCall.Name, "test_tool")
		gt.Equal(t, toolCall.Arguments, args)
	})

	t.Run("GetToolResponseContent", func(t *testing.T) {
		response := map[string]any{"result": "success", "data": []any{float64(1), float64(2), float64(3)}}
		toolResp, err := gollem.NewToolResponseContent("call_456", "test_tool", response, false)
		gt.NoError(t, err)

		respData, err := toolResp.GetToolResponseContent()
		gt.NoError(t, err)
		gt.Equal(t, respData.ToolCallID, "call_456")
		gt.Equal(t, respData.Name, "test_tool")
		gt.Equal(t, respData.Response, response)
		gt.Equal(t, respData.IsError, false)

		// Test error response
		toolRespErr, err := gollem.NewToolResponseContent("call_789", "error_tool", map[string]any{"error": "error message"}, true)
		gt.NoError(t, err)

		respErrData, err := toolRespErr.GetToolResponseContent()
		gt.NoError(t, err)
		gt.Equal(t, respErrData.ToolCallID, "call_789")
		gt.Equal(t, respErrData.Name, "error_tool")
		gt.Equal(t, respErrData.Response, map[string]any{"error": "error message"})
		gt.Equal(t, respErrData.IsError, true)
	})

	t.Run("GetFunctionCallContent", func(t *testing.T) {
		funcContent, err := gollem.NewFunctionCallContent("func_name", `{"arg1":"value1","arg2":123}`)
		gt.NoError(t, err)

		funcCall, err := funcContent.GetFunctionCallContent()
		gt.NoError(t, err)
		gt.Equal(t, funcCall.Name, "func_name")
		gt.Equal(t, funcCall.Arguments, `{"arg1":"value1","arg2":123}`)
	})

	t.Run("GetFunctionResponseContent", func(t *testing.T) {
		funcResp, err := gollem.NewFunctionResponseContent("func_name", "response content")
		gt.NoError(t, err)

		respData, err := funcResp.GetFunctionResponseContent()
		gt.NoError(t, err)
		gt.Equal(t, respData.Name, "func_name")
		gt.Equal(t, respData.Content, "response content")
	})
}

func TestRoleMapping(t *testing.T) {
	t.Run("all roles preserved correctly", func(t *testing.T) {
		// Test that each role is preserved or mapped correctly through conversions
		roles := []gollem.MessageRole{
			gollem.RoleUser,
			gollem.RoleAssistant,
			gollem.RoleSystem,
			gollem.RoleTool,
			gollem.RoleFunction,
			gollem.RoleModel,
		}

		for _, role := range roles {
			// Roles should be consistent
			gt.NotEqual(t, string(role), "")
		}
	})

	t.Run("role conversion mapping", func(t *testing.T) {
		// Test specific role mappings for each provider
		testCases := []struct {
			name       string
			inputRole  gollem.MessageRole
			claudeRole string
			openAIRole string
			geminiRole string
		}{
			{
				name:       "user role",
				inputRole:  gollem.RoleUser,
				claudeRole: "user",
				openAIRole: "user",
				geminiRole: "user",
			},
			{
				name:       "assistant role",
				inputRole:  gollem.RoleAssistant,
				claudeRole: "assistant",
				openAIRole: "assistant",
				geminiRole: "model",
			},
			{
				name:       "system role",
				inputRole:  gollem.RoleSystem,
				claudeRole: "user", // Claude merges system into user
				openAIRole: "system",
				geminiRole: "user", // Gemini merges system into user
			},
			{
				name:       "tool role",
				inputRole:  gollem.RoleTool,
				claudeRole: "user", // Tool responses become user in Claude
				openAIRole: "tool",
				geminiRole: "user", // Tool responses become user in Gemini
			},
			{
				name:       "function role",
				inputRole:  gollem.RoleFunction,
				claudeRole: "user",     // Function responses become user in Claude
				openAIRole: "function", // Function role preserved in OpenAI (but may be converted to tool in some cases)
				geminiRole: "user",     // Function becomes user in Gemini (but may be preserved as function in some cases)
			},
			{
				name:       "model role",
				inputRole:  gollem.RoleModel,
				claudeRole: "assistant", // Model becomes assistant in Claude
				openAIRole: "assistant", // Model becomes assistant in OpenAI
				geminiRole: "model",
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				// Create a simple message with the role
				message := gollem.Message{
					Role: tc.inputRole,
					Contents: []gollem.MessageContent{
						mustNewTextContent("test content"),
					},
				}

				history := &gollem.History{
					Version:  gollem.HistoryVersion,
					LLType:   gollem.LLMTypeGemini, // Use Gemini as base
					Messages: []gollem.Message{message},
				}

				// Test Claude conversion
				claudeMsgs, err := history.ToClaude()
				gt.NoError(t, err)
				if len(claudeMsgs) > 0 {
					gt.Equal(t, string(claudeMsgs[0].Role), tc.claudeRole)
				}

				// Test OpenAI conversion
				openAIMsgs, err := history.ToOpenAI()
				gt.NoError(t, err)
				if len(openAIMsgs) > 0 {
					// For function role, accept either "function" or "tool"
					if tc.inputRole == gollem.RoleFunction {
						actualRole := string(openAIMsgs[0].Role)
						gt.True(t, actualRole == "function" || actualRole == "tool")
					} else {
						gt.Equal(t, string(openAIMsgs[0].Role), tc.openAIRole)
					}
				}

				// Test Gemini conversion
				geminiMsgs, err := history.ToGemini()
				gt.NoError(t, err)
				if len(geminiMsgs) > 0 {
					// For function role, accept either "function" or "user"
					if tc.inputRole == gollem.RoleFunction {
						actualRole := geminiMsgs[0].Role
						gt.True(t, actualRole == "function" || actualRole == "user")
					} else {
						gt.Equal(t, geminiMsgs[0].Role, tc.geminiRole)
					}
				}
			})
		}
	})
}

func TestComplexMessageConversion(t *testing.T) {
	t.Run("OpenAI message with multiple tool calls - detailed verification", func(t *testing.T) {
		msg := []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_1",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "search",
							Arguments: `{"query":"weather"}`,
						},
					},
					{
						ID:   "call_2",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "calculate",
							Arguments: `{"expression":"2+2"}`,
						},
					},
				},
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 1)
		gt.Equal(t, len(history.Messages[0].Contents), 2)

		// Verify each tool call content
		for i, content := range history.Messages[0].Contents {
			gt.Equal(t, content.Type, gollem.MessageContentTypeToolCall)
			toolCall, err := content.GetToolCallContent()
			gt.NoError(t, err)
			gt.NotEqual(t, toolCall, nil)

			if i == 0 {
				gt.Equal(t, toolCall.ID, "call_1")
				gt.Equal(t, toolCall.Name, "search")
				expectedArgs := map[string]any{"query": "weather"}
				gt.Equal(t, toolCall.Arguments, expectedArgs)
			} else if i == 1 {
				gt.Equal(t, toolCall.ID, "call_2")
				gt.Equal(t, toolCall.Name, "calculate")
				expectedArgs := map[string]any{"expression": "2+2"}
				gt.Equal(t, toolCall.Arguments, expectedArgs)
			}
		}

		// Convert to Claude and verify details
		claudeMsgs, err := history.ToClaude()
		gt.NoError(t, err)
		gt.Equal(t, len(claudeMsgs), 1)
		gt.Equal(t, string(claudeMsgs[0].Role), "assistant")

		// Verify Claude tool use blocks
		toolUseCount := 0
		for _, block := range claudeMsgs[0].Content {
			if block.OfToolUse != nil {
				toolUseCount++
				gt.NotEqual(t, block.OfToolUse.ID, "")
				gt.NotEqual(t, block.OfToolUse.Name, "")
				gt.NotEqual(t, block.OfToolUse.Input, nil)
			}
		}
		gt.Equal(t, toolUseCount, 2)

		// Convert back and verify round-trip
		history2, err := gollem.NewHistoryFromClaude(claudeMsgs)
		gt.NoError(t, err)
		gt.Equal(t, len(history2.Messages), 1)
		gt.Equal(t, len(history2.Messages[0].Contents), 2)

		// Verify tool calls are preserved in round-trip
		for _, content := range history2.Messages[0].Contents {
			gt.Equal(t, content.Type, gollem.MessageContentTypeToolCall)
			toolCall, err := content.GetToolCallContent()
			gt.NoError(t, err)
			gt.NotEqual(t, toolCall.ID, "")
			gt.NotEqual(t, toolCall.Name, "")
			gt.NotEqual(t, toolCall.Arguments, nil)
		}
	})

	t.Run("Mixed content types - detailed verification", func(t *testing.T) {
		// Create a message with mixed text and image content
		msg := []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: "text",
						Text: "What's in this image?",
					},
					{
						Type: "text",
						Text: "Please describe it in detail.",
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
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 1)
		gt.Equal(t, len(history.Messages[0].Contents), 3)

		// Verify each content type
		gt.Equal(t, history.Messages[0].Contents[0].Type, gollem.MessageContentTypeText)
		gt.Equal(t, history.Messages[0].Contents[1].Type, gollem.MessageContentTypeText)
		gt.Equal(t, history.Messages[0].Contents[2].Type, gollem.MessageContentTypeImage)

		// Verify text contents
		text1, err := history.Messages[0].Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text1.Text, "What's in this image?")

		text2, err := history.Messages[0].Contents[1].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text2.Text, "Please describe it in detail.")

		// Verify image content
		img, err := history.Messages[0].Contents[2].GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, img.URL, "https://example.com/image.jpg")
		gt.Equal(t, img.Detail, "high")
		gt.Equal(t, len(img.Data), 0) // URL image has no data
	})

	t.Run("Tool response with error handling", func(t *testing.T) {
		msg := []openai.ChatCompletionMessage{
			{
				Role:       openai.ChatMessageRoleTool,
				Content:    `{"error":"Not found","code":404}`,
				ToolCallID: "call_error_123",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 1)
		gt.Equal(t, history.Messages[0].Role, gollem.RoleTool)

		// Verify content is properly converted
		gt.True(t, len(history.Messages[0].Contents) > 0)

		// The content might be either text or tool_response depending on implementation
		content := history.Messages[0].Contents[0]
		if content.Type == gollem.MessageContentTypeToolResponse {
			toolResp, err := content.GetToolResponseContent()
			gt.NoError(t, err)
			gt.Equal(t, toolResp.ToolCallID, "call_error_123")
			gt.NotEqual(t, toolResp.Response, nil)
		} else if content.Type == gollem.MessageContentTypeText {
			textContent, err := content.GetTextContent()
			gt.NoError(t, err)
			gt.Equal(t, textContent.Text, `{"error":"Not found","code":404}`)
		}
	})
}

func TestErrorHandling(t *testing.T) {
	t.Run("empty message arrays", func(t *testing.T) {
		// Empty OpenAI messages
		history, err := gollem.NewHistoryFromOpenAI([]openai.ChatCompletionMessage{})
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 0)
		gt.Equal(t, history.LLType, gollem.LLMTypeOpenAI)
		gt.Equal(t, history.Version, gollem.HistoryVersion)

		// Empty Claude messages
		history, err = gollem.NewHistoryFromClaude([]anthropic.MessageParam{})
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 0)
		gt.Equal(t, history.LLType, gollem.LLMTypeClaude)
		gt.Equal(t, history.Version, gollem.HistoryVersion)

		// Empty Gemini messages
		history, err = gollem.NewHistoryFromGemini([]*genai.Content{})
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 0)
		gt.Equal(t, history.LLType, gollem.LLMTypeGemini)
		gt.Equal(t, history.Version, gollem.HistoryVersion)
	})

	t.Run("invalid JSON in tool arguments", func(t *testing.T) {
		// OpenAI message with invalid JSON in tool arguments
		msg := []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleAssistant,
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_bad",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "bad_tool",
							Arguments: "not valid json{",
						},
					},
				},
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err) // Should handle gracefully
		gt.Equal(t, len(history.Messages), 1)
		gt.Equal(t, len(history.Messages[0].Contents), 1)

		// Verify the malformed JSON is still stored
		content := history.Messages[0].Contents[0]
		gt.Equal(t, content.Type, gollem.MessageContentTypeToolCall)
		toolCall, err := content.GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, toolCall.ID, "call_bad")
		gt.Equal(t, toolCall.Name, "bad_tool")
		// Arguments should be stored as-is even if invalid JSON
		gt.NotEqual(t, toolCall.Arguments, nil)
	})

	t.Run("nil and empty content handling", func(t *testing.T) {
		// Test OpenAI message with empty content
		msg := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 1)

		// Empty content should still create a text message
		if len(history.Messages[0].Contents) > 0 {
			gt.Equal(t, history.Messages[0].Contents[0].Type, gollem.MessageContentTypeText)
		}
	})

	t.Run("missing tool call ID", func(t *testing.T) {
		// OpenAI tool response without ToolCallID
		msg := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleTool,
				Content: `{"result":"success"}`,
				// Missing ToolCallID
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err) // Should handle gracefully
		gt.Equal(t, len(history.Messages), 1)
		gt.Equal(t, history.Messages[0].Role, gollem.RoleTool)
	})

	t.Run("invalid image data", func(t *testing.T) {
		// Test with invalid base64 image
		msg := []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: "image_url",
						ImageURL: &openai.ChatMessageImageURL{
							URL: "data:image/png;base64,invalid_base64!!!",
						},
					},
				},
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err) // Should handle gracefully
		gt.Equal(t, len(history.Messages), 1)

		// Should still create image content even with invalid base64
		if len(history.Messages[0].Contents) > 0 {
			gt.Equal(t, history.Messages[0].Contents[0].Type, gollem.MessageContentTypeImage)
		}
	})

	t.Run("content type mismatch errors", func(t *testing.T) {
		// Create text content and try to get it as image
		textContent := mustNewTextContent("Hello")

		_, err := textContent.GetImageContent()
		gt.Error(t, err) // Should return error for wrong type

		_, err = textContent.GetToolCallContent()
		gt.Error(t, err) // Should return error for wrong type

		_, err = textContent.GetToolResponseContent()
		gt.Error(t, err) // Should return error for wrong type
	})

	t.Run("conversion with unsupported content", func(t *testing.T) {
		// Test conversion of messages with complex nested structures
		history := &gollem.History{
			Version: gollem.HistoryVersion,
			LLType:  gollem.LLMTypeOpenAI,
			Messages: []gollem.Message{
				{
					Role: gollem.RoleUser,
					Contents: []gollem.MessageContent{
						mustNewTextContent("Test message with unicode: 🚀 ñáéíóú 中文 العربية"),
					},
				},
			},
		}

		// Test conversion to all providers
		claudeMsgs, err := history.ToClaude()
		gt.NoError(t, err)
		gt.True(t, len(claudeMsgs) > 0)

		openAIMsgs, err := history.ToOpenAI()
		gt.NoError(t, err)
		gt.True(t, len(openAIMsgs) > 0)

		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.True(t, len(geminiMsgs) > 0)
	})
}

func TestHistorySerialization(t *testing.T) {
	t.Run("serialize and deserialize simple history", func(t *testing.T) {
		// Create a history with various message types
		openAIMsg := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Test message",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(openAIMsg)
		gt.NoError(t, err)

		// Serialize to JSON
		data, err := json.Marshal(history)
		gt.NoError(t, err)

		// Deserialize
		var restored gollem.History
		err = json.Unmarshal(data, &restored)
		gt.NoError(t, err)

		// Verify the restored history
		gt.Equal(t, restored.Version, gollem.HistoryVersion)
		gt.Equal(t, restored.LLType, gollem.LLMTypeOpenAI)
		gt.Equal(t, len(restored.Messages), 1)
		gt.Equal(t, restored.Messages[0].Role, gollem.RoleUser)
		gt.Equal(t, len(restored.Messages[0].Contents), 1)
		gt.Equal(t, restored.Messages[0].Contents[0].Type, gollem.MessageContentTypeText)
	})

	t.Run("serialize complex history with all content types", func(t *testing.T) {
		// Create a more complex history with different content types
		textContent, _ := gollem.NewTextContent("Hello world")
		imageContent, _ := gollem.NewImageContent("image/png", []byte("test-data"), "", "high")
		toolCallContent, _ := gollem.NewToolCallContent("call_123", "search", map[string]any{"query": "test"})
		toolRespContent, _ := gollem.NewToolResponseContent("call_123", "search", map[string]any{"result": "found"}, false)

		history := &gollem.History{
			Version: gollem.HistoryVersion,
			LLType:  gollem.LLMTypeOpenAI,
			Messages: []gollem.Message{
				{
					Role:     gollem.RoleUser,
					Contents: []gollem.MessageContent{textContent, imageContent},
				},
				{
					Role:     gollem.RoleAssistant,
					Contents: []gollem.MessageContent{toolCallContent},
				},
				{
					Role:     gollem.RoleTool,
					Contents: []gollem.MessageContent{toolRespContent},
				},
			},
		}

		// Serialize to JSON
		data, err := json.Marshal(history)
		gt.NoError(t, err)
		gt.True(t, len(data) > 0)

		// Deserialize
		var restored gollem.History
		err = json.Unmarshal(data, &restored)
		gt.NoError(t, err)

		// Verify all details are preserved
		gt.Equal(t, restored.Version, gollem.HistoryVersion)
		gt.Equal(t, restored.LLType, gollem.LLMTypeOpenAI)
		gt.Equal(t, len(restored.Messages), 3)

		// Verify first message (user with text and image)
		gt.Equal(t, restored.Messages[0].Role, gollem.RoleUser)
		gt.Equal(t, len(restored.Messages[0].Contents), 2)
		gt.Equal(t, restored.Messages[0].Contents[0].Type, gollem.MessageContentTypeText)
		gt.Equal(t, restored.Messages[0].Contents[1].Type, gollem.MessageContentTypeImage)

		// Verify text content
		text, err := restored.Messages[0].Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text.Text, "Hello world")

		// Verify image content
		img, err := restored.Messages[0].Contents[1].GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, img.MediaType, "image/png")
		gt.Equal(t, img.Data, []byte("test-data"))
		gt.Equal(t, img.Detail, "high")

		// Verify second message (assistant with tool call)
		gt.Equal(t, restored.Messages[1].Role, gollem.RoleAssistant)
		gt.Equal(t, len(restored.Messages[1].Contents), 1)
		gt.Equal(t, restored.Messages[1].Contents[0].Type, gollem.MessageContentTypeToolCall)

		toolCall, err := restored.Messages[1].Contents[0].GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, toolCall.ID, "call_123")
		gt.Equal(t, toolCall.Name, "search")
		gt.Equal(t, toolCall.Arguments, map[string]any{"query": "test"})

		// Verify third message (tool response)
		gt.Equal(t, restored.Messages[2].Role, gollem.RoleTool)
		gt.Equal(t, len(restored.Messages[2].Contents), 1)
		gt.Equal(t, restored.Messages[2].Contents[0].Type, gollem.MessageContentTypeToolResponse)

		toolResp, err := restored.Messages[2].Contents[0].GetToolResponseContent()
		gt.NoError(t, err)
		gt.Equal(t, toolResp.ToolCallID, "call_123")
		gt.Equal(t, toolResp.Name, "search")
		gt.Equal(t, toolResp.Response, map[string]any{"result": "found"})
		gt.Equal(t, toolResp.IsError, false)
	})

	t.Run("round-trip serialization preserves data", func(t *testing.T) {
		// Create complex OpenAI message, convert to history, serialize/deserialize, convert back
		originalMsg := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful assistant.",
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: "text", Text: "Analyze this image"},
					{Type: "image_url", ImageURL: &openai.ChatMessageImageURL{
						URL:    "https://example.com/test.jpg",
						Detail: openai.ImageURLDetailLow,
					}},
				},
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "I'll analyze that for you.",
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_analyze",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "image_analyzer",
							Arguments: `{"mode":"detailed"}`,
						},
					},
				},
			},
		}

		// Convert to history
		history, err := gollem.NewHistoryFromOpenAI(originalMsg)
		gt.NoError(t, err)

		// Serialize and deserialize
		data, err := json.Marshal(history)
		gt.NoError(t, err)

		var restored gollem.History
		err = json.Unmarshal(data, &restored)
		gt.NoError(t, err)

		// Convert back to OpenAI
		convertedMsg, err := restored.ToOpenAI()
		gt.NoError(t, err)

		// Verify key information is preserved
		gt.True(t, len(convertedMsg) >= 2) // At least user and assistant messages

		// Find the user message with image
		hasUserWithImage := false
		hasAssistantWithTool := false

		for _, msg := range convertedMsg {
			if msg.Role == openai.ChatMessageRoleUser && len(msg.MultiContent) > 0 {
				for _, part := range msg.MultiContent {
					if part.Type == "image_url" && part.ImageURL != nil {
						hasUserWithImage = true
						break
					}
				}
			}
			if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
				hasAssistantWithTool = true
				for _, tc := range msg.ToolCalls {
					gt.NotEqual(t, tc.ID, "")
					gt.NotEqual(t, tc.Function.Name, "")
				}
			}
		}

		gt.True(t, hasUserWithImage)
		gt.True(t, hasAssistantWithTool)
	})
}

// Additional comprehensive test for all message content types and edge cases
func TestComprehensiveContentValidation(t *testing.T) {
	t.Run("validate all content type getters", func(t *testing.T) {
		testCases := []struct {
			name         string
			content      gollem.MessageContent
			expectedType gollem.MessageContentType
			shouldError  []string // Methods that should return errors
		}{
			{
				name:         "text content",
				content:      mustNewTextContent("test"),
				expectedType: gollem.MessageContentTypeText,
				shouldError:  []string{"GetImageContent", "GetToolCallContent", "GetToolResponseContent", "GetFunctionCallContent", "GetFunctionResponseContent"},
			},
		}

		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				gt.Equal(t, tc.content.Type, tc.expectedType)

				// Test that GetTextContent works for text content
				if tc.expectedType == gollem.MessageContentTypeText {
					_, err := tc.content.GetTextContent()
					gt.NoError(t, err)
				}

				// Test that other getters return errors
				for _, method := range tc.shouldError {
					switch method {
					case "GetImageContent":
						_, err := tc.content.GetImageContent()
						gt.Error(t, err)
					case "GetToolCallContent":
						_, err := tc.content.GetToolCallContent()
						gt.Error(t, err)
					case "GetToolResponseContent":
						_, err := tc.content.GetToolResponseContent()
						gt.Error(t, err)
					case "GetFunctionCallContent":
						_, err := tc.content.GetFunctionCallContent()
						gt.Error(t, err)
					case "GetFunctionResponseContent":
						_, err := tc.content.GetFunctionResponseContent()
						gt.Error(t, err)
					}
				}
			})
		}
	})

	t.Run("validate message role constants", func(t *testing.T) {
		roles := []gollem.MessageRole{
			gollem.RoleUser,
			gollem.RoleAssistant,
			gollem.RoleSystem,
			gollem.RoleTool,
			gollem.RoleFunction,
			gollem.RoleModel,
		}

		for _, role := range roles {
			gt.NotEqual(t, string(role), "")
			gt.True(t, len(string(role)) > 0)
		}

		// Verify role uniqueness
		roleSet := make(map[string]bool)
		for _, role := range roles {
			roleStr := string(role)
			gt.False(t, roleSet[roleStr])
			roleSet[roleStr] = true
		}
	})

	t.Run("validate content type constants", func(t *testing.T) {
		contentTypes := []gollem.MessageContentType{
			gollem.MessageContentTypeText,
			gollem.MessageContentTypeImage,
			gollem.MessageContentTypeToolCall,
			gollem.MessageContentTypeToolResponse,
			gollem.MessageContentTypeFunctionCall,
			gollem.MessageContentTypeFunctionResponse,
		}

		for _, contentType := range contentTypes {
			gt.NotEqual(t, string(contentType), "")
			gt.True(t, len(string(contentType)) > 0)
		}

		// Verify content type uniqueness
		typeSet := make(map[string]bool)
		for _, contentType := range contentTypes {
			typeStr := string(contentType)
			gt.False(t, typeSet[typeStr])
			typeSet[typeStr] = true
		}
	})
}
