package gollem_test

import (
	"encoding/base64"
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
	t.Run("text message conversion", func(t *testing.T) {
		input := []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello, Claude!")),
		}

		history, err := gollem.NewHistoryFromClaude(input)
		gt.NoError(t, err)

		// Verify history metadata
		gt.Equal(t, history.LLType, gollem.LLMTypeClaude)
		gt.Equal(t, history.Version, gollem.HistoryVersion)
		gt.Equal(t, len(history.Messages), 1)

		// Verify message details
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 1)

		// Verify content details
		content := msg.Contents[0]
		gt.Equal(t, content.Type, gollem.MessageContentTypeText)
		textContent, err := content.GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Hello, Claude!")

		// Test round-trip conversion
		claudeMsgs, err := history.ToClaude()
		gt.NoError(t, err)
		gt.Equal(t, len(claudeMsgs), 1)
		gt.Equal(t, claudeMsgs[0].Role, anthropic.MessageParamRoleUser)
		gt.Equal(t, len(claudeMsgs[0].Content), 1)
		gt.Equal(t, claudeMsgs[0].Content[0].OfText.Text, "Hello, Claude!")
	})

	t.Run("tool use conversion", func(t *testing.T) {
		args := map[string]any{
			"location": "Tokyo",
			"units":    "celsius",
		}
		input := []anthropic.MessageParam{
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock("tool_123", args, "get_weather"),
			),
		}

		history, err := gollem.NewHistoryFromClaude(input)
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleAssistant)
		gt.Equal(t, len(msg.Contents), 1)

		// Verify tool call content
		content := msg.Contents[0]
		gt.Equal(t, content.Type, gollem.MessageContentTypeToolCall)
		toolCall, err := content.GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, toolCall.ID, "tool_123")
		gt.Equal(t, toolCall.Name, "get_weather")
		gt.Equal(t, toolCall.Arguments["location"], "Tokyo")
		gt.Equal(t, toolCall.Arguments["units"], "celsius")

		// Round-trip verification
		claudeMsgs, err := history.ToClaude()
		gt.NoError(t, err)
		gt.Equal(t, len(claudeMsgs), 1)
		gt.Equal(t, claudeMsgs[0].Content[0].OfToolUse.ID, "tool_123")
		gt.Equal(t, claudeMsgs[0].Content[0].OfToolUse.Name, "get_weather")
	})

	t.Run("tool result conversion", func(t *testing.T) {
		input := []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("tool_456", `{"temperature": 25, "condition": "sunny"}`, false),
			),
		}

		history, err := gollem.NewHistoryFromClaude(input)
		gt.NoError(t, err)

		// Verify message structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 1)

		// Verify tool response content
		content := msg.Contents[0]
		gt.Equal(t, content.Type, gollem.MessageContentTypeToolResponse)
		toolResp, err := content.GetToolResponseContent()
		gt.NoError(t, err)
		gt.Equal(t, toolResp.ToolCallID, "tool_456")
		gt.Equal(t, toolResp.Name, "") // Claude doesn't include tool name in response
		gt.Equal(t, toolResp.Response["content"], `{"temperature": 25, "condition": "sunny"}`)
		gt.Equal(t, toolResp.IsError, false)
	})

	t.Run("image message conversion", func(t *testing.T) {
		// Create valid Base64 data
		imageData := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

		input := []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("Look at this image:"),
				anthropic.NewImageBlockBase64("image/png", imageData),
			),
		}

		history, err := gollem.NewHistoryFromClaude(input)
		gt.NoError(t, err)

		// Verify message structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 2)

		// Verify text content
		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Look at this image:")

		// Verify image content (should be decoded)
		imgContent, err := msg.Contents[1].GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, imgContent.MediaType, "image/png")

		// Decode the original Base64 to compare
		expectedBytes, _ := base64.StdEncoding.DecodeString(imageData)
		gt.Equal(t, imgContent.Data, expectedBytes)

		// Round-trip should re-encode to Base64
		claudeMsgs, err := history.ToClaude()
		gt.NoError(t, err)
		gt.Equal(t, len(claudeMsgs[0].Content), 2)
		gt.Equal(t, claudeMsgs[0].Content[1].OfImage.Source.OfBase64.Data, imageData)
	})

	t.Run("mixed content message", func(t *testing.T) {
		input := []anthropic.MessageParam{
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("I'll search for that."),
				anthropic.NewToolUseBlock("call_001", map[string]any{"query": "weather tokyo"}, "web_search"),
				anthropic.NewTextBlock("And calculate this."),
				anthropic.NewToolUseBlock("call_002", map[string]any{"expr": "2+2"}, "calculator"),
			),
		}

		history, err := gollem.NewHistoryFromClaude(input)
		gt.NoError(t, err)

		// Verify message structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleAssistant)
		gt.Equal(t, len(msg.Contents), 4)

		// Verify each content in order
		text1, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text1.Text, "I'll search for that.")

		tool1, err := msg.Contents[1].GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, tool1.ID, "call_001")
		gt.Equal(t, tool1.Name, "web_search")
		gt.Equal(t, tool1.Arguments["query"], "weather tokyo")

		text2, err := msg.Contents[2].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text2.Text, "And calculate this.")

		tool2, err := msg.Contents[3].GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, tool2.ID, "call_002")
		gt.Equal(t, tool2.Name, "calculator")
		gt.Equal(t, tool2.Arguments["expr"], "2+2")
	})
}

