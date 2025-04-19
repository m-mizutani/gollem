package servant

import (
	"context"
)

// Servant is core structure of the package.
type Servant struct {
	llm          LLMClient
	tools        []Tool
	toolSets     []ToolSet
	mcpClients   []MCPClient
	msgCallback  func(ctx context.Context, msg string) error
	toolCallback func(ctx context.Context, tool Tool) error
	errCallback  func(ctx context.Context, err error, tool Tool) error
}

func New(llmClient LLMClient, options ...Option) *Servant {
	s := &Servant{
		llm:   llmClient,
		tools: make([]Tool, 0),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

type Option func(*Servant)

func WithTools(tools ...Tool) Option {
	return func(s *Servant) {
		s.tools = append(s.tools, tools...)
	}
}

func WithToolSets(toolSets ...ToolSet) Option {
	return func(s *Servant) {
		s.toolSets = append(s.toolSets, toolSets...)
	}
}

func WithMCPClients(mcpClients ...MCPClient) Option {
	return func(s *Servant) {
		s.mcpClients = append(s.mcpClients, mcpClients...)
	}
}

// WithMsgCallback sets a callback function for the message. The callback function is called when receiving a generated text message from the LLM.
// Usage:
//
//	servant.WithMsgCallback(func(ctx context.Context, msg string) error {
//		println(msg)
//		return nil
//	})
func WithMsgCallback(callback func(ctx context.Context, msg string) error) Option {
	return func(s *Servant) {
		s.msgCallback = callback
	}
}

// WithToolCallback sets a callback function for the tool. The callback function is called when receiving a tool call from the LLM.
// Usage:
//
//	servant.WithToolCallback(func(ctx context.Context, tool servant.Tool) error {
//		println("running tool: " + tool.Spec().Name)
//		return nil
//	})
func WithToolCallback(callback func(ctx context.Context, tool Tool) error, call FunctionCall) Option {
	return func(s *Servant) {
		s.toolCallback = callback
	}
}

// WithErrCallback sets a callback function for the error of the tool execution. If the callback returns an error (can be same as argument of the callback), the tool execution is aborted. If the callback returns nil, the tool execution is continued.
// Usage:
//
//	servant.WithErrCallback(func(ctx context.Context, err error, tool servant.Tool) error {
//		if errors.Is(err, someErrorYouKnow) {
//			return err // Abort the tool execution
//		}
//		return nil // Continue the tool execution
//	})
func WithErrCallback(callback func(ctx context.Context, err error, tool Tool) error) Option {
	return func(s *Servant) {
		s.errCallback = callback
	}
}

func (s *Servant) Order(ctx context.Context, prompt string) error {
	return nil
}
