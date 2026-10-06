package gollem

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/m-mizutani/goerr/v2"
)

const defaultMaxRetry = 3

// continuePrompt is sent after a response that ended without an answer.
const continuePrompt = "Your previous response ended without an answer. Respond now with valid JSON matching the schema."

// isNaturalFinishReason reports whether a Response.FinishReason means that the
// model ended the generation on its own, as opposed to a token limit, a
// refusal or a tool call.
func isNaturalFinishReason(reason string) bool {
	switch reason {
	case "end_turn", // Claude
		"STOP",      // Gemini
		"stop",      // OpenAI Chat Completions and Ollama
		"completed": // OpenAI Responses API
		return true
	}
	return false
}

// QueryResponse holds the result of a Query call.
type QueryResponse[T any] struct {
	Data        *T
	InputToken  int
	OutputToken int
}

// QueryOption configures a Query call.
type QueryOption func(*queryConfig)

type queryConfig struct {
	systemPrompt string
	history      *History
	maxRetry     int // default: 3
}

// WithQuerySystemPrompt sets the system prompt for the query.
func WithQuerySystemPrompt(prompt string) QueryOption {
	return func(cfg *queryConfig) {
		cfg.systemPrompt = prompt
	}
}

// WithQueryHistory sets the conversation history for the query.
func WithQueryHistory(history *History) QueryOption {
	return func(cfg *queryConfig) {
		cfg.history = history
	}
}

// WithQueryMaxRetry sets the maximum number of retries when JSON unmarshal
// fails or the response has no text. Default is 3.
func WithQueryMaxRetry(n int) QueryOption {
	return func(cfg *queryConfig) {
		cfg.maxRetry = n
	}
}

// Query executes a one-shot LLM query and returns structured data parsed into type T.
// It generates a JSON schema from T, creates a session with JSON content type,
// calls the LLM, and unmarshals the response into T.
// If JSON unmarshalling fails, it retries up to maxRetry times (default 3),
// feeding back the error to the LLM for correction. A response that has no
// text although the model ended the generation on its own (for example a
// response with only a thinking block) is answered with a message asking the
// model to continue, within the same number of retries.
func Query[T any](ctx context.Context, client LLMClient, prompt string, opts ...QueryOption) (*QueryResponse[T], error) {
	cfg := &queryConfig{
		maxRetry: defaultMaxRetry,
	}
	for _, opt := range opts {
		opt(cfg)
	}

	schema, err := ToSchema(*new(T))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate schema from type parameter")
	}

	sessionOpts := []SessionOption{
		WithSessionContentType(ContentTypeJSON),
		WithSessionResponseSchema(schema),
	}
	if cfg.systemPrompt != "" {
		sessionOpts = append(sessionOpts, WithSessionSystemPrompt(cfg.systemPrompt))
	}
	if cfg.history != nil {
		sessionOpts = append(sessionOpts, WithSessionHistory(cfg.history))
	}

	session, err := client.NewSession(ctx, sessionOpts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create session for query")
	}
	if session == nil {
		return nil, goerr.New("LLMClient.NewSession returned nil session")
	}

	if cfg.maxRetry < 0 {
		cfg.maxRetry = 0
	}

	input := []Input{Text(prompt)}

	return queryWithRetry[T](ctx, session, input, cfg.maxRetry)
}

// queryWithRetry is the shared retry loop for Query and SessionQuery.
// It calls session.Generate with the given input and options, unmarshals
// the response into T, validates against schema, and retries on failure.
func queryWithRetry[T any](ctx context.Context, session Session, input []Input, maxRetry int, genOpts ...GenerateOption) (*QueryResponse[T], error) {
	schema, err := ToSchema(*new(T))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate schema from type parameter")
	}

	var totalInputToken, totalOutputToken int

	for attempt := range maxRetry + 1 {
		resp, err := session.Generate(ctx, input, genOpts...)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to generate content",
				goerr.V("attempt", attempt+1),
			)
		}

		totalInputToken += resp.InputToken
		totalOutputToken += resp.OutputToken

		// Retrying with a JSON correction prompt does not address a refusal,
		// and the text of a refused response is not the requested JSON.
		if resp.Refusal != nil {
			return nil, goerr.Wrap(ErrProhibitedContent, "the provider refused the query",
				goerr.V("attempt", attempt+1),
				goerr.V("finish_reason", resp.FinishReason),
				goerr.V("refusal_reason", resp.Refusal.Reason),
				goerr.V("refusal_categories", resp.Refusal.Categories),
				goerr.V("refusal_explanation", resp.Refusal.Explanation),
			)
		}

		if len(resp.Texts) == 0 {
			// The model ended its turn without an answer, for example with only a
			// thinking block. Resending the same request is answered the same way,
			// so a new user message asks it to continue, as the Claude
			// documentation recommends for an empty end_turn response. A response
			// with function calls is excluded because the calls must be answered
			// with their results before any other user message.
			if isNaturalFinishReason(resp.FinishReason) && len(resp.FunctionCalls) == 0 && attempt < maxRetry {
				input = []Input{Text(continuePrompt)}
				continue
			}
			return nil, goerr.New("no text in response",
				goerr.V("attempt", attempt+1),
				goerr.V("finish_reason", resp.FinishReason),
				goerr.V("other_block_types", resp.OtherBlockTypes),
				goerr.V("output_tokens", resp.OutputToken),
				goerr.V("function_calls", len(resp.FunctionCalls)),
			)
		}

		jsonText := strings.Join(resp.Texts, "")

		var result T
		if unmarshalErr := json.Unmarshal([]byte(jsonText), &result); unmarshalErr != nil {
			if attempt < maxRetry {
				input = []Input{
					Text(fmt.Sprintf(
						"Your previous response was not valid JSON that matches the schema. Error: %s\nYour response was: %s\nPlease respond with valid JSON matching the schema.",
						unmarshalErr.Error(), jsonText,
					)),
				}
				continue
			}
			return nil, goerr.Wrap(unmarshalErr, "failed to unmarshal response JSON after retries",
				goerr.V("attempts", maxRetry+1),
				goerr.V("response", jsonText),
			)
		}

		// Validate the response against the schema
		var raw any
		if unmarshalErr := json.Unmarshal([]byte(jsonText), &raw); unmarshalErr != nil {
			return nil, goerr.Wrap(unmarshalErr, "failed to unmarshal response for validation",
				goerr.V("response", jsonText),
			)
		}
		if validateErr := schema.ValidateValue("root", raw); validateErr != nil {
			if attempt < maxRetry {
				input = []Input{
					Text(fmt.Sprintf(
						"Your previous response was valid JSON but did not match the schema constraints. Error: %s\nYour response was: %s\nPlease respond with valid JSON matching the schema.",
						validateErr.Error(), jsonText,
					)),
				}
				continue
			}
			return nil, goerr.Wrap(validateErr, "response JSON failed schema validation after retries",
				goerr.V("attempts", maxRetry+1),
				goerr.V("response", jsonText),
			)
		}

		return &QueryResponse[T]{
			Data:        &result,
			InputToken:  totalInputToken,
			OutputToken: totalOutputToken,
		}, nil
	}

	// unreachable, but satisfy the compiler
	return nil, goerr.New("unexpected: retry loop completed without result")
}