func TestHistoryFromOpenAI(t *testing.T) {
	t.Run("simple text message", func(t *testing.T) {
		input := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello, OpenAI!",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(input)
		gt.NoError(t, err)

		// Verify history metadata
		gt.Equal(t, history.LLType, gollem.LLMTypeOpenAI)
		gt.Equal(t, history.Version, gollem.HistoryVersion)
		gt.Equal(t, len(history.Messages), 1)

		// Verify message details
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 1)

		// Verify content
		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Hello, OpenAI!")

		// Round-trip verification
		openAIMsgs, err := history.ToOpenAI()
		gt.NoError(t, err)
		gt.Equal(t, len(openAIMsgs), 1)
		gt.Equal(t, openAIMsgs[0].Role, openai.ChatMessageRoleUser)
		gt.Equal(t, openAIMsgs[0].Content, "Hello, OpenAI!")
	})

	t.Run("system message", func(t *testing.T) {
		input := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are a helpful assistant.",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(input)
		gt.NoError(t, err)

		// Verify system role is preserved
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleSystem)

		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "You are a helpful assistant.")
	})

	t.Run("assistant with tool calls", func(t *testing.T) {
		input := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "Let me help you with that.",
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_789",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "get_time",
							Arguments: `{"timezone":"UTC"}`,
						},
					},
					{
						ID:   "call_790",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "get_weather",
							Arguments: `{"city":"London"}`,
						},
					},
				},
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(input)
		gt.NoError(t, err)

		// Verify message structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleAssistant)
		gt.Equal(t, len(msg.Contents), 3) // 1 text + 2 tool calls

		// Verify text content
		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Let me help you with that.")

		// Verify first tool call
		tool1, err := msg.Contents[1].GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, tool1.ID, "call_789")
		gt.Equal(t, tool1.Name, "get_time")
		gt.Equal(t, tool1.Arguments["timezone"], "UTC")

		// Verify second tool call
		tool2, err := msg.Contents[2].GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, tool2.ID, "call_790")
		gt.Equal(t, tool2.Name, "get_weather")
		gt.Equal(t, tool2.Arguments["city"], "London")

		// Round-trip verification
		openAIMsgs, err := history.ToOpenAI()
		gt.NoError(t, err)
		gt.Equal(t, len(openAIMsgs), 1)
		gt.Equal(t, openAIMsgs[0].Content, "Let me help you with that.")
		gt.Equal(t, len(openAIMsgs[0].ToolCalls), 2)
		gt.Equal(t, openAIMsgs[0].ToolCalls[0].ID, "call_789")
		gt.Equal(t, openAIMsgs[0].ToolCalls[1].ID, "call_790")
	})

	t.Run("tool response message", func(t *testing.T) {
		input := []openai.ChatCompletionMessage{
			{
				Role:       openai.ChatMessageRoleTool,
				Content:    `{"time":"2024-01-01T12:00:00Z","timezone":"UTC"}`,
				ToolCallID: "call_789",
				Name:       "get_time",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(input)
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleTool)

		// Tool responses with ToolCallID create both text and tool response content
		gt.Equal(t, len(msg.Contents), 2)

		// First should be text content
		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, `{"time":"2024-01-01T12:00:00Z","timezone":"UTC"}`)

		// Second should be tool response content
		toolResp, err := msg.Contents[1].GetToolResponseContent()
		gt.NoError(t, err)
		gt.Equal(t, toolResp.ToolCallID, "call_789")
		gt.Equal(t, toolResp.Name, "get_time")

		// Round-trip verification - might create multiple messages
		openAIMsgs, err := history.ToOpenAI()
		gt.NoError(t, err)
		gt.True(t, len(openAIMsgs) >= 1)

		// Find the tool messages (may be split into text and tool response)
		foundToolContent := false
		foundToolResponse := false
		for _, msg := range openAIMsgs {
			if msg.Role == openai.ChatMessageRoleTool {
				if msg.ToolCallID == "call_789" {
					foundToolResponse = true
					// Tool response should have the response data
					var respData map[string]interface{}
					if err := json.Unmarshal([]byte(msg.Content), &respData); err == nil {
						// May have content key or the direct data
						gt.True(t, len(respData) > 0)
					}
				} else if msg.Content == `{"time":"2024-01-01T12:00:00Z","timezone":"UTC"}` {
					foundToolContent = true
				}
			}
		}
		// Should have at least one tool message
		gt.True(t, foundToolContent || foundToolResponse)
	})

	t.Run("multi-content message with text and image", func(t *testing.T) {
		validBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="

		input := []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: "text",
						Text: "What's in this image?",
					},
					{
						Type: "text",
						Text: "Please describe in detail.",
					},
					{
						Type: "image_url",
						ImageURL: &openai.ChatMessageImageURL{
							URL:    "data:image/png;base64," + validBase64,
							Detail: openai.ImageURLDetailHigh,
						},
					},
				},
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(input)
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 3)

		// Verify first text
		text1, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text1.Text, "What's in this image?")

		// Verify second text
		text2, err := msg.Contents[1].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text2.Text, "Please describe in detail.")

		// Verify image (should be decoded from Base64)
		imgContent, err := msg.Contents[2].GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, imgContent.MediaType, "image/png")
		gt.Equal(t, imgContent.Detail, "high")

		// Data should be decoded
		expectedData, _ := base64.StdEncoding.DecodeString(validBase64)
		gt.Equal(t, imgContent.Data, expectedData)

		// Round-trip should re-encode
		openAIMsgs, err := history.ToOpenAI()
		gt.NoError(t, err)
		gt.Equal(t, len(openAIMsgs[0].MultiContent), 3)
		gt.Equal(t, openAIMsgs[0].MultiContent[2].ImageURL.URL, "data:image/png;base64,"+validBase64)
	})

	t.Run("function message (legacy format)", func(t *testing.T) {
		input := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleFunction,
				Content: "Function result data",
				Name:    "my_function",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(input)
		gt.NoError(t, err)

		// Verify function role is preserved
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleFunction)

		// Content should be text
		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Function result data")
	})
}

