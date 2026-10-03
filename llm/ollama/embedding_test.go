package ollama_test

import (
	"context"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/llm/ollama"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/gt"
)

func TestGenerateEmbedding(t *testing.T) {
	ctx := context.Background()
	replyEmbeddings := func(embeddings [][]float64) func(w http.ResponseWriter, req recordedRequest) {
		return func(w http.ResponseWriter, req recordedRequest) {
			writeJSON(w, http.StatusOK, map[string]any{
				"model":             "embed-model",
				"embeddings":        embeddings,
				"prompt_eval_count": 6,
			})
		}
	}

	t.Run("request and result", func(t *testing.T) {
		fs := newFakeServer(t, replyEmbeddings([][]float64{{0.1, 0.2}, {0.3, 0.4}}))
		client := newTestClient(t, fs, ollama.WithEmbeddingModel("embed-model"))

		result, err := client.GenerateEmbedding(ctx, 2, []string{"a", "b"})
		gt.NoError(t, err).Required()
		gt.Equal(t, result, [][]float64{{0.1, 0.2}, {0.3, 0.4}})

		req := fs.Last(t)
		gt.Equal(t, req.Path, "/api/embed")
		gt.Equal(t, req.Body["model"], any("embed-model"))
		gt.Equal(t, req.Body["input"], any([]any{"a", "b"}))
		gt.Equal(t, req.Body["dimensions"], any(float64(2)))
	})

	t.Run("dimension 0 is not sent", func(t *testing.T) {
		fs := newFakeServer(t, replyEmbeddings([][]float64{{0.1}}))
		client := newTestClient(t, fs, ollama.WithEmbeddingModel("embed-model"))
		_, err := client.GenerateEmbedding(ctx, 0, []string{"a"})
		gt.NoError(t, err).Required()
		_, hasDimensions := fs.Last(t).Body["dimensions"]
		gt.False(t, hasDimensions)
	})

	type invalidCase struct {
		opts      []ollama.Option
		dimension int
		input     []string
	}
	runInvalid := func(tc invalidCase) func(t *testing.T) {
		return func(t *testing.T) {
			fs := newFakeServer(t, replyEmbeddings([][]float64{{0.1}}))
			client := newTestClient(t, fs, tc.opts...)
			_, err := client.GenerateEmbedding(ctx, tc.dimension, tc.input)
			gt.Error(t, err).Is(gollem.ErrInvalidParameter)
			gt.A(t, fs.Requests()).Length(0)
		}
	}
	withModel := []ollama.Option{ollama.WithEmbeddingModel("embed-model")}
	t.Run("embedding model not set", runInvalid(invalidCase{input: []string{"a"}}))
	t.Run("empty input", runInvalid(invalidCase{opts: withModel}))
	t.Run("negative dimension", runInvalid(invalidCase{opts: withModel, dimension: -1, input: []string{"a"}}))

	t.Run("count mismatch", func(t *testing.T) {
		fs := newFakeServer(t, replyEmbeddings([][]float64{{0.1}}))
		client := newTestClient(t, fs, ollama.WithEmbeddingModel("embed-model"))
		_, err := client.GenerateEmbedding(ctx, 0, []string{"a", "b"})
		gt.Error(t, err).Contains("embedding count does not match input count")
	})

	t.Run("HTTP error", func(t *testing.T) {
		fs := newFakeServer(t, func(w http.ResponseWriter, req recordedRequest) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "model 'embed-model' not found"})
		})
		client := newTestClient(t, fs, ollama.WithEmbeddingModel("embed-model"))
		_, err := client.GenerateEmbedding(ctx, 0, []string{"a"})
		gt.Error(t, err).Contains("status=404")
		gt.Error(t, err).Contains("model 'embed-model' not found")
	})

	t.Run("trace", func(t *testing.T) {
		fs := newFakeServer(t, replyEmbeddings([][]float64{{0.1}}))
		client := newTestClient(t, fs, ollama.WithEmbeddingModel("embed-model"))
		rec := trace.New()
		tctx := trace.WithHandler(rec.StartAgentExecute(ctx), rec)
		_, err := client.GenerateEmbedding(tctx, 0, []string{"a"})
		gt.NoError(t, err).Required()
		rec.EndAgentExecute(tctx, nil)

		spans := llmCallSpans(rec.Trace().RootSpan)
		gt.A(t, spans).Length(1).Required()
		gt.Equal(t, spans[0].LLMCall.Model, "embed-model")
		gt.Equal(t, spans[0].LLMCall.InputTokens, 6)
	})
}

func TestGenerateEmbeddingIntegration(t *testing.T) {
	model, ok := os.LookupEnv("TEST_OLLAMA_EMBEDDING_MODEL")
	if !ok {
		t.Skip("TEST_OLLAMA_EMBEDDING_MODEL is not set")
	}
	opts := []ollama.Option{ollama.WithEmbeddingModel(model)}
	if baseURL, ok := os.LookupEnv("TEST_OLLAMA_BASE_URL"); ok {
		opts = append(opts, ollama.WithBaseURL(baseURL))
	}
	// The chat model is not used by GenerateEmbedding; the embedding model
	// name satisfies New's required argument.
	client, err := ollama.New(context.Background(), model, opts...)
	gt.NoError(t, err).Required()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	result, err := client.GenerateEmbedding(ctx, 0, []string{"hello", "world"})
	gt.NoError(t, err).Required()
	gt.A(t, result).Length(2).Required()
	gt.A(t, result[0]).Longer(0)
}
