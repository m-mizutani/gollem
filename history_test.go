package gollem_test

import (
	"encoding/json"
	"testing"

	"cloud.google.com/go/vertexai/genai"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/packages/param"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/sashabaranov/go-openai"
)

// newTestToolResultBlock creates a test tool result block with the given ID and content
// This helper reduces the verbosity of creating anthropic.ToolResultBlock for test data.
func newTestToolResultBlock(id, content string) anthropic.ContentBlockParamUnion {
	return newTestToolResultBlockWithError(id, content, false)
}

// newTestToolResultBlockWithError creates a test tool result block with the given ID, content, and error status
// This helper reduces the verbosity of creating anthropic.ToolResultBlock for test data that may represent errors.
func newTestToolResultBlockWithError(id, content string, isError bool) anthropic.ContentBlockParamUnion {
	toolResult := anthropic.NewToolResultBlock(id)
	if content != "" {
		toolResult.OfToolResult.Content = []anthropic.ToolResultBlockParamContentUnion{
			{OfText: &anthropic.TextBlockParam{Text: content}},
		}
	}
	if isError {
		toolResult.OfToolResult.IsError = param.NewOpt(true)
	}
	return toolResult
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
	history := gollem.NewHistoryFromOpenAI(messages)

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
	history := gollem.NewHistoryFromClaude(messages)

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
			Parts: []genai.Part{
				genai.Text("Hello"),
				genai.Blob{
					MIMEType: "image/jpeg",
					Data:     []byte("fake image data"),
				},
				genai.FileData{
					MIMEType: "application/pdf",
					FileURI:  "gs://bucket/file.pdf",
				},
			},
		},
		{
			Role: "model",
			Parts: []genai.Part{
				genai.Text("Hi, how can I help you?"),
				genai.FunctionCall{
					Name: "test_function",
					Args: map[string]interface{}{
						"param1": "value1",
						"param2": float64(123),
					},
				},
			},
		},
		{
			Role: "model",
			Parts: []genai.Part{
				genai.Text("Function result"),
				genai.FunctionResponse{
					Name: "test_function",
					Response: map[string]interface{}{
						"status": "success",
						"result": "operation completed",
					},
				},
			},
		},
	}

	// Create History object
	history := gollem.NewHistoryFromGemini(messages)

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
	gt.Equal(t, "Hello", restoredMessages[0].Parts[0].(genai.Text))
	gt.Equal(t, "image/jpeg", restoredMessages[0].Parts[1].(genai.Blob).MIMEType)
	gt.Equal(t, "application/pdf", restoredMessages[0].Parts[2].(genai.FileData).MIMEType)
	gt.Equal(t, "gs://bucket/file.pdf", restoredMessages[0].Parts[2].(genai.FileData).FileURI)

	gt.Equal(t, "model", restoredMessages[1].Role)
	gt.Equal(t, 2, len(restoredMessages[1].Parts))
	gt.Equal(t, "Hi, how can I help you?", restoredMessages[1].Parts[0].(genai.Text))
	gt.Equal(t, "test_function", restoredMessages[1].Parts[1].(genai.FunctionCall).Name)
	gt.Equal(t, "value1", restoredMessages[1].Parts[1].(genai.FunctionCall).Args["param1"])
	gt.Equal(t, float64(123), restoredMessages[1].Parts[1].(genai.FunctionCall).Args["param2"].(float64))

	gt.Equal(t, "model", restoredMessages[2].Role)
	gt.Equal(t, 2, len(restoredMessages[2].Parts))
	gt.Equal(t, "Function result", restoredMessages[2].Parts[0].(genai.Text))
	gt.Equal(t, "test_function", restoredMessages[2].Parts[1].(genai.FunctionResponse).Name)
	gt.Equal(t, "success", restoredMessages[2].Parts[1].(genai.FunctionResponse).Response["status"])
	gt.Equal(t, "operation completed", restoredMessages[2].Parts[1].(genai.FunctionResponse).Response["result"])
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
		gt.Equal(t, 0, len(cloned.OpenAI))
		gt.Equal(t, 0, len(cloned.Claude))
		gt.Equal(t, 0, len(cloned.Gemini))
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

		original := gollem.NewHistoryFromOpenAI(messages)
		cloned := original.Clone()

		// Verify basic properties
		gt.NotNil(t, cloned)
		gt.Equal(t, original.LLType, cloned.LLType)
		gt.Equal(t, original.Version, cloned.Version)
		gt.Equal(t, len(original.OpenAI), len(cloned.OpenAI))

		// Verify content equality
		for i := range original.OpenAI {
			gt.Equal(t, original.OpenAI[i].Role, cloned.OpenAI[i].Role)
			gt.Equal(t, original.OpenAI[i].Content, cloned.OpenAI[i].Content)
			gt.Equal(t, original.OpenAI[i].Name, cloned.OpenAI[i].Name)

			// Check function call equality
			if original.OpenAI[i].FunctionCall != nil {
				gt.NotNil(t, cloned.OpenAI[i].FunctionCall)
				gt.Equal(t, original.OpenAI[i].FunctionCall.Name, cloned.OpenAI[i].FunctionCall.Name)
				gt.Equal(t, original.OpenAI[i].FunctionCall.Arguments, cloned.OpenAI[i].FunctionCall.Arguments)
			} else {
				gt.Nil(t, cloned.OpenAI[i].FunctionCall)
			}
		}

		// Verify independence - modifying clone should not affect original
		cloned.OpenAI[0].Content = "Modified content"
		gt.NotEqual(t, original.OpenAI[0].Content, cloned.OpenAI[0].Content)
		gt.Equal(t, "You are a helpful assistant.", original.OpenAI[0].Content)
		gt.Equal(t, "Modified content", cloned.OpenAI[0].Content)

		// Verify slice independence
		cloned.OpenAI = append(cloned.OpenAI, openai.ChatCompletionMessage{
			Role:    "user",
			Content: "New message",
		})
		gt.Equal(t, 4, len(original.OpenAI))
		gt.Equal(t, 5, len(cloned.OpenAI))
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

		original := gollem.NewHistoryFromClaude(messages)
		cloned := original.Clone()

		// Verify basic properties
		gt.NotNil(t, cloned)
		gt.Equal(t, original.LLType, cloned.LLType)
		gt.Equal(t, original.Version, cloned.Version)
		gt.Equal(t, len(original.Claude), len(cloned.Claude))

		// Verify content structure
		for i := range original.Claude {
			gt.Equal(t, original.Claude[i].Role, cloned.Claude[i].Role)
			gt.Equal(t, len(original.Claude[i].Content), len(cloned.Claude[i].Content))

			for j := range original.Claude[i].Content {
				gt.Equal(t, original.Claude[i].Content[j].Type, cloned.Claude[i].Content[j].Type)
			}
		}

		// Verify independence by converting back and checking
		originalConverted, err := original.ToClaude()
		gt.NoError(t, err)
		clonedConverted, err := cloned.ToClaude()
		gt.NoError(t, err)

		// Should be equal at this point
		gt.Equal(t, len(originalConverted), len(clonedConverted))

		// Verify slice independence
		cloned.Claude = append(cloned.Claude, cloned.Claude[0])
		gt.Equal(t, 3, len(original.Claude))
		gt.Equal(t, 4, len(cloned.Claude))
	})

	t.Run("gemini history clone", func(t *testing.T) {
		messages := []*genai.Content{
			{
				Role: "user",
				Parts: []genai.Part{
					genai.Text("Hello"),
					genai.Blob{
						MIMEType: "image/jpeg",
						Data:     []byte("fake image data"),
					},
				},
			},
			{
				Role: "model",
				Parts: []genai.Part{
					genai.FunctionCall{
						Name: "test_function",
						Args: map[string]interface{}{
							"param1": "value1",
							"param2": float64(123),
						},
					},
					genai.FunctionResponse{
						Name: "test_function",
						Response: map[string]interface{}{
							"status": "success",
							"data":   []interface{}{"item1", "item2"},
						},
					},
				},
			},
		}

		original := gollem.NewHistoryFromGemini(messages)
		cloned := original.Clone()

		// Verify basic properties
		gt.NotNil(t, cloned)
		gt.Equal(t, original.LLType, cloned.LLType)
		gt.Equal(t, original.Version, cloned.Version)
		gt.Equal(t, len(original.Gemini), len(cloned.Gemini))

		// Verify content structure
		for i := range original.Gemini {
			gt.Equal(t, original.Gemini[i].Role, cloned.Gemini[i].Role)
			gt.Equal(t, len(original.Gemini[i].Parts), len(cloned.Gemini[i].Parts))
		}

		// Verify independence by converting back
		originalConverted, err := original.ToGemini()
		gt.NoError(t, err)
		clonedConverted, err := cloned.ToGemini()
		gt.NoError(t, err)
		gt.Equal(t, originalConverted, clonedConverted)

		// Test data independence for byte slices
		if len(cloned.Gemini) > 0 && len(cloned.Gemini[0].Parts) > 1 {
			// Modify byte data in clone
			cloned.Gemini[0].Parts[1].Data[0] = 255
			gt.NotEqual(t, original.Gemini[0].Parts[1].Data[0], cloned.Gemini[0].Parts[1].Data[0])
		}

		// Test map independence
		if len(cloned.Gemini) > 1 && len(cloned.Gemini[1].Parts) > 0 {
			// Modify map in clone
			cloned.Gemini[1].Parts[0].Args["new_key"] = "new_value"
			gt.False(t, func() bool {
				_, exists := original.Gemini[1].Parts[0].Args["new_key"]
				return exists
			}())
		}

		// Verify slice independence
		cloned.Gemini = append(cloned.Gemini, cloned.Gemini[0])
		gt.Equal(t, 2, len(original.Gemini))
		gt.Equal(t, 3, len(cloned.Gemini))
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

		history := gollem.NewHistoryFromClaude(messages)
		cloned := history.Clone()

		gt.NotNil(t, cloned)
		gt.Equal(t, history.LLType, cloned.LLType)
		gt.Equal(t, history.Version, cloned.Version)
		gt.Equal(t, len(history.Claude), len(cloned.Claude))
		gt.Equal(t, 0, len(cloned.OpenAI))
		gt.Equal(t, 0, len(cloned.Gemini))

		// Verify independence by converting back to messages
		originalConverted, err := history.ToClaude()
		gt.NoError(t, err)
		clonedConverted, err := cloned.ToClaude()
		gt.NoError(t, err)

		// Should be equal initially
		gt.Equal(t, len(originalConverted), len(clonedConverted))

		// Verify slice independence
		cloned.Claude = append(cloned.Claude, cloned.Claude[0])
		gt.Equal(t, 1, len(history.Claude))
		gt.Equal(t, 2, len(cloned.Claude))
	})
}
