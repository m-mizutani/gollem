package gollem_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gollem/llm/gemini"
	"github.com/m-mizutani/gollem/llm/openai"
	"github.com/m-mizutani/gt"
)

// mockLLMClient implements the LLMClient interface for testing
type mockLLMClient struct {
	responses []string
	index     int
	history   *gollem.History
}

func (m *mockLLMClient) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	responseIdx := m.index
	if responseIdx >= len(m.responses) {
		responseIdx = len(m.responses) - 1 // Use last response if index exceeds
	}
	if responseIdx < 0 || len(m.responses) == 0 {
		return &mockSession{
			client:   m,
			response: "Default summary",
		}, nil
	}
	return &mockSession{
		client:   m,
		response: m.responses[responseIdx],
	}, nil
}

func (m *mockLLMClient) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return nil, nil
}

func (m *mockLLMClient) CountTokens(ctx context.Context, history *gollem.History) (int, error) {
	// Simple token estimation for testing
	// Count characters in messages to simulate token counting
	totalChars := 0
	for _, msg := range history.Messages {
		for _, content := range msg.Contents {
			totalChars += len(content.Data)
		}
	}
	// Rough estimation: 4 chars = 1 token
	return totalChars / 4, nil
}

func (m *mockLLMClient) IsCompatibleHistory(ctx context.Context, history *gollem.History) error {
	return nil
}

type mockSession struct {
	client   *mockLLMClient
	response string
}

func (s *mockSession) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	s.client.index++
	return &gollem.Response{
		Texts: []string{s.response},
	}, nil
}

func (s *mockSession) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	ch := make(chan *gollem.Response, 1)
	ch <- &gollem.Response{
		Texts: []string{s.response},
	}
	close(ch)
	return ch, nil
}

func (s *mockSession) History() (*gollem.History, error) {
	return s.client.history, nil
}

func TestNewHistoryCompactor_PerformCompact_ShouldCompactLogic(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{"Summary of the conversation"},
	}
	compactor := gollem.NewHistoryCompactor(mockClient)
	ctx := context.Background()

	t.Run("empty history should not compact", func(t *testing.T) {
		history := &gollem.History{
			Version: gollem.HistoryVersion,
		}

		result, err := compactor(ctx, history, mockClient)
		gt.NoError(t, err)
		gt.Equal(t, history, result) // Should return same instance when no compaction needed
	})

	t.Run("nil history should return error", func(t *testing.T) {
		_, err := compactor(ctx, nil, mockClient)
		gt.Error(t, err)
	})

	t.Run("should compact when token count exceeds limit", func(t *testing.T) {
		// Create compactor with very low token limit to force compaction
		forceCompactor := gollem.NewHistoryCompactor(mockClient,
			gollem.WithMaxTokens(50),
			gollem.WithPreserveRecentTokens(20))

		// Create messages with content so they have non-zero token counts
		messages := make([]gollem.Message, 10)
		for i := range messages {
			textContent, _ := gollem.NewTextContent("This is a test message with content")
			messages[i] = gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{textContent},
			}
		}

		history := &gollem.History{
			LLType:   gollem.LLMTypeOpenAI,
			Version:  gollem.HistoryVersion,
			Messages: messages,
		}

		result, err := forceCompactor(ctx, history, mockClient)
		gt.NoError(t, err)
		gt.True(t, result != history) // Should return compacted version
	})

	t.Run("should not compact when under limits", func(t *testing.T) {
		history := &gollem.History{
			LLType:   gollem.LLMTypeOpenAI,
			Version:  gollem.HistoryVersion,
			Messages: make([]gollem.Message, 2), // Only 2 messages
		}

		result, err := compactor(ctx, history, mockClient)
		gt.NoError(t, err)
		gt.Equal(t, history, result) // Should return same instance when no compaction needed
	})

	t.Run("small history should not compact", func(t *testing.T) {
		textContent, _ := gollem.NewTextContent("Hello")
		history := &gollem.History{
			LLType:  gollem.LLMTypeOpenAI,
			Version: gollem.HistoryVersion,
			Messages: []gollem.Message{
				{Role: gollem.RoleUser, Contents: []gollem.MessageContent{textContent}},
			},
		}
		// Create a new compactor with high threshold to avoid compaction
		noCompactor := gollem.NewHistoryCompactor(mockClient, gollem.WithMaxTokens(10000))

		result, err := noCompactor(ctx, history, mockClient)
		gt.NoError(t, err)
		gt.Equal(t, history, result) // Should return same instance
	})

	t.Run("compaction needed returns compacted history", func(t *testing.T) {
		// Create many messages to exceed the token limit
		messages := make([]gollem.Message, 20)
		for i := range messages {
			textContent, _ := gollem.NewTextContent("This is a long test message with lots of content to ensure we exceed the token limit for compaction")
			role := gollem.RoleUser
			if i%2 == 1 {
				role = gollem.RoleAssistant
			}
			messages[i] = gollem.Message{
				Role:     role,
				Contents: []gollem.MessageContent{textContent},
			}
		}

		history := &gollem.History{
			LLType:   gollem.LLMTypeOpenAI,
			Version:  gollem.HistoryVersion,
			Messages: messages,
		}
		// Create a new compactor with low threshold to force compaction
		forceCompactor := gollem.NewHistoryCompactor(mockClient,
			gollem.WithMaxTokens(50),
			gollem.WithPreserveRecentTokens(10))

		result, err := forceCompactor(ctx, history, mockClient)
		gt.NoError(t, err)
		gt.True(t, result != history) // Should return compacted version
	})
}