func TestHistoryFromGemini(t *testing.T) {
	t.Run("user text message", func(t *testing.T) {
		input := []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Hello, Gemini!"},
				},
			},
		}

		history, err := gollem.NewHistoryFromGemini(input)
		gt.NoError(t, err)

		// Verify history metadata
		gt.Equal(t, history.LLType, gollem.LLMTypeGemini)
		gt.Equal(t, history.Version, gollem.HistoryVersion)
		gt.Equal(t, len(history.Messages), 1)

		// Verify message
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 1)

		// Verify content
		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Hello, Gemini!")

		// Round-trip verification
		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, len(geminiMsgs), 1)
		gt.Equal(t, geminiMsgs[0].Role, "user")
		gt.Equal(t, geminiMsgs[0].Parts[0].Text, "Hello, Gemini!")
	})

	t.Run("model message", func(t *testing.T) {
		input := []*genai.Content{
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Hello from model!"},
				},
			},
		}

		history, err := gollem.NewHistoryFromGemini(input)
		gt.NoError(t, err)

		// Model role should be preserved
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleModel)

		textContent, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, textContent.Text, "Hello from model!")

		// Round-trip should preserve model role
		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, geminiMsgs[0].Role, "model")
	})

	t.Run("function call", func(t *testing.T) {
		input := []*genai.Content{
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "I'll calculate that for you."},
					{FunctionCall: &genai.FunctionCall{
						Name: "calculate",
						Args: map[string]any{
							"expression": "2+2",
							"precision":  float64(2),
						},
					}},
				},
			},
		}

		history, err := gollem.NewHistoryFromGemini(input)
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleModel)
		gt.Equal(t, len(msg.Contents), 2)

		// Verify text
		text, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text.Text, "I'll calculate that for you.")

		// Verify tool call (Gemini function calls become tool calls)
		toolCall, err := msg.Contents[1].GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, toolCall.Name, "calculate")

		// Verify arguments (already as map for tool calls)
		gt.Equal(t, toolCall.Arguments["expression"], "2+2")
		gt.Equal(t, toolCall.Arguments["precision"], 2.0)

		// Round-trip verification
		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, len(geminiMsgs[0].Parts), 2)
		gt.Equal(t, geminiMsgs[0].Parts[1].FunctionCall.Name, "calculate")
		gt.Equal(t, geminiMsgs[0].Parts[1].FunctionCall.Args["expression"], "2+2")
	})

	t.Run("function response", func(t *testing.T) {
		input := []*genai.Content{
			{
				Role: "function",
				Parts: []*genai.Part{
					{FunctionResponse: &genai.FunctionResponse{
						Name: "calculate",
						Response: map[string]any{
							"result": float64(4),
							"steps":  []any{"2+2", "=4"},
						},
					}},
				},
			},
		}

		history, err := gollem.NewHistoryFromGemini(input)
		gt.NoError(t, err)

		// Function role should be preserved
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleFunction)
		gt.Equal(t, len(msg.Contents), 1)

		// Verify tool response content (Gemini function responses become tool responses)
		toolResp, err := msg.Contents[0].GetToolResponseContent()
		gt.NoError(t, err)
		gt.Equal(t, toolResp.Name, "calculate")

		// Response is already a map
		gt.Equal(t, toolResp.Response["result"], 4.0)
		steps := toolResp.Response["steps"].([]interface{})
		gt.Equal(t, steps[0], "2+2")
		gt.Equal(t, steps[1], "=4")

		// Round-trip verification
		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, geminiMsgs[0].Role, "function")
		gt.Equal(t, geminiMsgs[0].Parts[0].FunctionResponse.Name, "calculate")
	})

	t.Run("inline data (image)", func(t *testing.T) {
		imageBytes := []byte("test image data")

		input := []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Look at this:"},
					{InlineData: &genai.Blob{
						MIMEType: "image/jpeg",
						Data:     imageBytes,
					}},
				},
			},
		}

		history, err := gollem.NewHistoryFromGemini(input)
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 2)

		// Verify text
		text, err := msg.Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text.Text, "Look at this:")

		// Verify image
		img, err := msg.Contents[1].GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, img.MediaType, "image/jpeg")
		gt.Equal(t, img.Data, imageBytes)
		gt.Equal(t, img.URL, "")

		// Round-trip verification
		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, len(geminiMsgs[0].Parts), 2)
		gt.Equal(t, geminiMsgs[0].Parts[1].InlineData.MIMEType, "image/jpeg")
		gt.Equal(t, geminiMsgs[0].Parts[1].InlineData.Data, imageBytes)
	})

	t.Run("file data (video)", func(t *testing.T) {
		input := []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{FileData: &genai.FileData{
						MIMEType: "video/mp4",
						FileURI:  "gs://bucket/video.mp4",
					}},
				},
			},
		}

		history, err := gollem.NewHistoryFromGemini(input)
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(history.Messages), 1)
		msg := history.Messages[0]
		gt.Equal(t, msg.Role, gollem.RoleUser)
		gt.Equal(t, len(msg.Contents), 1)

		// File data becomes image content with URL
		img, err := msg.Contents[0].GetImageContent()
		gt.NoError(t, err)
		gt.Equal(t, img.MediaType, "video/mp4")
		gt.Equal(t, img.URL, "gs://bucket/video.mp4")
		gt.Equal(t, len(img.Data), 0)

		// Round-trip verification
		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, geminiMsgs[0].Parts[0].FileData.MIMEType, "video/mp4")
		gt.Equal(t, geminiMsgs[0].Parts[0].FileData.FileURI, "gs://bucket/video.mp4")
	})
}

