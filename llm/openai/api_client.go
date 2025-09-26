package openai

import (
	"context"

	"github.com/sashabaranov/go-openai"
)

// apiClient is the interface for OpenAI API calls (unexported for encapsulation)
type apiClient interface {
	CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error)
	CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error)
}

// realAPIClient wraps the actual OpenAI client
type realAPIClient struct {
	client *openai.Client
}

func (r *realAPIClient) CreateChatCompletion(ctx context.Context, req openai.ChatCompletionRequest) (openai.ChatCompletionResponse, error) {
	return r.client.CreateChatCompletion(ctx, req)
}

func (r *realAPIClient) CreateChatCompletionStream(ctx context.Context, req openai.ChatCompletionRequest) (*openai.ChatCompletionStream, error) {
	return r.client.CreateChatCompletionStream(ctx, req)
}
