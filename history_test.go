package gollem_test

import (
	"encoding/json"
	"testing"

	"cloud.google.com/go/vertexai/genai"
	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/sashabaranov/go-openai"
)

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
				anthropic.NewToolResultBlock("tool_2", `{"temperature": 30, "condition": "cloudy"}`, false),
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
				anthropic.NewToolResultBlock("tool_3", `{"temperature": 35, "condition": "rainy"}`, false),
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