func TestCrossProviderConversion(t *testing.T) {
	t.Run("OpenAI to Claude conversion", func(t *testing.T) {
		openAIMsgs := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "You are helpful.",
			},
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "Hello",
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "Hi there!",
				ToolCalls: []openai.ToolCall{
					{
						ID:   "call_123",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "search",
							Arguments: `{"q":"weather"}`,
						},
					},
				},
			},
			{
				Role:       openai.ChatMessageRoleTool,
				Content:    `{"result":"sunny"}`,
				ToolCallID: "call_123",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(openAIMsgs)
		gt.NoError(t, err)

		// Convert to Claude
		claudeMsgs, err := history.ToClaude()
		gt.NoError(t, err)

		// System message should be merged into first user message
		gt.Equal(t, len(claudeMsgs), 3) // user, assistant, user (tool result)

		// First message should be user with system content merged
		gt.Equal(t, claudeMsgs[0].Role, anthropic.MessageParamRoleUser)
		gt.Equal(t, len(claudeMsgs[0].Content), 2)
		gt.Equal(t, claudeMsgs[0].Content[0].OfText.Text, "You are helpful.\n\n")
		gt.Equal(t, claudeMsgs[0].Content[1].OfText.Text, "Hello")

		// Second message should be assistant with text and tool use
		gt.Equal(t, claudeMsgs[1].Role, anthropic.MessageParamRoleAssistant)
		gt.Equal(t, len(claudeMsgs[1].Content), 2)
		gt.Equal(t, claudeMsgs[1].Content[0].OfText.Text, "Hi there!")
		gt.Equal(t, claudeMsgs[1].Content[1].OfToolUse.ID, "call_123")
		gt.Equal(t, claudeMsgs[1].Content[1].OfToolUse.Name, "search")

		// Third message should be user with tool result
		gt.Equal(t, claudeMsgs[2].Role, anthropic.MessageParamRoleUser)
		// Tool responses may have multiple content blocks
		gt.True(t, len(claudeMsgs[2].Content) >= 1)
		// Find the tool result block
		foundToolResult := false
		for _, block := range claudeMsgs[2].Content {
			if block.OfToolResult != nil && block.OfToolResult.ToolUseID == "call_123" {
				foundToolResult = true
				break
			}
		}
		gt.True(t, foundToolResult)
	})

	t.Run("Claude to Gemini conversion", func(t *testing.T) {
		claudeMsgs := []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("Question?"),
			),
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("Let me search."),
				anthropic.NewToolUseBlock("tool_1", map[string]any{"q": "answer"}, "search"),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("tool_1", `{"found": true}`, false),
			),
		}

		history, err := gollem.NewHistoryFromClaude(claudeMsgs)
		gt.NoError(t, err)

		// Convert to Gemini
		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)

		gt.Equal(t, len(geminiMsgs), 3)

		// First message - user
		gt.Equal(t, geminiMsgs[0].Role, "user")
		gt.Equal(t, geminiMsgs[0].Parts[0].Text, "Question?")

		// Second message - model with text and function call
		gt.Equal(t, geminiMsgs[1].Role, "model")
		gt.Equal(t, len(geminiMsgs[1].Parts), 2)
		gt.Equal(t, geminiMsgs[1].Parts[0].Text, "Let me search.")
		gt.Equal(t, geminiMsgs[1].Parts[1].FunctionCall.Name, "search")
		gt.Equal(t, geminiMsgs[1].Parts[1].FunctionCall.Args["q"], "answer")

		// Third message - user with function response (tool results go to user in Gemini)
		gt.Equal(t, geminiMsgs[2].Role, "user")
		gt.Equal(t, len(geminiMsgs[2].Parts), 1)
		gt.NotNil(t, geminiMsgs[2].Parts[0].FunctionResponse)
	})

	t.Run("Gemini to OpenAI conversion", func(t *testing.T) {
		geminiMsgs := []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Question"},
				},
			},
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Answer"},
					{FunctionCall: &genai.FunctionCall{
						Name: "tool",
						Args: map[string]any{"x": float64(1)},
					}},
				},
			},
			{
				Role: "function",
				Parts: []*genai.Part{
					{FunctionResponse: &genai.FunctionResponse{
						Name:     "tool",
						Response: map[string]any{"y": float64(2)},
					}},
				},
			},
		}

		history, err := gollem.NewHistoryFromGemini(geminiMsgs)
		gt.NoError(t, err)

		// Convert to OpenAI
		openAIMsgs, err := history.ToOpenAI()
		gt.NoError(t, err)

		gt.Equal(t, len(openAIMsgs), 3)

		// First message - user
		gt.Equal(t, openAIMsgs[0].Role, openai.ChatMessageRoleUser)
		gt.Equal(t, openAIMsgs[0].Content, "Question")

		// Second message - assistant with text and tool call
		gt.Equal(t, openAIMsgs[1].Role, openai.ChatMessageRoleAssistant)
		gt.Equal(t, openAIMsgs[1].Content, "Answer")
		gt.Equal(t, len(openAIMsgs[1].ToolCalls), 1)
		gt.Equal(t, openAIMsgs[1].ToolCalls[0].Function.Name, "tool")
		gt.Equal(t, openAIMsgs[1].ToolCalls[0].Function.Arguments, `{"x":1}`)

		// Third message - function or tool
		gt.True(t, openAIMsgs[2].Role == openai.ChatMessageRoleFunction || openAIMsgs[2].Role == openai.ChatMessageRoleTool)
		gt.Equal(t, openAIMsgs[2].Content, `{"y":2}`)
		if openAIMsgs[2].Role == openai.ChatMessageRoleFunction {
			gt.Equal(t, openAIMsgs[2].Name, "tool")
		}
	})
}

