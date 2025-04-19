package llm

import (
	"context"
)

type Tool interface {
	Name() string
	Description() string
	Parameters() map[string]*Parameter
	Run(args map[string]any) (map[string]any, error)
}

type Session interface {
	Generate(ctx context.Context, input ...Input) (*Response, error)
}

type LLMClient interface {
	NewSession(ctx context.Context, tools []Tool) (Session, error)
}

type MCPClient interface {
	ListTools() ([]Tool, error)
	CallTool(ctx context.Context, name string, args map[string]any) (map[string]any, error)
}
