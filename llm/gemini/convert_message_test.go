package gemini_test

import (
	"encoding/json"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/internal/historytest"
	"github.com/gollem-dev/gollem/llm/gemini"
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
								ID:       part.FunctionResponse.ID,
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
			history, err := gemini.NewHistory(tc.contents, testIssuer)
			gt.NoError(t, err)

			// Convert back to Gemini contents
			restored, err := gemini.ToContents(history, testIssuer)
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

	t.Run("PDF inline data", runTest(testCase{
		name: "PDF inline data",
		contents: []*genai.Content{
			{
				Role: "user",
				Parts: []*genai.Part{
					{Text: "Analyze this PDF"},
					{InlineData: &genai.Blob{MIMEType: "application/pdf", Data: []byte("%PDF-1.4 test")}},
				},
			},
			{
				Role:  "model",
				Parts: []*genai.Part{{Text: "This PDF contains test data."}},
			},
		},
	}))

	t.Run("thought signature on function call", runTest(testCase{
		name: "thought signature on function call",
		contents: []*genai.Content{
			{
				Role:  "user",
				Parts: []*genai.Part{{Text: "Write a file"}},
			},
			{
				Role: "model",
				Parts: []*genai.Part{
					{
						Text:             "Let me think about this...",
						Thought:          true,
						ThoughtSignature: []byte("thought-sig-001"),
					},
					{
						Text:             "I'll write the file for you.",
						ThoughtSignature: []byte("text-sig-002"),
					},
					{
						FunctionCall: &genai.FunctionCall{
							Name: "write_file",
							Args: map[string]any{"path": "test.txt", "content": "hello"},
						},
						ThoughtSignature: []byte("fc-sig-003"),
					},
				},
			},
		},
	}))

	t.Run("thought part only", runTest(testCase{
		name: "thought part only",
		contents: []*genai.Content{
			{
				Role:  "user",
				Parts: []*genai.Part{{Text: "Hello"}},
			},
			{
				Role: "model",
				Parts: []*genai.Part{
					{
						Text:             "Internal reasoning...",
						Thought:          true,
						ThoughtSignature: []byte("thought-sig"),
					},
					{
						Text: "Hello! How can I help you?",
					},
				},
			},
		},
	}))
}

func TestThoughtSignatureRoundTrip(t *testing.T) {
	// Verify that ThoughtSignature is preserved through Gemini -> Message -> Gemini conversion
	contents := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Write a file"}},
		},
		{
			Role: "model",
			Parts: []*genai.Part{
				{
					Text:             "Thinking...",
					Thought:          true,
					ThoughtSignature: []byte("thought-sig-abc"),
				},
				{
					FunctionCall: &genai.FunctionCall{
						Name: "write_file",
						Args: map[string]any{"path": "test.txt"},
					},
					ThoughtSignature: []byte("fc-sig-def"),
				},
			},
		},
	}

	// Convert to gollem History
	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)

	// Verify the contents carry provider-bound data issued by testIssuer
	gt.A(t, history.Messages).Length(2)
	modelMsg := history.Messages[1]
	gt.A(t, modelMsg.Contents).Length(2)

	gt.NotNil(t, modelMsg.Contents[0].Provider)
	gt.Equal(t, testIssuer, modelMsg.Contents[0].Provider.Issuer)
	gt.NotNil(t, modelMsg.Contents[1].Provider)
	gt.Equal(t, testIssuer, modelMsg.Contents[1].Provider.Issuer)

	// Convert back to Gemini contents
	restored, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)

	gt.A(t, restored).Length(2)
	modelContent := restored[1]
	gt.A(t, modelContent.Parts).Length(2)

	// Verify thought part preserved
	thoughtPart := modelContent.Parts[0]
	gt.Value(t, thoughtPart.Thought).Equal(true)
	gt.Value(t, thoughtPart.ThoughtSignature).Equal([]byte("thought-sig-abc"))
	gt.Value(t, thoughtPart.Text).Equal("Thinking...")

	// Verify function call part preserved
	fcPart := modelContent.Parts[1]
	gt.Value(t, fcPart.ThoughtSignature).Equal([]byte("fc-sig-def"))
	gt.Value(t, fcPart.FunctionCall.Name).Equal("write_file")
}