func TestRoundTripConversions(t *testing.T) {
	t.Run("OpenAI round-trip with all content types", func(t *testing.T) {
		validBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
		original := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleSystem,
				Content: "System prompt",
			},
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{Type: "text", Text: "User text"},
					{Type: "image_url", ImageURL: &openai.ChatMessageImageURL{
						URL:    "data:image/png;base64," + validBase64,
						Detail: openai.ImageURLDetailLow,
					}},
				},
			},
			{
				Role:    openai.ChatMessageRoleAssistant,
				Content: "Assistant response",
				ToolCalls: []openai.ToolCall{
					{
						ID:   "tc_1",
						Type: "function",
						Function: openai.FunctionCall{
							Name:      "func1",
							Arguments: `{"arg":"val"}`,
						},
					},
				},
			},
			{
				Role:       openai.ChatMessageRoleTool,
				Content:    `{"result":"ok"}`,
				ToolCallID: "tc_1",
				Name:       "func1",
			},
		}

		// Convert to history
		history, err := gollem.NewHistoryFromOpenAI(original)
		gt.NoError(t, err)

		// Convert back to OpenAI
		converted, err := history.ToOpenAI()
		gt.NoError(t, err)

		// Verify essential structure is preserved
		gt.True(t, len(converted) >= 3) // At least user, assistant, tool

		// Find and verify user message with image
		foundUserWithImage := false
		for _, msg := range converted {
			if msg.Role == openai.ChatMessageRoleUser && len(msg.MultiContent) > 0 {
				for _, part := range msg.MultiContent {
					if part.Type == "image_url" && part.ImageURL != nil {
						foundUserWithImage = true
						gt.Equal(t, part.ImageURL.URL, "data:image/png;base64,"+validBase64)
						gt.Equal(t, part.ImageURL.Detail, openai.ImageURLDetailLow)
					}
				}
			}
		}
		gt.True(t, foundUserWithImage)

		// Find and verify assistant with tool call
		foundAssistantWithTool := false
		for _, msg := range converted {
			if msg.Role == openai.ChatMessageRoleAssistant && len(msg.ToolCalls) > 0 {
				foundAssistantWithTool = true
				gt.Equal(t, msg.ToolCalls[0].ID, "tc_1")
				gt.Equal(t, msg.ToolCalls[0].Function.Name, "func1")
				gt.Equal(t, msg.ToolCalls[0].Function.Arguments, `{"arg":"val"}`)
			}
		}
		gt.True(t, foundAssistantWithTool)

		// Find and verify tool response
		foundToolResponse := false
		for _, msg := range converted {
			if msg.Role == openai.ChatMessageRoleTool {
				foundToolResponse = true
				// Tool response may have been transformed
				if msg.ToolCallID == "tc_1" {
					// Check that content contains the result
					var content map[string]interface{}
					if err := json.Unmarshal([]byte(msg.Content), &content); err == nil {
						// Content may be wrapped or direct
						gt.True(t, content["result"] == "ok" || content["content"] == `{"result":"ok"}`)
					}
				} else {
					// Or it might be the original content
					gt.Equal(t, msg.Content, `{"result":"ok"}`)
				}
			}
		}
		gt.True(t, foundToolResponse)
	})

	t.Run("Claude round-trip preserves content", func(t *testing.T) {
		imageBase64 := "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg=="
		original := []anthropic.MessageParam{
			anthropic.NewUserMessage(
				anthropic.NewTextBlock("User message"),
				anthropic.NewImageBlockBase64("image/png", imageBase64),
			),
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("Response"),
				anthropic.NewToolUseBlock("use_1", map[string]any{"key": "value"}, "tool_name"),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock("use_1", `{"status":"complete"}`, false),
			),
		}

		// Convert to history
		history, err := gollem.NewHistoryFromClaude(original)
		gt.NoError(t, err)

		// Convert back to Claude
		converted, err := history.ToClaude()
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(converted), 3)

		// First message - user with text and image
		gt.Equal(t, converted[0].Role, anthropic.MessageParamRoleUser)
		gt.Equal(t, len(converted[0].Content), 2)
		gt.Equal(t, converted[0].Content[0].OfText.Text, "User message")
		gt.Equal(t, string(converted[0].Content[1].OfImage.Source.OfBase64.MediaType), "image/png")
		gt.Equal(t, converted[0].Content[1].OfImage.Source.OfBase64.Data, imageBase64)

		// Second message - assistant with text and tool use
		gt.Equal(t, converted[1].Role, anthropic.MessageParamRoleAssistant)
		gt.Equal(t, len(converted[1].Content), 2)
		gt.Equal(t, converted[1].Content[0].OfText.Text, "Response")
		gt.Equal(t, converted[1].Content[1].OfToolUse.ID, "use_1")
		gt.Equal(t, converted[1].Content[1].OfToolUse.Name, "tool_name")

		// Third message - user with tool result
		gt.Equal(t, converted[2].Role, anthropic.MessageParamRoleUser)
		gt.Equal(t, len(converted[2].Content), 1)
		gt.Equal(t, converted[2].Content[0].OfToolResult.ToolUseID, "use_1")
	})

	t.Run("Gemini round-trip with all part types", func(t *testing.T) {
		imageData := []byte("image bytes")
		original := []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "User text"},
					{InlineData: &genai.Blob{
						MIMEType: "image/jpeg",
						Data:     imageData,
					}},
				},
			},
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Model response"},
					{FunctionCall: &genai.FunctionCall{
						Name: "my_func",
						Args: map[string]any{"param": "value"},
					}},
				},
			},
			{
				Role: "function",
				Parts: []*genai.Part{
					{FunctionResponse: &genai.FunctionResponse{
						Name:     "my_func",
						Response: map[string]any{"result": "success"},
					}},
				},
			},
		}

		// Convert to history
		history, err := gollem.NewHistoryFromGemini(original)
		gt.NoError(t, err)

		// Convert back to Gemini
		converted, err := history.ToGemini()
		gt.NoError(t, err)

		// Verify structure
		gt.Equal(t, len(converted), 3)

		// First message - user with text and image
		gt.Equal(t, converted[0].Role, "user")
		gt.Equal(t, len(converted[0].Parts), 2)
		gt.Equal(t, converted[0].Parts[0].Text, "User text")
		gt.Equal(t, converted[0].Parts[1].InlineData.MIMEType, "image/jpeg")
		gt.Equal(t, converted[0].Parts[1].InlineData.Data, imageData)

		// Second message - model with text and function call
		gt.Equal(t, converted[1].Role, "model")
		gt.Equal(t, len(converted[1].Parts), 2)
		gt.Equal(t, converted[1].Parts[0].Text, "Model response")
		gt.Equal(t, converted[1].Parts[1].FunctionCall.Name, "my_func")
		gt.Equal(t, converted[1].Parts[1].FunctionCall.Args["param"], "value")

		// Third message - function response
		gt.Equal(t, converted[2].Role, "function")
		gt.Equal(t, len(converted[2].Parts), 1)
		gt.Equal(t, converted[2].Parts[0].FunctionResponse.Name, "my_func")
		gt.Equal(t, converted[2].Parts[0].FunctionResponse.Response["result"], "success")
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

		// Tool call should still be created
		content := history.Messages[0].Contents[0]
		gt.Equal(t, content.Type, gollem.MessageContentTypeToolCall)
		toolCall, err := content.GetToolCallContent()
		gt.NoError(t, err)
		gt.Equal(t, toolCall.ID, "call_bad")
		gt.Equal(t, toolCall.Name, "bad_tool")
		// Arguments should be empty map or contain raw string
		gt.NotNil(t, toolCall.Arguments)
	})

	t.Run("empty content handling", func(t *testing.T) {
		msg := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleUser,
				Content: "",
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 1)

		// Empty content should create empty text
		if len(history.Messages[0].Contents) > 0 {
			content := history.Messages[0].Contents[0]
			gt.Equal(t, content.Type, gollem.MessageContentTypeText)
			text, err := content.GetTextContent()
			gt.NoError(t, err)
			gt.Equal(t, text.Text, "")
		}
	})

	t.Run("missing tool call ID", func(t *testing.T) {
		msg := []openai.ChatCompletionMessage{
			{
				Role:    openai.ChatMessageRoleTool,
				Content: `{"result":"success"}`,
				// Missing ToolCallID
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err)
		gt.Equal(t, len(history.Messages), 1)
		gt.Equal(t, history.Messages[0].Role, gollem.RoleTool)

		// Content should still be created
		gt.True(t, len(history.Messages[0].Contents) > 0)
		content := history.Messages[0].Contents[0]
		gt.Equal(t, content.Type, gollem.MessageContentTypeText)
		text, err := content.GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text.Text, `{"result":"success"}`)
	})

	t.Run("invalid image data URL", func(t *testing.T) {
		msg := []openai.ChatCompletionMessage{
			{
				Role: openai.ChatMessageRoleUser,
				MultiContent: []openai.ChatMessagePart{
					{
						Type: "image_url",
						ImageURL: &openai.ChatMessageImageURL{
							URL: "data:image/png;base64,invalid!!!",
						},
					},
				},
			},
		}

		history, err := gollem.NewHistoryFromOpenAI(msg)
		gt.NoError(t, err) // Should handle gracefully
		gt.Equal(t, len(history.Messages), 1)
		gt.Equal(t, len(history.Messages[0].Contents), 1)

		// Should create image content with URL preserved
		content := history.Messages[0].Contents[0]
		gt.Equal(t, content.Type, gollem.MessageContentTypeImage)
		img, err := content.GetImageContent()
		gt.NoError(t, err)
		// Invalid Base64 should preserve URL
		gt.Equal(t, img.URL, "data:image/png;base64,invalid!!!")
		gt.Equal(t, img.MediaType, "image/png")
	})

	t.Run("content type mismatch errors", func(t *testing.T) {
		textContent := mustNewTextContent("Hello")

		_, err := textContent.GetImageContent()
		gt.Error(t, err)

		_, err = textContent.GetToolCallContent()
		gt.Error(t, err)

		_, err = textContent.GetToolResponseContent()
		gt.Error(t, err)

		_, err = textContent.GetFunctionCallContent()
		gt.Error(t, err)

		_, err = textContent.GetFunctionResponseContent()
		gt.Error(t, err)

		// Verify correct getter works
		text, err := textContent.GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, text.Text, "Hello")
	})

	t.Run("Unicode and special characters", func(t *testing.T) {
		specialText := "Unicode: 🚀 ñáéíóú 中文 العربية\nNewlines\tTabs\"Quotes\""

		history := &gollem.History{
			Version: gollem.HistoryVersion,
			LLType:  gollem.LLMTypeOpenAI,
			Messages: []gollem.Message{
				{
					Role: gollem.RoleUser,
					Contents: []gollem.MessageContent{
						mustNewTextContent(specialText),
					},
				},
			},
		}

		// Test conversion to all providers handles special characters
		claudeMsgs, err := history.ToClaude()
		gt.NoError(t, err)
		gt.Equal(t, len(claudeMsgs), 1)
		gt.Equal(t, claudeMsgs[0].Content[0].OfText.Text, specialText)

		openAIMsgs, err := history.ToOpenAI()
		gt.NoError(t, err)
		gt.Equal(t, len(openAIMsgs), 1)
		gt.Equal(t, openAIMsgs[0].Content, specialText)

		geminiMsgs, err := history.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, len(geminiMsgs), 1)
		gt.Equal(t, geminiMsgs[0].Parts[0].Text, specialText)
	})
}

