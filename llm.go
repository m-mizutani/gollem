package servantic

import "context"

type Session interface {
	Generate(ctx context.Context, input ...Input) (*Response, error)
}

// LLMClient is a client for each LLM service.
type LLMClient interface {
	NewSession(ctx context.Context, tools []Tool) (Session, error)
}

type FunctionCall struct {
	ID        string
	Name      string
	Arguments map[string]any
}

// Response is a general response type for each servantic.
type Response struct {
	Texts         []string
	FunctionCalls []*FunctionCall
}

type Input interface {
	isInput() restrictedValue
}

type restrictedValue struct{}

// Text is a text input as prompt.
// Usage:
// input := servantic.Text("Hello, world!")
type Text string

func (t Text) isInput() restrictedValue {
	return restrictedValue{}
}

// FunctionResponse is a function response.
// Usage:
//
//	input := servantic.FunctionResponse{
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
