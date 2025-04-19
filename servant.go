package servantic

import (
	"context"
)

// servantic is core structure of the package.
type servantic struct {
	llm          LLMClient
	tools        []Tool
	toolSets     []ToolSet
	mcpClients   []MCPClient
	msgCallback  func(ctx context.Context, msg string) error
	toolCallback func(ctx context.Context, tool Tool) error
	errCallback  func(ctx context.Context, err error, tool Tool) error
}

func New(llmClient LLMClient, options ...Option) *servantic {
	s := &servantic{
		llm:   llmClient,
		tools: make([]Tool, 0),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

type Option func(*servantic)

func WithTools(tools ...Tool) Option {
	return func(s *servantic) {
		s.tools = append(s.tools, tools...)
	}
}

func WithToolSets(toolSets ...ToolSet) Option {
	return func(s *servantic) {
		s.toolSets = append(s.toolSets, toolSets...)
	}
}

func WithMCPClients(mcpClients ...MCPClient) Option {
	return func(s *servantic) {
		s.mcpClients = append(s.mcpClients, mcpClients...)
	}
}

// WithMsgCallback sets a callback function for the message. The callback function is called when receiving a generated text message from the LLM.
// Usage:
//
//	servantic.WithMsgCallback(func(ctx context.Context, msg string) error {
//		println(msg)
//		return nil
//	})
func WithMsgCallback(callback func(ctx context.Context, msg string) error) Option {
	return func(s *servantic) {
		s.msgCallback = callback
	}
}

// WithToolCallback sets a callback function for the tool. The callback function is called when receiving a tool call from the LLM.
// Usage:
//
//	servantic.WithToolCallback(func(ctx context.Context, tool servantic.Tool) error {
//		println("running tool: " + tool.Spec().Name)
//		return nil
//	})
func WithToolCallback(callback func(ctx context.Context, tool Tool) error, call FunctionCall) Option {
	return func(s *servantic) {
		s.toolCallback = callback
	}
}

// WithErrCallback sets a callback function for the error of the tool execution. If the callback returns an error (can be same as argument of the callback), the tool execution is aborted. If the callback returns nil, the tool execution is continued.
// Usage:
//
//	servantic.WithErrCallback(func(ctx context.Context, err error, tool servantic.Tool) error {
//		if errors.Is(err, someErrorYouKnow) {
//			return err // Abort the tool execution
//		}
//		return nil // Continue the tool execution
//	})
func WithErrCallback(callback func(ctx context.Context, err error, tool Tool) error) Option {
	return func(s *servantic) {
		s.errCallback = callback
	}
}

func (s *servantic) Order(ctx context.Context, prompt string) error {
	return nil
}
