package claude

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
)

// GenerateEmbedding generates embeddings for the given input text. Claude does not support emmbedding generation directly.
func (c *Client) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	return nil, goerr.New("Claude does not support embedding generation")
}
