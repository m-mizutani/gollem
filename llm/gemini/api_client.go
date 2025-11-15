package gemini

import (
	"context"

	genai "google.golang.org/genai"
)

// apiClient is the interface for Gemini API calls (unexported for encapsulation)
// This interface provides stateless API calls without chat session dependency
type apiClient interface {
	// GenerateContent generates content without maintaining chat state
	GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	// GenerateContentStream generates content stream without maintaining chat state
	GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) <-chan StreamResponse
	// CountTokens counts the number of tokens in the given contents
	CountTokens(ctx context.Context, model string, contents []*genai.Content, config *genai.CountTokensConfig) (*genai.CountTokensResponse, error)
}

// StreamResponse wraps the response and error from streaming
type StreamResponse struct {
	Resp *genai.GenerateContentResponse
	Err  error
}

// realAPIClient wraps the actual Gemini client for stateless operations
type realAPIClient struct {
	client *genai.Client
}

func (r *realAPIClient) GenerateContent(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	return r.client.Models.GenerateContent(ctx, model, contents, config)
}

func (r *realAPIClient) GenerateContentStream(ctx context.Context, model string, contents []*genai.Content, config *genai.GenerateContentConfig) <-chan StreamResponse {
	ch := make(chan StreamResponse)
	go func() {
		defer close(ch)
		for resp, err := range r.client.Models.GenerateContentStream(ctx, model, contents, config) {
			ch <- StreamResponse{Resp: resp, Err: err}
		}
	}()
	return ch
}

func (r *realAPIClient) CountTokens(ctx context.Context, model string, contents []*genai.Content, config *genai.CountTokensConfig) (*genai.CountTokensResponse, error) {
	return r.client.Models.CountTokens(ctx, model, contents, config)
}
