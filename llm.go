package servantic

import "context"

type Session interface {
	Generate(ctx context.Context, input ...Input) (*Response, error)
}

// LLMClient is a client for each LLM service.
type LLMClient interface {
	NewSession(ctx context.Context, tools []Tool) (Session, error)
}

type MCPClient interface {
	ListTools() ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (map[string]any, error)
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
	restricted() inputRestricted
}

type inputRestricted struct{}

// Text is a text input as prompt.
// Usage:
// input := servantic.Text("Hello, world!")
type Text string

func (t Text) restricted() inputRestricted {
	return inputRestricted{}
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

func (f FunctionResponse) restricted() inputRestricted {
	return inputRestricted{}
}
