package gemini_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gt"
	"google.golang.org/genai"
)

func TestMiddlewareHistoryIntervention(t *testing.T) {
	testHistoryModification := func(t *testing.T) {
		// Create a mock API client that returns a simple response
		mockClient := &apiClientMock{
			GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
				return &genai.GenerateContentResponse{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Parts: []*genai.Part{
									{Text: "Response from API"},
								},
								Role: "model",
							},
						},
					},
				}, nil
			},
			GenerateContentStreamFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) <-chan gemini.StreamResponse {
				ch := make(chan gemini.StreamResponse)
				close(ch)
				return ch
			},
		}

		// Create initial history with proper version
		initialHistory := &gollem.History{
			Version: gollem.HistoryVersion,
			LLType:  gollem.LLMTypeGemini,
		}

		// Create middleware that modifies history
		historyMiddleware := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
				// Modify the history before passing to next handler
				if req.History != nil {
					// Add a system message to history
					textData, _ := json.Marshal(map[string]string{
						"text": "History was modified by middleware",
					})
					modifiedHistory := &gollem.History{
						Version: req.History.Version,
						LLType:  req.History.LLType,
						Messages: append(req.History.Messages, gollem.Message{
							Role: gollem.RoleSystem,
							Contents: []gollem.MessageContent{
								{
									Type: gollem.MessageContentTypeText,
									Data: textData,
								},
							},
						}),
					}
					req.History = modifiedHistory
				}

				// Call the next handler
				resp, err := next(ctx, req)
				if err != nil {
					return nil, err
				}

				return resp, nil
			}
		}

		// Create session config with middleware
		cfg := gollem.NewSessionConfig(
			gollem.WithSessionHistory(initialHistory),
			gollem.WithSessionContentBlockMiddleware(historyMiddleware),
		)

		// Create session with mock client
		session := gemini.NewSessionWithAPIClient(mockClient, cfg, "gemini-1.5-pro")

		// Generate content
		ctx := context.Background()
		resp, err := session.GenerateContent(ctx, gollem.Text("test input"))
		gt.NoError(t, err)
		gt.NotNil(t, resp)

		// Get history from session and verify it was modified
		history, err := session.History()
		gt.NoError(t, err)
		gt.NotNil(t, history)

		// Check if history contains the middleware-added message
		found := false
		for _, msg := range history.Messages {
			if msg.Role == gollem.RoleSystem {
				for _, content := range msg.Contents {
					if content.Type == gollem.MessageContentTypeText {
						var textContent map[string]string
						if err := json.Unmarshal(content.Data, &textContent); err == nil {
							if textContent["text"] == "History was modified by middleware" {
								found = true
								break
							}
						}
					}
				}
			}
		}
		gt.True(t, found)
	}

	t.Run("history modification", testHistoryModification)
}

func TestMiddlewareChainExecution(t *testing.T) {
	testChainOrder := func(t *testing.T) {
		executionOrder := []string{}

		// Create a mock API client
		mockClient := &apiClientMock{
			GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
				executionOrder = append(executionOrder, "api_call")
				return &genai.GenerateContentResponse{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Parts: []*genai.Part{
									{Text: "response"},
								},
								Role: "model",
							},
						},
					},
				}, nil
			},
			GenerateContentStreamFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) <-chan gemini.StreamResponse {
				ch := make(chan gemini.StreamResponse)
				close(ch)
				return ch
			},
		}

		// Create middleware 1
		mw1 := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
				executionOrder = append(executionOrder, "mw1_before")
				resp, err := next(ctx, req)
				executionOrder = append(executionOrder, "mw1_after")
				return resp, err
			}
		}

		// Create middleware 2
		mw2 := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
				executionOrder = append(executionOrder, "mw2_before")
				resp, err := next(ctx, req)
				executionOrder = append(executionOrder, "mw2_after")
				return resp, err
			}
		}

		// Create session config with multiple middleware
		initialHistory := &gollem.History{
			Version: gollem.HistoryVersion,
			LLType:  gollem.LLMTypeGemini,
		}
		cfg := gollem.NewSessionConfig(
			gollem.WithSessionHistory(initialHistory),
			gollem.WithSessionContentBlockMiddleware(mw1),
			gollem.WithSessionContentBlockMiddleware(mw2),
		)

		// Create session with mock client
		session := gemini.NewSessionWithAPIClient(mockClient, cfg, "gemini-1.5-pro")

		// Generate content
		ctx := context.Background()
		_, err := session.GenerateContent(ctx, gollem.Text("test"))
		gt.NoError(t, err)

		// Verify execution order
		expected := []string{"mw1_before", "mw2_before", "api_call", "mw2_after", "mw1_after"}
		gt.Equal(t, expected, executionOrder)
	}

	t.Run("chain execution order", testChainOrder)
}

