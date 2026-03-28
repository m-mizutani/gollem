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
// Unlike [Query], it reuses the provided session so the conversation
// history built by prior Generate calls is preserved.
// The response schema is derived from T and passed as a per-call
// [GenerateOption], leaving the session's default config unchanged.
// If JSON unmarshalling or schema validation fails, it retries up to
// maxRetry times (default 3), feeding back the error for correction.
//
// Example:
//
//	type Answer struct {
//	    Name string `json:"name"`
//	}
//	// session already has conversation context from earlier Generate calls
//	resp, err := gollem.SessionQuery[Answer](ctx, session, "What is my name?")
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
