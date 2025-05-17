package openai

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// GenerateEmbedding generates embeddings for the given input text.
func (c *Client) GenerateEmbedding(ctx context.Context, input string) ([]float64, error) {
	req := openai.EmbeddingRequest{
		Input: input,
		Model: openai.AdaEmbeddingV2,
	}

	resp, err := c.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create embedding")
	}

	if len(resp.Data) == 0 {
		return nil, goerr.New("no embedding data returned")
	}

	// Convert []float32 to []float64
	embedding := make([]float64, len(resp.Data[0].Embedding))
	for i, v := range resp.Data[0].Embedding {
		embedding[i] = float64(v)
	}

	return embedding, nil
}