func TestThoughtPartsExcludedFromResponse(t *testing.T) {
	// Verify that thought parts (Thought: true) are converted to ThinkingContentType.
	// This ensures proper separation of internal reasoning from visible response.

	// Convert a model message with thought parts to gollem format
	contents := []*genai.Content{
		{
			Role: "model",
			Parts: []*genai.Part{
				{
					Text:    "Internal reasoning",
					Thought: true,
				},
				{
					Text: "Visible response",
				},
			},
		},
	}

	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)

	// Both parts should be in the message (for history preservation)
	gt.A(t, history.Messages[0].Contents).Length(2)

	// First part should be ThinkingContentType with provider-bound data
	thoughtContent := history.Messages[0].Contents[0]
	gt.Value(t, thoughtContent.Type).Equal(gollem.MessageContentTypeThinking)
	gt.NotNil(t, thoughtContent.Provider)

	// Normal text without thought/signature has no provider-bound data
	normalContent := history.Messages[0].Contents[1]
	gt.Value(t, normalContent.Type).Equal(gollem.MessageContentTypeText)
	gt.Nil(t, normalContent.Provider)
}

func TestContentsWithoutSignatures(t *testing.T) {
	// Verify that contents without any thought or signature carry no provider-bound data
	contents := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "Hello"}},
		},
		{
			Role: "model",
			Parts: []*genai.Part{
				{Text: "Hi there!"},
				{
					FunctionCall: &genai.FunctionCall{
						Name: "greet",
						Args: map[string]any{"name": "world"},
					},
				},
			},
		},
	}

	// Convert to history (no ThoughtSignature anywhere)
	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)

	for _, msg := range history.Messages {
		for _, content := range msg.Contents {
			gt.Nil(t, content.Provider)
		}
	}

	// Convert back should work without error
	restored, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)

	// Restored parts should have zero-value Thought/ThoughtSignature
	modelContent := restored[1]
	for _, part := range modelContent.Parts {
		gt.Value(t, part.Thought).Equal(false)
		gt.Value(t, part.ThoughtSignature).Equal([]byte(nil))
	}
}

// TestFunctionCallIDPreservedWithExplicitID verifies that Gemini 3.x style
// FunctionCall/FunctionResponse IDs are propagated through the gollem.History
// boundary in both directions. Without this, parallel tool calls under the
// strict-match contract would lose correlation.
func TestFunctionCallIDPreservedWithExplicitID(t *testing.T) {
	const callID = "call_abc123"

	contents := []*genai.Content{
		{
			Role:  "user",
			Parts: []*genai.Part{{Text: "What's the weather?"}},
		},
		{
			Role: "model",
			Parts: []*genai.Part{{
				FunctionCall: &genai.FunctionCall{
					ID:   callID,
					Name: "get_weather",
					Args: map[string]any{"location": "Tokyo"},
				},
			}},
		},
		{
			Role: "user",
			Parts: []*genai.Part{{
				FunctionResponse: &genai.FunctionResponse{
					ID:       callID,
					Name:     "get_weather",
					Response: map[string]any{"temperature": float64(25)},
				},
			}},
		},
	}

	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)

	// gollem.ToolCallContent.ID must mirror FunctionCall.ID.
	callContent, err := history.Messages[1].Contents[0].GetToolCallContent()
	gt.NoError(t, err)
	gt.Value(t, callContent.ID).Equal(callID)

	respContent, err := history.Messages[2].Contents[0].GetToolResponseContent()
	gt.NoError(t, err)
	gt.Value(t, respContent.ToolCallID).Equal(callID)

	// Round-trip back into Gemini parts and ensure IDs survive.
	restored, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)

	gt.Value(t, restored[1].Parts[0].FunctionCall.ID).Equal(callID)
	gt.Value(t, restored[2].Parts[0].FunctionResponse.ID).Equal(callID)
}

// TestFunctionCallIDBackwardCompatNoID verifies that legacy responses without
// FunctionCall.ID still work: gollem fabricates a stable fallback id internally
// and strips it on the way out so older Gemini models do not see a synthetic id.
func TestFunctionCallIDBackwardCompatNoID(t *testing.T) {
	contents := []*genai.Content{
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
					Response: map[string]any{"temperature": float64(25)},
				},
			}},
		},
	}

	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)

	// Fallback id is populated for internal correlation.
	callContent, err := history.Messages[0].Contents[0].GetToolCallContent()
	gt.NoError(t, err)
	gt.Value(t, callContent.ID).NotEqual("")

	respContent, err := history.Messages[1].Contents[0].GetToolResponseContent()
	gt.NoError(t, err)
	gt.Value(t, respContent.ToolCallID).Equal(callContent.ID)

	// On the way out, the fallback id must be stripped so we do not feed Gemini
	// an id it never issued.
	restored, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)

	gt.Value(t, restored[0].Parts[0].FunctionCall.ID).Equal("")
	gt.Value(t, restored[1].Parts[0].FunctionResponse.ID).Equal("")
}

