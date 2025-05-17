package openai

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// GenerateEmbedding generates embeddings for the given input text.
func (c *Client) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	/*
			AdaEmbeddingV2  EmbeddingModel = "text-embedding-ada-002"
		SmallEmbedding3 EmbeddingModel = "text-embedding-3-small"
		LargeEmbedding3 EmbeddingModel = "text-embedding-3-large"
	*/
	modelMap := map[string]openai.EmbeddingModel{
		"text-embedding-ada-002": openai.AdaEmbeddingV2,
		"text-embedding-3-small": openai.SmallEmbedding3,
		"text-embedding-3-large": openai.LargeEmbedding3,
	}

	model, ok := modelMap[c.embeddingModel]
	if !ok {
		return nil, goerr.New("invalid or unsupported embedding model. See https://platform.openai.com/docs/guides/embeddings#embedding-models", goerr.V("model", c.embeddingModel))
	}

	req := openai.EmbeddingRequest{
		Input:      input,
		Model:      model,
		Dimensions: dimension,
	}

	resp, err := c.client.CreateEmbeddings(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create embedding")
	}

	if len(resp.Data) == 0 {
		return nil, goerr.New("no embedding data returned")
	}

	embeddings := make([][]float64, len(resp.Data))
	for i, data := range resp.Data {
		embeddings[i] = make([]float64, len(data.Embedding))
		for j, v := range data.Embedding {
			embeddings[i][j] = float64(v)
		}
	}

	return embeddings, nil
}
