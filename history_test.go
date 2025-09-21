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

// newTestToolResultBlock creates a test tool result block with the given ID and content
// This helper reduces the verbosity of creating anthropic.ToolResultBlock for test data.
func newTestToolResultBlock(id, content string) anthropic.ContentBlockParamUnion {
	return newTestToolResultBlockWithError(id, content, false)
}

// newTestToolResultBlockWithError creates a test tool result block with the given ID, content, and error status
// This helper reduces the verbosity of creating anthropic.ToolResultBlock for test data that may represent errors.
func newTestToolResultBlockWithError(id, content string, isError bool) anthropic.ContentBlockParamUnion {
	// Use the new 3-argument form of NewToolResultBlock
	return anthropic.NewToolResultBlock(id, content, isError)
}

func TestHistoryOpenAI(t *testing.T) {
	// Create OpenAI messages with various content types
	messages := []openai.ChatCompletionMessage{
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
		{
			Role:    "user",
			Content: "What's the weather like?",
		},
		{
			Role:    "assistant",
			Content: "",
			FunctionCall: &openai.FunctionCall{
				Name:      "get_weather",
				Arguments: `{"location": "Tokyo"}`,
			},
		},
		{
			Role:    "tool",
			Name:    "get_weather",
			Content: `{"temperature": 25, "condition": "sunny"}`,
		},
		{
			Role:    "assistant",
			Content: "The weather in Tokyo is sunny with a temperature of 25°C.",
		},
	}

	// Create History object
	history, err := gollem.NewHistoryFromOpenAI(messages)
	gt.NoError(t, err)

	// Convert to JSON
	data, err := json.Marshal(history)
	gt.NoError(t, err)

	// Restore from JSON
	var restored gollem.History
	gt.NoError(t, json.Unmarshal(data, &restored))

	restoredMessages, err := restored.ToOpenAI()
	gt.NoError(t, err)

	gt.Equal(t, messages, restoredMessages)

	// Validate specific message types
	gt.Equal(t, "system", restoredMessages[0].Role)
	gt.Equal(t, "You are a helpful assistant.", restoredMessages[0].Content)

	gt.Equal(t, "assistant", restoredMessages[2].Role)
	gt.Equal(t, "Hi, how can I help you?", restoredMessages[2].Content)

	gt.Equal(t, "assistant", restoredMessages[4].Role)
	gt.Equal(t, "", restoredMessages[4].Content)
	gt.Equal(t, "get_weather", restoredMessages[4].FunctionCall.Name)
	gt.Equal(t, `{"location": "Tokyo"}`, restoredMessages[4].FunctionCall.Arguments)

	gt.Equal(t, "tool", restoredMessages[5].Role)
	gt.Equal(t, "get_weather", restoredMessages[5].Name)
	gt.Equal(t, `{"temperature": 25, "condition": "sunny"}`, restoredMessages[5].Content)
}

func TestHistoryClaude(t *testing.T) {
	// Create Claude messages with various content types
	messages := []anthropic.MessageParam{
		{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock("Hello"),
				anthropic.NewImageBlockBase64("image/jpeg", "base64encodedimage"),
			},
		},
		{
			Role: anthropic.MessageParamRoleAssistant,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock("Hi, how can I help you?"),
			},
		},
		{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock("What's the weather like?"),
			},
		},
		{
			Role: anthropic.MessageParamRoleAssistant,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewToolUseBlock("tool_1", `{"location": "Tokyo"}`, "get_weather"),
			},
		},
		{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				newTestToolResultBlock("tool_2", `{"temperature": 30, "condition": "cloudy"}`),
			},
		},
		{
			Role: anthropic.MessageParamRoleAssistant,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock("Second message"),
			},
		},
		{
			Role: anthropic.MessageParamRoleUser,
			Content: []anthropic.ContentBlockParamUnion{
				newTestToolResultBlock("tool_3", `{"temperature": 35, "condition": "rainy"}`),
			},
		},
		{
			Role: anthropic.MessageParamRoleAssistant,
			Content: []anthropic.ContentBlockParamUnion{
				anthropic.NewTextBlock("The weather in Tokyo is sunny with a temperature of 25°C."),
			},
		},
	}

	// Create History object
	history, err := gollem.NewHistoryFromClaude(messages)
	gt.NoError(t, err)

	// Convert to JSON
	data, err := json.Marshal(history)
	gt.NoError(t, err)

	// Restore from JSON
	var restored gollem.History
	gt.NoError(t, json.Unmarshal(data, &restored))

	restoredMessages, err := restored.ToClaude()
	gt.NoError(t, err)

	// Compare each message individually to make debugging easier
	gt.Equal(t, len(messages), len(restoredMessages))
	for i := range messages {
		gt.Equal(t, messages[i].Role, restoredMessages[i].Role)
		gt.Equal(t, len(messages[i].Content), len(restoredMessages[i].Content))

		for j := range messages[i].Content {
			// Test specific field access based on type
			if messages[i].Content[j].OfToolResult != nil {
				gt.Value(t, restoredMessages[i].Content[j].OfToolResult.ToolUseID).Equal(messages[i].Content[j].OfToolResult.ToolUseID)
				gt.Value(t, restoredMessages[i].Content[j].OfToolResult.IsError).Equal(messages[i].Content[j].OfToolResult.IsError)
			}
		}
	}
}

