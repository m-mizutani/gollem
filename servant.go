package servant

import (
	"context"

	"github.com/m-mizutani/servant/llm"
)

// Servant is core structure of the package.
type Servant struct {
	llm          llm.LLMClient
	tools        []llm.Tool
	mcpClients   []llm.MCPClient
	msgCallback  func(ctx context.Context, msg string) error
	toolCallback func(ctx context.Context, tool llm.Tool) error
}

func New(llmClient llm.LLMClient, options ...Option) *Servant {
	s := &Servant{
		llm:   llmClient,
		tools: make([]llm.Tool, 0),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

type Option func(*Servant)

func WithTools(tools ...llm.Tool) Option {
	return func(s *Servant) {
		s.tools = append(s.tools, tools...)
	}
}

func WithMCPClients(mcpClients ...llm.MCPClient) Option {
	return func(s *Servant) {
		s.mcpClients = append(s.mcpClients, mcpClients...)
	}
}

func (s *Servant) Order(ctx context.Context, prompt string) error {
	return nil
}
