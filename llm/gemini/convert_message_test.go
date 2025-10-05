package gemini_test

import (
	"encoding/json"
	"testing"

	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gt"
	"google.golang.org/genai"
)

// normalizeGeminiMessages normalizes JSON content in function response parts
// to handle JSON key reordering during round-trip conversion
func normalizeGeminiMessages(contents []*genai.Content) []*genai.Content {
	result := make([]*genai.Content, len(contents))
	for i, content := range contents {
		normalizedParts := make([]*genai.Part, len(content.Parts))
		for j, part := range content.Parts {
			// Normalize FunctionResponse parts
			if part.FunctionResponse != nil && part.FunctionResponse.Response != nil {
				// Re-marshal to normalize JSON key order
				if data, err := json.Marshal(part.FunctionResponse.Response); err == nil {
					var normalized map[string]any
					if err := json.Unmarshal(data, &normalized); err == nil {
						normalizedParts[j] = &genai.Part{
							FunctionResponse: &genai.FunctionResponse{
								Name:     part.FunctionResponse.Name,
								Response: normalized,
							},
						}
						continue
					}
				}
			}
			normalizedParts[j] = part
		}
		result[i] = &genai.Content{
			Role:  content.Role,
			Parts: normalizedParts,
		}
	}
	return result
}

func TestGeminiMessageRoundTrip(t *testing.T) {
	type testCase struct {
		name     string
		contents []*genai.Content
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Convert Gemini contents to gollem.History
			history, err := gemini.NewHistory(tc.contents)
			gt.NoError(t, err)

			// Convert back to Gemini contents
			restored, err := gemini.ToContents(history)
			gt.NoError(t, err)

			// Normalize JSON content before comparison
			normalizedOrig := normalizeGeminiMessages(tc.contents)
			normalizedRest := normalizeGeminiMessages(restored)

			// Compare normalized messages
			gt.Equal(t, normalizedOrig, normalizedRest)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		contents: []*genai.Content{
			{
				Role:  "user",
				Parts: []*genai.Part{{Text: "Hello"}},
			},
			{
				Role:  "model",
				Parts: []*genai.Part{{Text: "Hi, how can I help you?"}},
			},
		},
	}))

	t.Run("function call and response", runTest(testCase{
		name: "function call and response",
		contents: []*genai.Content{
			{
				Role:  "user",
				Parts: []*genai.Part{{Text: "What's the weather?"}},
			},
			{
				Role: "model",
				Parts: []*genai.Part{{
					FunctionCall: &genai.FunctionCall{
						Name: "get_weather",
						Args: map[string]any{"location": "Tokyo"},
					},
				}},
			},
			{
				Role: "user",
				Parts: []*genai.Part{{
					FunctionResponse: &genai.FunctionResponse{
						Name:     "get_weather",
						Response: map[string]any{"temperature": float64(25), "condition": "sunny"},
					},
				}},
			},
			{
				Role:  "model",
				Parts: []*genai.Part{{Text: "The weather in Tokyo is sunny with a temperature of 25°C."}},
			},
		},
	}))

	t.Run("multiple parts", runTest(testCase{
		name: "multiple parts",
		contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Tell me a joke"},
					{Text: "and the weather"},
				},
			},
			{
				Role: "model",
				Parts: []*genai.Part{
					{Text: "Here's a joke: Why did the chicken cross the road?"},
					{
						FunctionCall: &genai.FunctionCall{
							Name: "get_weather",
							Args: map[string]any{"location": "London"},
						},
					},
				},
			},
		},
	}))
}