func TestToContentsLeavesHistoryUnchanged(t *testing.T) {
	sys, err := gollem.NewTextContent("be brief")
	gt.NoError(t, err)
	user, err := gollem.NewTextContent("hello")
	gt.NoError(t, err)
	assistant, err := gollem.NewTextContent("hi")
	gt.NoError(t, err)
	resp1, err := gollem.NewToolResponseContent("c1", "alpha", map[string]any{"ok": true}, false)
	gt.NoError(t, err)
	resp2, err := gollem.NewToolResponseContent("c2", "beta", map[string]any{"ok": true}, false)
	gt.NoError(t, err)

	history := &gollem.History{
		LLType:  gollem.LLMTypeGemini,
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{Role: gollem.RoleSystem, Contents: []gollem.MessageContent{sys}},
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{user}},
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{assistant}},
			{Role: gollem.RoleTool, Contents: []gollem.MessageContent{resp1}},
			{Role: gollem.RoleTool, Contents: []gollem.MessageContent{resp2}},
		},
	}
	before := append([]gollem.Message(nil), history.Messages...)

	first, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)

	// The same History is converted again on every later request, so a conversion must not
	// consume the system message or duplicate the trailing message.
	gt.Equal(t, before, history.Messages)

	second, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)
	gt.Equal(t, first, second)
}

func TestToolResponsesInSeparateMessagesBecomeOneContent(t *testing.T) {
	text, err := gollem.NewTextContent("go")
	gt.NoError(t, err)
	call1, err := gollem.NewToolCallContent("c1", "alpha", map[string]any{"x": 1})
	gt.NoError(t, err)
	call2, err := gollem.NewToolCallContent("c2", "beta", map[string]any{"y": 2})
	gt.NoError(t, err)
	resp1, err := gollem.NewToolResponseContent("c1", "alpha", map[string]any{"ok": true}, false)
	gt.NoError(t, err)
	resp2, err := gollem.NewToolResponseContent("c2", "beta", map[string]any{"ok": true}, false)
	gt.NoError(t, err)

	history := &gollem.History{
		LLType:  gollem.LLMTypeGemini,
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{text}},
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{call1, call2}},
			// A runtime that runs the calls one at a time appends one message per result.
			{Role: gollem.RoleTool, Contents: []gollem.MessageContent{resp1}},
			{Role: gollem.RoleTool, Contents: []gollem.MessageContent{resp2}},
		},
	}

	contents, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)

	// Gemini requires the answering turn to carry as many functionResponse parts as the
	// call turn carried functionCall parts.
	gt.Equal(t, 3, len(contents))
	gt.Equal(t, "user", contents[2].Role)
	gt.Equal(t, 2, len(contents[2].Parts))
	gt.Value(t, contents[2].Parts[0].FunctionResponse.Name).Equal("alpha")
	gt.Value(t, contents[2].Parts[1].FunctionResponse.Name).Equal("beta")
}

// This package's own conversion carries an integer wider than float64 through unchanged.
//
// It does not survive the request, and cannot be made to from here: genai's
// Models.generateContent passes the whole request through InternalDeepMarshal
// (common.go:371-378 in v1.53.0), which is json.Marshal followed by a plain json.Unmarshal
// into map[string]any, so every number becomes a float64 before the body is built. The
// inbound direction has the same shape. The test therefore pins the boundary this package
// controls; the Gemini limitation is recorded in docs/tools.md.
func TestGeminiHistoryPreservesWideIntegers(t *testing.T) {
	const wide = "9007199254740993"

	call, err := gollem.NewToolCallContent("call_1", "lookup", map[string]any{"id": json.Number(wide)})
	gt.NoError(t, err)
	resp, err := gollem.NewToolResponseContent("call_1", "lookup", map[string]any{"account": json.Number(wide)}, false)
	gt.NoError(t, err)

	history := &gollem.History{
		LLType:  gollem.LLMTypeGemini,
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{call}},
			{Role: gollem.RoleTool, Contents: []gollem.MessageContent{resp}},
		},
	}

	contents, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)

	encodedArgs, err := json.Marshal(contents[0].Parts[0].FunctionCall.Args)
	gt.NoError(t, err)
	gt.Equal(t, `{"id":`+wide+`}`, string(encodedArgs))

	encodedResp, err := json.Marshal(contents[1].Parts[0].FunctionResponse.Response)
	gt.NoError(t, err)
	gt.Equal(t, `{"account":`+wide+`}`, string(encodedResp))
}