func TestNewHistoryCompactor_DefaultBehavior(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{"Summary of the conversation"},
	}
	compactor := gollem.NewHistoryCompactor(mockClient)
	ctx := context.Background()

	// Test with small number of messages (should not compact with default 50k token limit)
	smallMessages := make([]gollem.Message, 10)
	for i := range smallMessages {
		textContent, _ := gollem.NewTextContent("Small message")
		smallMessages[i] = gollem.Message{
			Role:     gollem.RoleUser,
			Contents: []gollem.MessageContent{textContent},
		}
	}
	history := &gollem.History{
		LLType:   gollem.LLMTypeOpenAI,
		Version:  gollem.HistoryVersion,
		Messages: smallMessages,
	}
	result, err := compactor(ctx, history, mockClient)
	gt.NoError(t, err)
	gt.Equal(t, history, result) // Should not compact

	// Test with many messages (should compact with mock token counting)
	largeMessages := make([]gollem.Message, 6000)
	for i := range largeMessages {
		textContent, _ := gollem.NewTextContent("This is a message with content to ensure proper token counting")
		largeMessages[i] = gollem.Message{
			Role:     gollem.RoleUser,
			Contents: []gollem.MessageContent{textContent},
		}
	}
	history = &gollem.History{
		LLType:   gollem.LLMTypeOpenAI,
		Version:  gollem.HistoryVersion,
		Messages: largeMessages,
	}
	result, err = compactor(ctx, history, mockClient)
	gt.NoError(t, err)
	gt.True(t, result != history) // Should compact
}

func TestHistory_CompactionFields(t *testing.T) {
	t.Run("should support compaction metadata", func(t *testing.T) {
		history := &gollem.History{
			LLType:      gollem.LLMTypeOpenAI,
			Version:     gollem.HistoryVersion,
			Summary:     "Test summary",
			Compacted:   true,
			OriginalLen: 100,
			Messages:    []gollem.Message{},
		}

		gt.Equal(t, "Test summary", history.Summary)
		gt.Equal(t, true, history.Compacted)
		gt.Equal(t, 100, history.OriginalLen)
	})
}

func TestCompactorWithRealClients(t *testing.T) {
	ctx := context.Background()

	t.Run("OpenAI client", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}

		client, err := openai.New(ctx, apiKey)
		gt.NoError(t, err)
		compactor := gollem.NewHistoryCompactor(client)

		messages := make([]gollem.Message, 5)
		for i := range messages {
			textContent, _ := gollem.NewTextContent("Test message")
			messages[i] = gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{textContent},
			}
		}

		history := &gollem.History{
			LLType:   gollem.LLMTypeOpenAI,
			Version:  gollem.HistoryVersion,
			Messages: messages,
		}

		// Should not compact small history with default settings
		result, err := compactor(ctx, history, client)
		gt.NoError(t, err)
		gt.Equal(t, history, result)
	})

	t.Run("Claude client", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}

		client, err := claude.New(ctx, apiKey)
		gt.NoError(t, err)
		compactor := gollem.NewHistoryCompactor(client)

		messages := make([]gollem.Message, 5)
		for i := range messages {
			textContent, _ := gollem.NewTextContent("Test message")
			messages[i] = gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{textContent},
			}
		}

		history := &gollem.History{
			LLType:   gollem.LLMTypeClaude,
			Version:  gollem.HistoryVersion,
			Messages: messages,
		}

		// Should not compact small history with default settings
		result, err := compactor(ctx, history, client)
		gt.NoError(t, err)
		gt.Equal(t, history, result)
	})

	t.Run("Gemini client", func(t *testing.T) {
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location := os.Getenv("TEST_GCP_LOCATION")
		if location == "" {
			location = "us-central1"
		}

		client, err := gemini.New(ctx, projectID, location)
		gt.NoError(t, err)
		compactor := gollem.NewHistoryCompactor(client)

		messages := make([]gollem.Message, 5)
		for i := range messages {
			textContent, _ := gollem.NewTextContent("Test message")
			messages[i] = gollem.Message{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{textContent},
			}
		}

		history := &gollem.History{
			LLType:   gollem.LLMTypeGemini,
			Version:  gollem.HistoryVersion,
			Messages: messages,
		}

		// Should not compact small history with default settings
		result, err := compactor(ctx, history, client)
		gt.NoError(t, err)
		gt.Equal(t, history, result)
	})
}

