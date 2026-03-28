package gollem

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
)

// SessionQueryOption configures a SessionQuery call.
type SessionQueryOption func(*sessionQueryConfig)

type sessionQueryConfig struct {
	maxRetry int
}

// WithSessionQueryMaxRetry sets the maximum number of retries when JSON unmarshal fails. Default is 3.
func WithSessionQueryMaxRetry(n int) SessionQueryOption {
	return func(cfg *sessionQueryConfig) {
		cfg.maxRetry = n
	}
}

// SessionQuery executes a structured query on an existing session.
// Unlike Query, it reuses the provided session and its conversation context,
// applying per-call GenerateOptions to override the session's default response format.
// If JSON unmarshalling fails, it retries up to maxRetry times (default 3),
// feeding back the error to the LLM for correction.
func SessionQuery[T any](ctx context.Context, session Session, prompt string, opts ...SessionQueryOption) (*QueryResponse[T], error) {
	cfg := &sessionQueryConfig{
		maxRetry: defaultMaxRetry,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	if session == nil {
		return nil, goerr.New("session must not be nil")
	}

	schema, err := ToSchema(*new(T))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate schema from type parameter")
	}

	if cfg.maxRetry < 0 {
		cfg.maxRetry = 0
	}

	genOpts := []GenerateOption{
		WithGenerateResponseSchema(schema),
	}

	input := []Input{Text(prompt)}

	return queryWithRetry[T](ctx, session, input, cfg.maxRetry, genOpts...)
}