var testIssuer = gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "gemini-test"}

var claudeIssuer = gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "claude-test"}

func TestSignatureOnlyPartBecomesThinking(t *testing.T) {
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "go"}}},
		{Role: "model", Parts: []*genai.Part{
			{Text: "answer"},
			{ThoughtSignature: []byte("sig")},
		}},
	}

	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)

	sigContent := history.Messages[1].Contents[1]
	gt.Equal(t, gollem.MessageContentTypeThinking, sigContent.Type)
	thinking, err := sigContent.GetThinkingContent()
	gt.NoError(t, err)
	gt.Equal(t, "", thinking.Text)
	gt.NotNil(t, sigContent.Provider)
	gt.Equal(t, testIssuer, sigContent.Provider.Issuer)
	gt.Equal(t, `{"thought_signature":"c2ln"}`, string(sigContent.Provider.Data))

	t.Run("same issuer restores a part with only the signature", func(t *testing.T) {
		restored, err := gemini.ToContents(history, testIssuer)
		gt.NoError(t, err)
		gt.Equal(t, contents, restored)
	})

	t.Run("another issuer drops the signature-only part", func(t *testing.T) {
		restored, err := gemini.ToContents(history, claudeIssuer)
		gt.NoError(t, err)
		gt.A(t, restored[1].Parts).Length(1).Required()
		gt.Equal(t, "answer", restored[1].Parts[0].Text)
	})
}

func TestSignedPartsAcrossIssuers(t *testing.T) {
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "go"}}},
		{Role: "model", Parts: []*genai.Part{
			{Text: "calling", ThoughtSignature: []byte("text-sig")},
			{FunctionCall: &genai.FunctionCall{ID: "c1", Name: "search", Args: map[string]any{"q": "x"}}, ThoughtSignature: []byte("fc-sig")},
		}},
	}

	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)
	call := history.Messages[1].Contents[1]
	gt.Equal(t, gollem.MessageContentTypeToolCall, call.Type)
	gt.NotNil(t, call.Provider)

	t.Run("same issuer keeps the signatures", func(t *testing.T) {
		restored, err := gemini.ToContents(history, testIssuer)
		gt.NoError(t, err)
		gt.Equal(t, []byte("text-sig"), restored[1].Parts[0].ThoughtSignature)
		gt.Equal(t, []byte("fc-sig"), restored[1].Parts[1].ThoughtSignature)
	})

	t.Run("another issuer sends the parts without signatures", func(t *testing.T) {
		restored, err := gemini.ToContents(history, gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "gemini-other"})
		gt.NoError(t, err)
		gt.A(t, restored[1].Parts).Length(2).Required()
		gt.Equal(t, "calling", restored[1].Parts[0].Text)
		gt.Nil(t, restored[1].Parts[0].ThoughtSignature)
		gt.Equal(t, "search", restored[1].Parts[1].FunctionCall.Name)
		gt.Nil(t, restored[1].Parts[1].ThoughtSignature)
	})
}

func TestSignedImagePart(t *testing.T) {
	contents := []*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "draw"}}},
		{Role: "model", Parts: []*genai.Part{
			{InlineData: &genai.Blob{MIMEType: "image/png", Data: []byte("png-bytes")}, ThoughtSignature: []byte("img-sig")},
		}},
	}

	history, err := gemini.NewHistory(contents, testIssuer)
	gt.NoError(t, err)
	image := history.Messages[1].Contents[0]
	gt.Equal(t, gollem.MessageContentTypeImage, image.Type)
	gt.NotNil(t, image.Provider)
	gt.Equal(t, testIssuer, image.Provider.Issuer)

	// Go through JSON, as a stored history does.
	data, err := json.Marshal(history)
	gt.NoError(t, err)
	var restoredHistory gollem.History
	gt.NoError(t, json.Unmarshal(data, &restoredHistory))

	t.Run("same issuer restores the signature", func(t *testing.T) {
		restored, err := gemini.ToContents(&restoredHistory, testIssuer)
		gt.NoError(t, err)
		gt.Equal(t, contents, restored)
	})

	t.Run("another issuer sends the image without the signature", func(t *testing.T) {
		restored, err := gemini.ToContents(&restoredHistory, gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "gemini-other"})
		gt.NoError(t, err)
		gt.A(t, restored[1].Parts).Length(1).Required()
		gt.Equal(t, []byte("png-bytes"), restored[1].Parts[0].InlineData.Data)
		gt.Nil(t, restored[1].Parts[0].ThoughtSignature)
	})
}