func TestMiddlewareSameAddressModifiedContent(t *testing.T) {
	testSameAddressModification := func(t *testing.T) {
		var receivedHistoryObjects []*gollem.History

		// Create a mock API client that returns a simple response
		mockClient := &apiClientMock{
			GenerateContentFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
				t.Logf("API called with %d contents", len(contents))
				return &genai.GenerateContentResponse{
					Candidates: []*genai.Candidate{
						{
							Content: &genai.Content{
								Parts: []*genai.Part{
									{Text: "Response from API"},
								},
								Role: "model",
							},
						},
					},
				}, nil
			},
			GenerateContentStreamFunc: func(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) <-chan gemini.StreamResponse {
				ch := make(chan gemini.StreamResponse)
				close(ch)
				return ch
			},
		}

		// Create initial history with proper version
		sharedHistory := &gollem.History{
			Version: gollem.HistoryVersion,
			LLType:  gollem.LLMTypeGemini,
		}

		// Create middleware that modifies the SAME history object's content and tracks history updates
		contentModifyingMiddleware := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
				// Track the received history object
				if req.History != nil {
					receivedHistoryObjects = append(receivedHistoryObjects, req.History)
					// Modify the existing history object's content (same address, different content)
					textData, _ := json.Marshal(map[string]string{
						"text": fmt.Sprintf("Modified content call %d", len(receivedHistoryObjects)),
					})
					// Append to the existing history object (same address)
					req.History.Messages = append(req.History.Messages, gollem.Message{
						Role: gollem.RoleSystem,
						Contents: []gollem.MessageContent{
							{
								Type: gollem.MessageContentTypeText,
								Data: textData,
							},
						},
					})
					t.Logf("Modified history object %p, now has %d messages", req.History, len(req.History.Messages))
				}

				// Call the next handler
				resp, err := next(ctx, req)
				if err != nil {
					return nil, err
				}

				return resp, nil
			}
		}

		// Create session config with middleware
		cfg := gollem.NewSessionConfig(
			gollem.WithSessionHistory(sharedHistory),
			gollem.WithSessionContentBlockMiddleware(contentModifyingMiddleware),
		)

		// Create session with mock client
		session := gemini.NewSessionWithAPIClient(mockClient, cfg, "gemini-1.5-pro")

		// Generate content multiple times with the same history object
		ctx := context.Background()

		// First call - should modify history
		_, err := session.GenerateContent(ctx, gollem.Text("first input"))
		gt.NoError(t, err)

		// Second call - should modify history again (same address but already modified content)
		_, err = session.GenerateContent(ctx, gollem.Text("second input"))
		gt.NoError(t, err)

		// Verify that middleware was called twice
		gt.Equal(t, 2, len(receivedHistoryObjects))

		// The key test: verify that history is properly updated between calls
		// This proves that history updates are applied correctly, regardless of address comparison
		t.Logf("First call history messages: %d", len(receivedHistoryObjects[0].Messages))
		t.Logf("Second call history messages: %d", len(receivedHistoryObjects[1].Messages))

		// First call should have at least the middleware addition
		gt.True(t, len(receivedHistoryObjects[0].Messages) >= 1)

		// Second call should have more messages (previous conversation + middleware addition)
		gt.True(t, len(receivedHistoryObjects[1].Messages) > len(receivedHistoryObjects[0].Messages))

		// This test verifies that our fix to "always update history" works correctly:
		// - Even if same address, the content changes are properly reflected
		// - The conversation history accumulates correctly across calls
	}

	t.Run("same address modified content", testSameAddressModification)
}
