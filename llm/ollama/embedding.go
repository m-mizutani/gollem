package ollama

import (
	"context"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/goerr/v2"
)

// GenerateEmbedding returns one embedding vector per input, in input order,
// using the model set by WithEmbeddingModel. A dimension of 0 returns vectors
// of the model's own size; a positive dimension asks the server to truncate
// and renormalize them to that size.
func (c *Client) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	if c.embeddingModel == "" {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "embedding model is not set; use WithEmbeddingModel")
	}
	if len(input) == 0 {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "input must not be empty")
	}
	if dimension < 0 {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "dimension must not be negative",
			goerr.V("dimension", dimension))
	}

	var traceData *trace.LLMCallData
	var llmErr error
	if h := trace.HandlerFrom(ctx); h != nil {
		ctx = h.StartLLMCall(ctx)
		defer func() { h.EndLLMCall(ctx, traceData, llmErr) }()
	}

	resp, err := c.api.embed(ctx, &embedRequest{
		Model:      c.embeddingModel,
		Input:      input,
		Dimensions: dimension,
	})
	if err != nil {
		llmErr = err
		return nil, goerr.Wrap(err, "failed to call Ollama embed API", goerr.V("model", c.embeddingModel))
	}

	if len(resp.Embeddings) != len(input) {
		llmErr = goerr.New("embedding count does not match input count",
			goerr.V("expected", len(input)), goerr.V("actual", len(resp.Embeddings)))
		return nil, llmErr
	}

	traceData = &trace.LLMCallData{
		InputTokens: resp.PromptEvalCount,
		Model:       resp.Model,
		Request:     &trace.LLMRequest{},
		Response:    &trace.LLMResponse{},
	}

	return resp.Embeddings, nil
}