func TestToContentsDropsOtherProviderThinking(t *testing.T) {
	user, err := gollem.NewTextContent("go")
	gt.NoError(t, err)
	thinking, err := gollem.NewThinkingContent("claude reasoning")
	gt.NoError(t, err)
	thinking.Provider = &gollem.ProviderData{Issuer: claudeIssuer, Data: json.RawMessage(`{"signature":"sig"}`)}
	answer, err := gollem.NewTextContent("answer")
	gt.NoError(t, err)

	history := &gollem.History{
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{user}},
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{thinking, answer}},
			// An assistant message holding only another provider's thinking.
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{thinking}},
		},
	}

	contents, err := gemini.ToContents(history, testIssuer)
	gt.NoError(t, err)
	// The thinking-only message is removed instead of being sent with no parts.
	gt.A(t, contents).Length(2).Required()
	for _, c := range contents {
		gt.A(t, c.Parts).Longer(0)
		for _, p := range c.Parts {
			gt.False(t, p.Thought)
		}
	}
}

// Cross-provider conversion tests. Each one is split at the gollem.History shared
// through internal/historytest; the test with the same name in the other provider
// package converts that History. See the historytest package documentation.

// History → Gemini. llm/claude converts Claude messages to the History.
func TestClaudeToGeminiConversion(t *testing.T) {
	runTest := func(expected []*genai.Content) func(t *testing.T) {
		return func(t *testing.T) {
			contents, err := gemini.ToContents(historytest.Load(t, "claude_to_gemini", "claude"), testIssuer)
			gt.NoError(t, err)
			gt.Equal(t, expected, contents)
		}
	}

	t.Run("text messages", runTest([]*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Hello, how are you?"}}},
		{Role: "model", Parts: []*genai.Part{{Text: "I'm doing well, thank you!"}}},
	}))

	t.Run("tool use with multiple calls", runTest([]*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Calculate 5+3 and 10*2"}}},
		{
			Role: "model",
			Parts: []*genai.Part{
				// Claude's tool_use IDs must be propagated to Gemini for
				// Gemini 3.x strict id matching.
				{FunctionCall: &genai.FunctionCall{ID: "toolu_123", Name: "calculate", Args: map[string]any{"expression": "5+3"}}},
				{FunctionCall: &genai.FunctionCall{ID: "toolu_456", Name: "calculate", Args: map[string]any{"expression": "10*2"}}},
			},
		},
		{
			Role: "user",
			Parts: []*genai.Part{
				// Claude now parses JSON, so result is properly structured. The tool name is
				// recovered from the tool_use block with the same ID, since a Claude
				// tool_result carries none and Gemini requires one.
				{FunctionResponse: &genai.FunctionResponse{ID: "toolu_123", Name: "calculate", Response: map[string]any{"result": float64(8)}}},
				{FunctionResponse: &genai.FunctionResponse{ID: "toolu_456", Name: "calculate", Response: map[string]any{"result": float64(20)}}},
			},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "5+3 equals 8, and 10*2 equals 20."}}},
	}))

	t.Run("mixed content blocks", runTest([]*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Tell me a joke and check the time"}}},
		{
			Role: "model",
			Parts: []*genai.Part{
				{Text: "Here's a joke: Why did the chicken cross the road?"},
				{FunctionCall: &genai.FunctionCall{ID: "toolu_789", Name: "get_current_time", Args: map[string]any{}}},
			},
		},
		{
			Role: "user",
			Parts: []*genai.Part{
				// Claude parses JSON response
				{FunctionResponse: &genai.FunctionResponse{ID: "toolu_789", Name: "get_current_time", Response: map[string]any{"time": "14:30:00", "timezone": "UTC"}}},
			},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "It's currently 14:30 UTC."}}},
	}))

	t.Run("image content", runTest([]*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: "Analyze this image"},
				{InlineData: &genai.Blob{MIMEType: "image/jpeg", Data: []byte{0xff, 0xd8, 0xff, 0xe0, 0x00, 0x10, 0x4a, 0x46, 0x49, 0x46}}},
			},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "This appears to be a landscape photo."}}},
	}))

	t.Run("error tool result", runTest([]*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Get the weather"}}},
		{
			Role: "model",
			Parts: []*genai.Part{
				{FunctionCall: &genai.FunctionCall{ID: "toolu_error", Name: "get_weather", Args: map[string]any{"location": "InvalidCity"}}},
			},
		},
		{
			Role: "user",
			Parts: []*genai.Part{
				// Error responses are also parsed as JSON
				{FunctionResponse: &genai.FunctionResponse{ID: "toolu_error", Name: "get_weather", Response: map[string]any{"error": "City not found"}}},
			},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "I couldn't find that city."}}},
	}))

	t.Run("PDF content", runTest([]*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: "Analyze this PDF"},
				{InlineData: &genai.Blob{MIMEType: "application/pdf", Data: []byte("%PDF-1.4 test")}},
			},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "This PDF contains test data."}}},
	}))
}

