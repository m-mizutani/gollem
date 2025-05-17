package gemini

import (
	"context"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/goerr/v2"
)

const (
	DefaultEmbeddingModel = "textembedding-gecko@latest"
)

// WithEmbeddingModel sets the model to use for embeddings.
// Default: "textembedding-gecko@latest"
func WithEmbeddingModel(modelName string) Option {
	return func(c *Client) {
		c.embeddingModel = modelName
	}
}

// GenerateEmbedding generates embeddings for the given input text.
func (c *Client) GenerateEmbedding(ctx context.Context, input string) ([]float64, error) {
	model := c.client.GenerativeModel(c.embeddingModel)
	resp, err := model.GenerateContent(ctx, genai.Text(input))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create embedding")
	}

	if len(resp.Candidates) == 0 || len(resp.Candidates[0].Content.Parts) == 0 {
		return nil, goerr.New("no embedding data returned")
	}

	// Convert []float32 to []float64
	embedding := make([]float64, len(resp.Candidates[0].Content.Parts[0].(genai.Text)))
	for i, v := range resp.Candidates[0].Content.Parts[0].(genai.Text) {
		embedding[i] = float64(v)
	}

	return embedding, nil
}
