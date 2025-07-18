package gollem_test

import (
	"context"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
	"github.com/sashabaranov/go-openai"
)

func TestDefaultHistoryCompressor_PerformCompress_ShouldCompressLogic(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{"Summary of the conversation"},
	}
	compressor := gollem.DefaultHistoryCompressor(mockClient)
	ctx := context.Background()

	t.Run("empty history should not compress", func(t *testing.T) {
		history := &gollem.History{}
		options := gollem.DefaultHistoryCompressionOptions()

		result, err := compressor(ctx, history, mockClient, options)
		gt.NoError(t, err)
		gt.Equal(t, history, result) // Should return same instance when no compression needed
	})

	t.Run("nil history should return error", func(t *testing.T) {
		options := gollem.DefaultHistoryCompressionOptions()

		_, err := compressor(ctx, nil, mockClient, options)
		gt.Error(t, err)
	})

	t.Run("should compress when message count exceeds limit", func(t *testing.T) {
		history := &gollem.History{
			LLType: gollem.LlmTypeOpenAI,
			OpenAI: make([]openai.ChatCompletionMessage, 60), // 60 > default 50
		}
		options := gollem.DefaultHistoryCompressionOptions()

		result, err := compressor(ctx, history, mockClient, options)
		gt.NoError(t, err)
		gt.True(t, result != history) // Should return compressed version
	})

	t.Run("should not compress when under limits", func(t *testing.T) {
		history := &gollem.History{
			LLType: gollem.LlmTypeOpenAI,
			OpenAI: make([]openai.ChatCompletionMessage, 10), // 10 < default 50
		}
		options := gollem.DefaultHistoryCompressionOptions()

		result, err := compressor(ctx, history, mockClient, options)
		gt.NoError(t, err)
		gt.Equal(t, history, result) // Should return same instance when no compression needed
	})
}

func TestDefaultHistoryCompressor_PerformCompress(t *testing.T) {
	mockClient := &mockLLMClient{
		responses: []string{"Summary of the conversation"},
	}
	compressor := gollem.DefaultHistoryCompressor(mockClient)
	ctx := context.Background()

	t.Run("no compression needed returns original history", func(t *testing.T) {
		history := &gollem.History{
			LLType: gollem.LlmTypeOpenAI,
			OpenAI: []openai.ChatCompletionMessage{
				{Role: "user", Content: "Hello"},
			},
		}
		options := gollem.DefaultHistoryCompressionOptions()
		options.MaxMessages = 10 // High threshold to avoid compression

		result, err := compressor(ctx, history, mockClient, options)
		gt.NoError(t, err)
		gt.Equal(t, history, result) // Should return same instance
	})

	t.Run("compression needed returns compressed history", func(t *testing.T) {
		history := &gollem.History{
			LLType: gollem.LlmTypeOpenAI,
			OpenAI: []openai.ChatCompletionMessage{
				{Role: "user", Content: "Message 1"},
				{Role: "assistant", Content: "Response 1"},
				{Role: "user", Content: "Message 2"},
				{Role: "assistant", Content: "Response 2"},
				{Role: "user", Content: "Message 3"},
			},
		}
		options := gollem.DefaultHistoryCompressionOptions()
		options.MaxMessages = 3    // Force compression
		options.PreserveRecent = 2 // Preserve only 2 recent messages

		result, err := compressor(ctx, history, mockClient, options)
		gt.NoError(t, err)
		gt.True(t, result != history)                     // Should return different instance
		gt.True(t, result.ToCount() <= history.ToCount()) // Should be smaller or equal
	})

	t.Run("nil history returns error", func(t *testing.T) {
		options := gollem.DefaultHistoryCompressionOptions()

		_, err := compressor(ctx, nil, mockClient, options)
		gt.True(t, err != nil)
	})
}

// TestDefaultHistoryCompressor_EstimateTokens removed - token estimation is now internal