// Gemini → History. llm/openai converts the History to OpenAI messages.
func TestGeminiToOpenAIConversion(t *testing.T) {
	runTest := func(contents []*genai.Content) func(t *testing.T) {
		return func(t *testing.T) {
			history, err := gemini.NewHistory(contents, testIssuer)
			gt.NoError(t, err).Required()
			historytest.Equal(t, "gemini_to_openai", "gemini", history)
		}
	}

	t.Run("text messages", runTest([]*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Hello from Gemini"}}},
		{Role: "model", Parts: []*genai.Part{{Text: "Hello! How can I assist you?"}}},
	}))

	t.Run("function calls with complex args", runTest([]*genai.Content{
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
	}))

	t.Run("multiple parts in single message", runTest([]*genai.Content{
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
	}))

	t.Run("PDF content", runTest([]*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: "Analyze this PDF"},
				{InlineData: &genai.Blob{MIMEType: "application/pdf", Data: []byte("%PDF-1.4 test")}},
			},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "This PDF contains test data."}}},
	}))
}

// Gemini → History → OpenAI → History → Gemini must restore the original contents.
// This test covers both ends; llm/openai converts the first History through OpenAI
// messages into the second.
func TestGeminiRoundTrip(t *testing.T) {
	runTest := func(contents []*genai.Content) func(t *testing.T) {
		return func(t *testing.T) {
			history, err := gemini.NewHistory(contents, testIssuer)
			gt.NoError(t, err).Required()
			historytest.Equal(t, "gemini_round_trip", "gemini", history)

			restored, err := gemini.ToContents(historytest.Load(t, "gemini_round_trip", "openai"), testIssuer)
			gt.NoError(t, err)
			gt.Equal(t, contents, restored)
		}
	}

	t.Run("text messages", runTest([]*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Hello"}}},
		{Role: "model", Parts: []*genai.Part{{Text: "Hi!"}}},
	}))

	t.Run("function calls", runTest([]*genai.Content{
		{Role: "user", Parts: []*genai.Part{{Text: "Search Python"}}},
		{
			Role: "model",
			Parts: []*genai.Part{{
				FunctionCall: &genai.FunctionCall{
					Name: "search",
					Args: map[string]any{"query": "Python"},
				},
			}},
		},
		{
			Role: "user",
			Parts: []*genai.Part{{
				FunctionResponse: &genai.FunctionResponse{
					Name:     "search",
					Response: map[string]any{"results": []any{"Python tutorial"}},
				},
			}},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "Found Python tutorial."}}},
	}))

	t.Run("PDF content", runTest([]*genai.Content{
		{
			Role: "user",
			Parts: []*genai.Part{
				{Text: "Analyze this PDF"},
				{InlineData: &genai.Blob{MIMEType: "application/pdf", Data: []byte("%PDF-1.4 test")}},
			},
		},
		{Role: "model", Parts: []*genai.Part{{Text: "This PDF contains test data."}}},
	}))
}

// The Gemini leg of Claude → History → Gemini → History → Claude. llm/claude covers
// both ends and compares the restored messages with the original.
func TestClaudeRoundTrip(t *testing.T) {
	run := func(t *testing.T) {
		contents, err := gemini.ToContents(historytest.Load(t, "claude_round_trip", "claude"), testIssuer)
		gt.NoError(t, err).Required()

		history, err := gemini.NewHistory(contents, testIssuer)
		gt.NoError(t, err).Required()
		historytest.Equal(t, "claude_round_trip", "gemini", history)
	}

	t.Run("text messages", run)
	t.Run("PDF content", run)
}