func TestBuildCompactedHistory(t *testing.T) {
	t.Run("builds compacted history with summary", func(t *testing.T) {
		// Create original history
		textContent1, _ := gollem.NewTextContent("Message 1")
		textContent2, _ := gollem.NewTextContent("Message 2")
		original := &gollem.History{
			LLType:  gollem.LLMTypeOpenAI,
			Version: gollem.HistoryVersion,
			Messages: []gollem.Message{
				{Role: gollem.RoleUser, Contents: []gollem.MessageContent{textContent1}},
				{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{textContent2}},
			},
		}

		// Create recent history
		textContent3, _ := gollem.NewTextContent("Recent message")
		recent := &gollem.History{
			LLType:  gollem.LLMTypeOpenAI,
			Version: gollem.HistoryVersion,
			Messages: []gollem.Message{
				{Role: gollem.RoleUser, Contents: []gollem.MessageContent{textContent3}},
			},
		}

		// Build compacted history
		summary := "This is a summary of the conversation"
		compacted := gollem.BuildCompactedHistory(original, summary, recent)

		gt.NotNil(t, compacted)
		gt.Equal(t, summary, compacted.Summary)
		gt.Equal(t, gollem.LLMTypeOpenAI, compacted.LLType)
		gt.Equal(t, gollem.HistoryVersion, compacted.Version)

		// Should have system message with summary + recent messages
		gt.True(t, len(compacted.Messages) >= len(recent.Messages))
	})

	t.Run("handles nil recent history", func(t *testing.T) {
		original := &gollem.History{
			LLType:   gollem.LLMTypeOpenAI,
			Version:  gollem.HistoryVersion,
			Messages: []gollem.Message{},
		}

		summary := "Summary"
		compacted := gollem.BuildCompactedHistory(original, summary, nil)

		gt.NotNil(t, compacted)
		gt.Equal(t, summary, compacted.Summary)
	})
}

func TestMessagesToTemplateMessages(t *testing.T) {
	t.Run("converts messages to template format", func(t *testing.T) {
		// Create test messages
		textContent, _ := gollem.NewTextContent("Hello")
		toolCallContent, _ := gollem.NewToolCallContent("call-1", "get_weather", map[string]interface{}{"location": "Tokyo"})
		toolResponseContent, _ := gollem.NewToolResponseContent("call-1", "get_weather", map[string]interface{}{"temperature": 25}, false)

		messages := []gollem.Message{
			{Role: gollem.RoleSystem, Contents: []gollem.MessageContent{textContent}}, // Should be skipped
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{textContent}},
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{toolCallContent}},
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{toolResponseContent}},
		}

		result := gollem.MessagesToTemplateMessages(messages)

		// System message should be skipped
		gt.Equal(t, 3, len(result))

		// Check first user message
		gt.Equal(t, "user", result[0].Role)
		gt.Equal(t, "Hello", result[0].Content)

		// Check assistant message with tool call
		gt.Equal(t, "assistant", result[1].Role)
		gt.Equal(t, 1, len(result[1].ToolCalls))
		gt.Equal(t, "get_weather", result[1].ToolCalls[0].Name)

		// Check user message with tool response
		gt.Equal(t, "user", result[2].Role)
		gt.Equal(t, 1, len(result[2].ToolResponses))
		gt.Equal(t, "get_weather", result[2].ToolResponses[0].Name)
	})
}