func TestHistoryGemini(t *testing.T) {
	// Create Gemini messages with various content types
	messages := []*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: "Hello"},
				{
					InlineData: &genai.Blob{
						MIMEType: "image/jpeg",
						Data:     []byte("fake image data"),
					},
				},
				{
					FileData: &genai.FileData{
						MIMEType: "application/pdf",
						FileURI:  "gs://bucket/file.pdf",
					},
				},
			},
		},
		{
			Role: "model",
			Parts: []*genai.Part{
				{Text: "Hi, how can I help you?"},
				{
					FunctionCall: &genai.FunctionCall{
						Name: "test_function",
						Args: map[string]interface{}{
							"param1": "value1",
							"param2": float64(123),
						},
					},
				},
			},
		},
		{
			Role: "model",
			Parts: []*genai.Part{
				{Text: "Function result"},
				{
					FunctionResponse: &genai.FunctionResponse{
						Name: "test_function",
						Response: map[string]interface{}{
							"status": "success",
							"result": "operation completed",
						},
					},
				},
			},
		},
	}

	// Create History object
	history, err := gollem.NewHistoryFromGemini(messages)
	gt.NoError(t, err)

	// Convert to JSON
	data, err := json.Marshal(history)
	gt.NoError(t, err)

	// Restore from JSON
	var restored gollem.History
	gt.NoError(t, json.Unmarshal(data, &restored))

	restoredMessages, err := restored.ToGemini()
	gt.NoError(t, err)
	gt.Equal(t, messages, restoredMessages)

	// Validate specific message types
	gt.Equal(t, "user", restoredMessages[0].Role)
	gt.Equal(t, 3, len(restoredMessages[0].Parts))
	gt.Equal(t, "Hello", restoredMessages[0].Parts[0].Text)
	gt.Equal(t, "image/jpeg", restoredMessages[0].Parts[1].InlineData.MIMEType)
	gt.Equal(t, "application/pdf", restoredMessages[0].Parts[2].FileData.MIMEType)
	gt.Equal(t, "gs://bucket/file.pdf", restoredMessages[0].Parts[2].FileData.FileURI)

	gt.Equal(t, "model", restoredMessages[1].Role)
	gt.Equal(t, 2, len(restoredMessages[1].Parts))
	gt.Equal(t, "Hi, how can I help you?", restoredMessages[1].Parts[0].Text)
	gt.Equal(t, "test_function", restoredMessages[1].Parts[1].FunctionCall.Name)
	gt.Equal(t, "value1", restoredMessages[1].Parts[1].FunctionCall.Args["param1"])
	gt.Equal(t, float64(123), restoredMessages[1].Parts[1].FunctionCall.Args["param2"].(float64))

	gt.Equal(t, "model", restoredMessages[2].Role)
	gt.Equal(t, 2, len(restoredMessages[2].Parts))
	gt.Equal(t, "Function result", restoredMessages[2].Parts[0].Text)
	gt.Equal(t, "test_function", restoredMessages[2].Parts[1].FunctionResponse.Name)
	gt.Equal(t, "success", restoredMessages[2].Parts[1].FunctionResponse.Response["status"])
	gt.Equal(t, "operation completed", restoredMessages[2].Parts[1].FunctionResponse.Response["result"])
}

