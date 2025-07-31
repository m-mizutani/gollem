package gollem

import "context"

// LLMClient is a client for each LLM service.
type LLMClient interface {
	NewSession(ctx context.Context, options ...SessionOption) (Session, error)
	GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error)
	CountTokens(ctx context.Context, history *History) (int, error)
	IsCompatibleHistory(ctx context.Context, history *History) error
}

type FunctionCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// Response is a general response type for each gollem.
type Response struct {
	Texts         []string
	FunctionCalls []*FunctionCall
	InputToken    int
	OutputToken   int

	// Error is an error that occurred during the generation for streaming response.
	Error error
}

func (r *Response) HasData() bool {
	return len(r.Texts) > 0 || len(r.FunctionCalls) > 0 || r.Error != nil
}

type Input interface {
	isInput() restrictedValue
}

type restrictedValue struct{}

// Text is a text input as prompt.
// Usage:
// input := gollem.Text("Hello, world!")
type Text string

func (t Text) isInput() restrictedValue {
	return restrictedValue{}
}

// FunctionResponse is a function response.
// Usage:
//
//	input := gollem.FunctionResponse{
//		Name:      "function_name",
//		Arguments: map[string]any{"key": "value"},
//	}
type FunctionResponse struct {
	ID    string
	Name  string
	Data  map[string]any
	Error error
}

func (f FunctionResponse) isInput() restrictedValue {
	return restrictedValue{}
}