func TestHistoryCompressionOptions_DefaultValues(t *testing.T) {
	options := gollem.DefaultHistoryCompressionOptions()

	gt.Equal(t, 50, options.MaxMessages)
	gt.Equal(t, 10, options.PreserveRecent)
}

func TestHistory_CompressionFields(t *testing.T) {
	t.Run("should support compression metadata", func(t *testing.T) {
		history := &gollem.History{
			LLType:      gollem.LlmTypeOpenAI,
			Summary:     "Test summary",
			Compressed:  true,
			OriginalLen: 100,
		}

		gt.Equal(t, gollem.LlmTypeOpenAI, history.LLType)
		gt.Equal(t, "Test summary", history.Summary)
		gt.True(t, history.Compressed)
		gt.Equal(t, 100, history.OriginalLen)
	})

	t.Run("should clone compression fields", func(t *testing.T) {
		original := &gollem.History{
			LLType:      gollem.LlmTypeOpenAI,
			Summary:     "Original summary",
			Compressed:  true,
			OriginalLen: 50,
		}

		cloned := original.Clone()
		gt.Equal(t, original.Summary, cloned.Summary)
		gt.Equal(t, original.Compressed, cloned.Compressed)
		gt.Equal(t, original.OriginalLen, cloned.OriginalLen)

		// Verify they are independent
		cloned.Summary = "Modified summary"
		gt.NotEqual(t, original.Summary, cloned.Summary)
	})
}

// Mock LLM client for testing
type mockLLMClient struct {
	responses []string
	index     int
}

func (m *mockLLMClient) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	return &mockSession{
		client:  m,
		history: &gollem.History{LLType: gollem.LlmTypeOpenAI},
	}, nil
}

func (m *mockLLMClient) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return nil, nil
}

func (m *mockLLMClient) CountTokens(ctx context.Context, history *gollem.History) (int, error) {
	if history == nil {
		return 0, nil
	}

	// Simple mock implementation - count messages * 10 as rough token estimate
	count := history.ToCount()
	return count * 10, nil
}

type mockSession struct {
	client  *mockLLMClient
	history *gollem.History
}

func (m *mockSession) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	response := "Mock response"
	if m.client.index < len(m.client.responses) {
		response = m.client.responses[m.client.index]
		m.client.index++
	}

	// Add messages to history for testing
	for _, inp := range input {
		if textInput, ok := inp.(gollem.Text); ok {
			m.history.OpenAI = append(m.history.OpenAI, openai.ChatCompletionMessage{
				Role:    "user",
				Content: string(textInput),
			})
		}
	}

	m.history.OpenAI = append(m.history.OpenAI, openai.ChatCompletionMessage{
		Role:    "assistant",
		Content: response,
	})

	return &gollem.Response{
		Texts: []string{response},
	}, nil
}

func (m *mockSession) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	ch := make(chan *gollem.Response, 1)
	response, err := m.GenerateContent(ctx, input...)
	if err != nil {
		close(ch)
		return ch, err
	}
	ch <- response
	close(ch)
	return ch, nil
}

func (m *mockSession) History() *gollem.History {
	return m.history
}

// Test CountTokens implementations for different LLM clients
func TestLLMClient_CountTokens(t *testing.T) {
	ctx := context.Background()

	t.Run("MockClient CountTokens", func(t *testing.T) {
		mockClient := &mockLLMClient{}

		// Test with nil history
		tokens, err := mockClient.CountTokens(ctx, nil)
		gt.NoError(t, err)
		gt.Equal(t, 0, tokens)

		// Test with OpenAI history
		history := &gollem.History{
			LLType: gollem.LlmTypeOpenAI,
			OpenAI: []openai.ChatCompletionMessage{
				{Role: "user", Content: "Hello"},
				{Role: "assistant", Content: "Hi there"},
			},
		}

		tokens, err = mockClient.CountTokens(ctx, history)
		gt.NoError(t, err)
		gt.Equal(t, 20, tokens) // 2 messages * 10
	})
}

// TestHistoryCompressor_WithAccurateTokenCounting removed - token estimation is now internal