func TestHistorySerialization(t *testing.T) {
	t.Run("complete history serialization", func(t *testing.T) {
		// Create complex history
		textContent, _ := gollem.NewTextContent("Test message")
		imageContent, _ := gollem.NewImageContent("image/png", []byte{0x89, 0x50, 0x4E, 0x47}, "", "high")
		toolCallContent, _ := gollem.NewToolCallContent("call_123", "search", map[string]any{
			"query": "test query",
			"limit": float64(10),
		})
		toolRespContent, _ := gollem.NewToolResponseContent("call_123", "search", map[string]any{
			"results": []any{"result1", "result2"},
			"count":   float64(2),
		}, false)
		funcCallContent, _ := gollem.NewFunctionCallContent("calculator", `{"operation":"add","values":[1,2]}`)
		funcRespContent, _ := gollem.NewFunctionResponseContent("calculator", "3")

		history := &gollem.History{
			Version: gollem.HistoryVersion,
			LLType:  gollem.LLMTypeOpenAI,
			Messages: []gollem.Message{
				{
					Role:     gollem.RoleSystem,
					Contents: []gollem.MessageContent{textContent},
				},
				{
					Role:     gollem.RoleUser,
					Contents: []gollem.MessageContent{textContent, imageContent},
				},
				{
					Role:     gollem.RoleAssistant,
					Contents: []gollem.MessageContent{textContent, toolCallContent},
				},
				{
					Role:     gollem.RoleTool,
					Contents: []gollem.MessageContent{toolRespContent},
				},
				{
					Role:     gollem.RoleModel,
					Contents: []gollem.MessageContent{funcCallContent},
				},
				{
					Role:     gollem.RoleFunction,
					Contents: []gollem.MessageContent{funcRespContent},
				},
			},
		}

		// Serialize
		data, err := json.Marshal(history)
		gt.NoError(t, err)

		// Deserialize
		var restored gollem.History
		err = json.Unmarshal(data, &restored)
		gt.NoError(t, err)

		// Verify complete structure
		gt.Equal(t, restored.Version, gollem.HistoryVersion)
		gt.Equal(t, restored.LLType, gollem.LLMTypeOpenAI)
		gt.Equal(t, len(restored.Messages), 6)

		// Verify each message role
		gt.Equal(t, restored.Messages[0].Role, gollem.RoleSystem)
		gt.Equal(t, restored.Messages[1].Role, gollem.RoleUser)
		gt.Equal(t, restored.Messages[2].Role, gollem.RoleAssistant)
		gt.Equal(t, restored.Messages[3].Role, gollem.RoleTool)
		gt.Equal(t, restored.Messages[4].Role, gollem.RoleModel)
		gt.Equal(t, restored.Messages[5].Role, gollem.RoleFunction)

		// Verify content types are preserved
		gt.Equal(t, restored.Messages[1].Contents[0].Type, gollem.MessageContentTypeText)
		gt.Equal(t, restored.Messages[1].Contents[1].Type, gollem.MessageContentTypeImage)
		gt.Equal(t, restored.Messages[2].Contents[1].Type, gollem.MessageContentTypeToolCall)
		gt.Equal(t, restored.Messages[3].Contents[0].Type, gollem.MessageContentTypeToolResponse)
		gt.Equal(t, restored.Messages[4].Contents[0].Type, gollem.MessageContentTypeFunctionCall)
		gt.Equal(t, restored.Messages[5].Contents[0].Type, gollem.MessageContentTypeFunctionResponse)

		// Verify actual content data
		text, _ := restored.Messages[0].Contents[0].GetTextContent()
		gt.Equal(t, text.Text, "Test message")

		img, _ := restored.Messages[1].Contents[1].GetImageContent()
		gt.Equal(t, img.MediaType, "image/png")
		gt.Equal(t, img.Data, []byte{0x89, 0x50, 0x4E, 0x47})
		gt.Equal(t, img.Detail, "high")

		toolCall, _ := restored.Messages[2].Contents[1].GetToolCallContent()
		gt.Equal(t, toolCall.ID, "call_123")
		gt.Equal(t, toolCall.Name, "search")
		gt.Equal(t, toolCall.Arguments["query"], "test query")
		gt.Equal(t, toolCall.Arguments["limit"], 10.0)

		toolResp, _ := restored.Messages[3].Contents[0].GetToolResponseContent()
		gt.Equal(t, toolResp.ToolCallID, "call_123")
		gt.Equal(t, toolResp.Name, "search")
		results := toolResp.Response["results"].([]interface{})
		gt.Equal(t, results[0], "result1")
		gt.Equal(t, results[1], "result2")
		gt.Equal(t, toolResp.Response["count"], 2.0)

		funcCall, _ := restored.Messages[4].Contents[0].GetFunctionCallContent()
		gt.Equal(t, funcCall.Name, "calculator")
		gt.Equal(t, funcCall.Arguments, `{"operation":"add","values":[1,2]}`)

		funcResp, _ := restored.Messages[5].Contents[0].GetFunctionResponseContent()
		gt.Equal(t, funcResp.Name, "calculator")
		gt.Equal(t, funcResp.Content, "3")
	})
}
