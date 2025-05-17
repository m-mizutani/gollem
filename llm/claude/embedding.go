package claude

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/m-mizutani/goerr/v2"
)

const (
	embeddingEndpoint = "https://api.anthropic.com/v1/embeddings"
)

type embeddingRequest struct {
	Model string `json:"model"`
	Input string `json:"input"`
}

type embeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
}

// GenerateEmbedding generates embeddings for the given input text.
func (c *Client) GenerateEmbedding(ctx context.Context, input string) ([]float64, error) {
	reqBody := embeddingRequest{
		Model: c.embeddingModel,
		Input: input,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to marshal request body")
	}

	req, err := http.NewRequestWithContext(ctx, "POST", embeddingEndpoint, bytes.NewBuffer(jsonData))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create request")
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to send request")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, goerr.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	var embeddingResp embeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&embeddingResp); err != nil {
		return nil, goerr.Wrap(err, "failed to decode response")
	}

	if len(embeddingResp.Data) == 0 || len(embeddingResp.Data[0].Embedding) == 0 {
		return nil, goerr.New("no embedding data returned")
	}

	// Convert []float32 to []float64
	embedding := make([]float64, len(embeddingResp.Data[0].Embedding))
	for i, v := range embeddingResp.Data[0].Embedding {
		embedding[i] = float64(v)
	}

	return embedding, nil
}