func TestHistoryClone(t *testing.T) {
	t.Run("nil history", func(t *testing.T) {
		var history *gollem.History
		cloned := history.Clone()
		gt.Nil(t, cloned)
	})

	t.Run("empty history", func(t *testing.T) {
		history := &gollem.History{
			LLType:  gollem.LLMTypeOpenAI,
			Version: 1,
		}
		cloned := history.Clone()
		gt.NotNil(t, cloned)
		gt.Equal(t, history.LLType, cloned.LLType)
		gt.Equal(t, history.Version, cloned.Version)
		gt.Equal(t, 0, len(cloned.Messages))
	})

	t.Run("openai history clone", func(t *testing.T) {
		messages := []openai.ChatCompletionMessage{
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
				Content: "",
				FunctionCall: &openai.FunctionCall{
					Name:      "get_weather",
					Arguments: `{"location": "Tokyo"}`,
				},
			},
			{
				Role:    "tool",
				Name:    "get_weather",
				Content: `{"temperature": 25, "condition": "sunny"}`,
			},
		}

		original, err := gollem.NewHistoryFromOpenAI(messages)
		gt.NoError(t, err)
		cloned := original.Clone()

		// Verify basic properties
		gt.NotNil(t, cloned)
		gt.Equal(t, original.LLType, cloned.LLType)
		gt.Equal(t, original.Version, cloned.Version)

		// Convert to OpenAI format to compare
		originalMessages, err := original.ToOpenAI()
		gt.NoError(t, err)
		clonedMessages, err := cloned.ToOpenAI()
		gt.NoError(t, err)

		gt.Equal(t, len(originalMessages), len(clonedMessages))

		// Verify content equality
		for i := range originalMessages {
			gt.Equal(t, originalMessages[i].Role, clonedMessages[i].Role)
			gt.Equal(t, originalMessages[i].Content, clonedMessages[i].Content)
			gt.Equal(t, originalMessages[i].Name, clonedMessages[i].Name)

			// Check function call equality
			if originalMessages[i].FunctionCall != nil {
				gt.NotNil(t, clonedMessages[i].FunctionCall)
				gt.Equal(t, originalMessages[i].FunctionCall.Name, clonedMessages[i].FunctionCall.Name)
				gt.Equal(t, originalMessages[i].FunctionCall.Arguments, clonedMessages[i].FunctionCall.Arguments)
			} else {
				gt.Nil(t, clonedMessages[i].FunctionCall)
			}
		}

		// Verify that the clone is truly independent by modifying it
		// and ensuring the original remains unchanged
		if original.Version == 2 && len(cloned.Messages) > 0 {
			// For V2 format, modify the Messages field
			originalMessageCount := len(original.Messages)
			cloned.Messages = append(cloned.Messages, gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{},
			})
			gt.Equal(t, originalMessageCount, len(original.Messages))
			gt.Equal(t, originalMessageCount+1, len(cloned.Messages))
		}
	})

	t.Run("claude history clone", func(t *testing.T) {
		messages := []anthropic.MessageParam{
			{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock("Hello"),
					anthropic.NewImageBlockBase64("image/jpeg", "base64encodedimage"),
				},
			},
			{
				Role: anthropic.MessageParamRoleAssistant,
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewToolUseBlock("tool_1", `{"location": "Tokyo"}`, "get_weather"),
				},
			},
			{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					newTestToolResultBlock("tool_1", `{"temperature": 25}`),
				},
			},
		}

		original, err := gollem.NewHistoryFromClaude(messages)
		gt.NoError(t, err)
		cloned := original.Clone()

		// Verify basic properties
		gt.NotNil(t, cloned)
		gt.Equal(t, original.LLType, cloned.LLType)
		gt.Equal(t, original.Version, cloned.Version)

		// Verify independence by converting back and checking
		originalConverted, err := original.ToClaude()
		gt.NoError(t, err)
		clonedConverted, err := cloned.ToClaude()
		gt.NoError(t, err)

		// Should be equal at this point
		gt.Equal(t, len(originalConverted), len(clonedConverted))
		for i := range originalConverted {
			gt.Equal(t, originalConverted[i].Role, clonedConverted[i].Role)
			gt.Equal(t, len(originalConverted[i].Content), len(clonedConverted[i].Content))
		}

		// Verify that the clone is truly independent by modifying it
		if original.Version == 2 && len(cloned.Messages) > 0 {
			// For V2 format, modify the Messages field
			originalMessageCount := len(original.Messages)
			cloned.Messages = append(cloned.Messages, gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{},
			})
			gt.Equal(t, originalMessageCount, len(original.Messages))
			gt.Equal(t, originalMessageCount+1, len(cloned.Messages))
		}
	})

	t.Run("gemini history clone", func(t *testing.T) {
		messages := []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Hello"},
					{
						InlineData: &genai.Blob{
							MIMEType: "image/jpeg",
							Data:     []byte("fake image data"),
						},
					},
				},
			},
			{
				Role: "model",
				Parts: []*genai.Part{
					{
						FunctionCall: &genai.FunctionCall{
							Name: "test_function",
							Args: map[string]interface{}{
								"param1": "value1",
								"param2": float64(123),
							},
						},
					},
					{
						FunctionResponse: &genai.FunctionResponse{
							Name: "test_function",
							Response: map[string]interface{}{
								"status": "success",
								"data":   []interface{}{"item1", "item2"},
							},
						},
					},
				},
			},
		}

		original, err := gollem.NewHistoryFromGemini(messages)
		gt.NoError(t, err)
		cloned := original.Clone()

		// Verify basic properties
		gt.NotNil(t, cloned)
		gt.Equal(t, original.LLType, cloned.LLType)
		gt.Equal(t, original.Version, cloned.Version)

		// Verify independence by converting back
		originalConverted, err := original.ToGemini()
		gt.NoError(t, err)
		clonedConverted, err := cloned.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, len(originalConverted), len(clonedConverted))

		// Verify content equality
		for i := range originalConverted {
			gt.Equal(t, originalConverted[i].Role, clonedConverted[i].Role)
			gt.Equal(t, len(originalConverted[i].Parts), len(clonedConverted[i].Parts))
		}

		// Verify that the clone is truly independent by modifying it
		if original.Version == 2 && len(cloned.Messages) > 0 {
			// For V2 format, modify the Messages field
			originalMessageCount := len(original.Messages)
			cloned.Messages = append(cloned.Messages, gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{},
			})
			gt.Equal(t, originalMessageCount, len(original.Messages))
			gt.Equal(t, originalMessageCount+1, len(cloned.Messages))
		}
	})

	t.Run("clone preserves all LLM types", func(t *testing.T) {
		// Test with a real Claude history created from messages
		messages := []anthropic.MessageParam{
			{
				Role: anthropic.MessageParamRoleUser,
				Content: []anthropic.ContentBlockParamUnion{
					anthropic.NewTextBlock("test message"),
				},
			},
		}

		history, err := gollem.NewHistoryFromClaude(messages)
		gt.NoError(t, err)
		cloned := history.Clone()

		gt.NotNil(t, cloned)
		gt.Equal(t, history.LLType, cloned.LLType)
		gt.Equal(t, history.Version, cloned.Version)
		gt.Equal(t, len(history.Messages), len(cloned.Messages))

		// Verify independence by converting back to messages
		originalConverted, err := history.ToClaude()
		gt.NoError(t, err)
		clonedConverted, err := cloned.ToClaude()
		gt.NoError(t, err)

		// Should be equal initially
		gt.Equal(t, len(originalConverted), len(clonedConverted))

		// Verify independence by modifying the clone
		if cloned.Version == 2 && len(cloned.Messages) > 0 {
			originalMessageCount := len(history.Messages)
			cloned.Messages = append(cloned.Messages, gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{},
			})
			gt.Equal(t, originalMessageCount, len(history.Messages))
			gt.Equal(t, originalMessageCount+1, len(cloned.Messages))
		}
	})
}
