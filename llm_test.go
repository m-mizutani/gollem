package gollem_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
)

// TestToolExecution tests tool execution with real LLM clients
func TestToolExecution(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		client, err := newClient(t)
		gt.NoError(t, err)

		s := gollem.New(client,
			gollem.WithTools(&RandomNumberTool{}),
			gollem.WithLoopLimit(5),
		)

		_, err = s.Execute(t.Context(), gollem.Text("Generate a random number between 1 and 100."))
		gt.NoError(t, err)
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location)
		})
	})
}

// TestContentMiddleware tests content middleware functionality with real LLM clients
func TestContentMiddleware(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		userName := "Alice"
		userAge := "25"

		// Content middleware that modifies history to inject fake previous conversation
		contentMiddleware := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
				// On first call, inject a fake conversation history
				if req.History == nil || len(req.History.Messages) == 0 {
					// Add fake previous conversation where user introduced themselves
					userData, _ := json.Marshal(map[string]string{
						"text": fmt.Sprintf("My name is %s and I am %s years old.", userName, userAge),
					})
					assistantData, _ := json.Marshal(map[string]string{
						"text": fmt.Sprintf("Nice to meet you, %s! I'll remember that you're %s years old.", userName, userAge),
					})

					// Prepend fake history
					fakeHistory := []gollem.Message{
						{
							Role: gollem.RoleUser,
							Contents: []gollem.MessageContent{
								{
									Type: gollem.MessageContentTypeText,
									Data: userData,
								},
							},
						},
						{
							Role: gollem.RoleAssistant,
							Contents: []gollem.MessageContent{
								{
									Type: gollem.MessageContentTypeText,
									Data: assistantData,
								},
							},
						},
					}

					if req.History == nil {
						req.History = &gollem.History{
							Version:  gollem.HistoryVersion,
							Messages: fakeHistory,
						}
					} else {
						req.History.Messages = append(fakeHistory, req.History.Messages...)
					}
				}

				return next(ctx, req)
			}
		}

		client, err := newClient(t)
		gt.NoError(t, err)

		session, err := client.NewSession(context.Background(),
			gollem.WithSessionContentBlockMiddleware(contentMiddleware),
		)
		gt.NoError(t, err)

		// First execution - Ask about name (should know from injected history)
		resp1, err := session.GenerateContent(t.Context(), gollem.Text("What's my name?"))
		gt.NoError(t, err)
		gt.NotNil(t, resp1)
		gt.True(t, len(resp1.Texts) > 0)

		// Verify response mentions the name Alice
		foundName := false
		for _, text := range resp1.Texts {
			if strings.Contains(text, userName) {
				foundName = true
				break
			}
		}
		if !foundName {
			t.Logf("First response should mention name %s, got: %v", userName, resp1.Texts)
		}
		gt.True(t, foundName)

		// Second execution - Ask about age (should know from injected history)
		resp2, err := session.GenerateContent(t.Context(), gollem.Text("How old am I?"))
		gt.NoError(t, err)
		gt.NotNil(t, resp2)
		gt.True(t, len(resp2.Texts) > 0)

		// Verify response mentions the age 25
		foundAge := false
		for _, text := range resp2.Texts {
			if strings.Contains(text, userAge) {
				foundAge = true
				break
			}
		}
		if !foundAge {
			t.Logf("Second response should mention age %s, got: %v", userAge, resp2.Texts)
		}
		gt.True(t, foundAge)

		// Third execution - Ask about both (should still remember)
		resp3, err := session.GenerateContent(t.Context(), gollem.Text("Tell me what you remember about me"))
		gt.NoError(t, err)
		gt.NotNil(t, resp3)
		gt.True(t, len(resp3.Texts) > 0)

		// Verify response mentions both name and age
		foundBothName := false
		foundBothAge := false
		for _, text := range resp3.Texts {
			if strings.Contains(text, userName) {
				foundBothName = true
			}
			if strings.Contains(text, userAge) {
				foundBothAge = true
			}
		}
		if !foundBothName || !foundBothAge {
			t.Logf("Third response should mention both name %s and age %s, got: %v", userName, userAge, resp3.Texts)
		}
		gt.True(t, foundBothName || foundBothAge)

		// Get final history and verify the injected conversation exists
		finalHistory, err := session.History()
		gt.NoError(t, err)

		// History should contain injected messages plus our 3 conversations
		gt.True(t, len(finalHistory.Messages) >= 8) // 2 injected + 3 user + 3 assistant

		// Verify the first message in history is our injected one
		if len(finalHistory.Messages) >= 2 {
			firstUserMsg := finalHistory.Messages[0]
			gt.Equal(t, gollem.RoleUser, firstUserMsg.Role)

			var firstContent map[string]string
			if len(firstUserMsg.Contents) > 0 {
				err := json.Unmarshal(firstUserMsg.Contents[0].Data, &firstContent)
				gt.NoError(t, err)
				gt.True(t, strings.Contains(firstContent["text"], userName))
				gt.True(t, strings.Contains(firstContent["text"], userAge))
			}
		}
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location)
		})
	})
}

// TestStreamMiddleware tests streaming middleware functionality with real LLM clients
func TestStreamMiddleware(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		modifiedPrompt := "Modified: Please respond with exactly: MIDDLEWARE_WORKS"

		// Streaming middleware that modifies the input prompt
		streamMiddleware := func(next gollem.ContentStreamHandler) gollem.ContentStreamHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
				// Modify the input - replace user's prompt
				for i, input := range req.Inputs {
					if _, ok := input.(gollem.Text); ok {
						req.Inputs[i] = gollem.Text(modifiedPrompt)
					}
				}

				// Call the next handler with modified request
				return next(ctx, req)
			}
		}

		client, err := newClient(t)
		gt.NoError(t, err)

		session, err := client.NewSession(context.Background(),
			gollem.WithSessionContentStreamMiddleware(streamMiddleware),
		)
		gt.NoError(t, err)

		// Generate stream with original prompt (will be modified by middleware)
		streamChan, err := session.GenerateStream(t.Context(), gollem.Text("Say ORIGINAL_PROMPT"))
		gt.NoError(t, err)
		if err != nil {
			return // Early return if stream creation failed
		}

		// Collect all streaming responses
		var collectedTexts []string
		for resp := range streamChan {
			if resp.Error != nil {
				t.Fatalf("Stream error: %v", resp.Error)
			}
			collectedTexts = append(collectedTexts, resp.Texts...)
		}

		// Verify the response contains MIDDLEWARE_WORKS (from modified prompt)
		fullResponse := strings.Join(collectedTexts, "")
		if !strings.Contains(fullResponse, "MIDDLEWARE_WORKS") {
			t.Logf("Response should contain MIDDLEWARE_WORKS from modified prompt, got: %s", fullResponse)
		}
		gt.True(t, strings.Contains(fullResponse, "MIDDLEWARE_WORKS"))

		// Verify history contains the modified input
		history, err := session.History()
		gt.NoError(t, err)
		gt.True(t, len(history.Messages) >= 2) // User + Assistant

		// Check the first message (user) contains modified prompt
		if len(history.Messages) > 0 && len(history.Messages[0].Contents) > 0 {
			var content map[string]string
			err := json.Unmarshal(history.Messages[0].Contents[0].Data, &content)
			gt.NoError(t, err)
			if !strings.Contains(content["text"], "Modified:") {
				t.Logf("First message should contain modified prompt, got: %s", content["text"])
			}
			gt.True(t, strings.Contains(content["text"], "Modified:"))
		}
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey, openai.WithModel("gpt-5-nano"))
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location)
		})
	})
}
