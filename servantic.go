package servantic

import (
	"context"

	"github.com/m-mizutani/goerr/v2"
)

// Servantic is core structure of the package.
type Servantic struct {
	llm LLMClient

	loopLimit  int
	retryLimit int

	tools      []Tool
	toolSets   []ToolSet
	mcpClients []MCPClient

	msgCallback  MsgCallback
	toolCallback ToolCallback
	errCallback  ErrCallback
}

const (
	DefaultLoopLimit  = 32
	DefaultRetryLimit = 8
)

// New creates a new servantic instance.
func New(llmClient LLMClient, options ...Option) *Servantic {
	s := &Servantic{
		llm:          llmClient,
		loopLimit:    DefaultLoopLimit,
		retryLimit:   DefaultRetryLimit,
		msgCallback:  defaultMsgCallback,
		toolCallback: defaultToolCallback,
		errCallback:  defaultErrCallback,
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

// Option is the type for the options of the servantic instance.
type Option func(*Servantic)

// WithLoopLimit sets the maximum number of loops for the servantic session iteration (ask LLM and execute tools is one loop).
func WithLoopLimit(loopLimit int) Option {
	return func(s *Servantic) {
		s.loopLimit = loopLimit
	}
}

// WithRetryLimit sets the maximum number of retries for the servantic session. This is counted for error response from Tool. When reaching the limit, the session is finished immediately.
func WithRetryLimit(retryLimit int) Option {
	return func(s *Servantic) {
		s.retryLimit = retryLimit
	}
}

// WithTools sets the tools for the servantic instance.
func WithTools(tools ...Tool) Option {
	return func(s *Servantic) {
		s.tools = append(s.tools, tools...)
	}
}

// WithToolSets sets the tool sets for the servantic instance.
func WithToolSets(toolSets ...ToolSet) Option {
	return func(s *Servantic) {
		s.toolSets = append(s.toolSets, toolSets...)
	}
}

// WithMCPClients sets the MCP clients for the servantic instance.
func WithMCPClients(mcpClients ...MCPClient) Option {
	return func(s *Servantic) {
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
	return func(s *Servantic) {
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
func WithToolCallback(callback func(ctx context.Context, tool FunctionCall) error) Option {
	return func(s *Servantic) {
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
func WithErrCallback(callback func(ctx context.Context, err error, tool FunctionCall) error) Option {
	return func(s *Servantic) {
		s.errCallback = callback
	}
}

// Order is the main function to start the servantic instance.
func (s *Servantic) Order(ctx context.Context, prompt string) error {
	tools := append(s.tools, &exitTool{})
	toolMap, err := buildToolMap(tools, s.toolSets)
	if err != nil {
		return goerr.Wrap(err, "failed to build tool map")
	}

	toolList := make([]Tool, 0, len(toolMap))
	for _, tool := range toolMap {
		toolList = append(toolList, tool)
	}

	ssn, err := s.llm.NewSession(ctx, toolList)
	if err != nil {
		return goerr.Wrap(err, "failed to create LLM session")
	}

	input := []Input{Text(prompt)}
	for i := 0; i < s.loopLimit; i++ {
		resp, err := ssn.Generate(ctx, input...)
		if err != nil {
			return goerr.Wrap(err, "failed to generate response")
		}

		// Check if the exit tool is called at first
		for _, toolCall := range resp.FunctionCalls {
			if toolCall.Name == ExitToolName {
				return nil
			}
		}

		// Call the msgCallback for all texts
		for _, text := range resp.Texts {
			if err := s.msgCallback(ctx, text); err != nil {
				return goerr.Wrap(err, "failed to call msgCallback")
			}
		}

		// Call the toolCallback for all tool calls
		for _, toolCall := range resp.FunctionCalls {
			if err := s.toolCallback(ctx, *toolCall); err != nil {
				return goerr.Wrap(err, "failed to call toolCallback")
			}

			tool, ok := toolMap[toolCall.Name]
			if !ok {
				input = append(input, FunctionResponse{
					ID:    toolCall.ID,
					Error: goerr.New(toolCall.Name+" is not found", goerr.V("call", toolCall)),
				})
				continue
			}

			result, err := tool.Run(ctx, toolCall.Arguments)
			if err != nil {
				if cbErr := s.errCallback(ctx, err, *toolCall); cbErr != nil {
					return goerr.Wrap(cbErr, "failed to call errCallback")
				}

				input = append(input, FunctionResponse{
					ID:    toolCall.ID,
					Error: goerr.Wrap(err, toolCall.Name+" failed to run", goerr.V("call", toolCall)),
				})
				continue
			}

			input = append(input, FunctionResponse{
				ID:   toolCall.ID,
				Data: result,
			})
		}
	}

	return goerr.Wrap(ErrLoopLimitExceeded, "")
}

type toolWrapper struct {
	spec *ToolSpec
	run  func(ctx context.Context, args map[string]any) (map[string]any, error)
}

func (x *toolWrapper) Spec() *ToolSpec {
	return x.spec
}

func (x *toolWrapper) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return x.run(ctx, args)
}

func buildToolMap(tools []Tool, toolSets []ToolSet) (map[string]Tool, error) {
	toolMap := map[string]Tool{}

	for _, tool := range tools {
		if _, ok := toolMap[tool.Spec().Name]; ok {
			return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict", goerr.V("tool_name", tool.Spec().Name))
		}
		toolMap[tool.Spec().Name] = tool
	}

	for _, toolSet := range toolSets {
		for _, spec := range toolSet.Specs() {
			if _, ok := toolMap[spec.Name]; ok {
				return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict", goerr.V("tool_name", spec.Name))
			}
			toolMap[spec.Name] = &toolWrapper{
				spec: spec,
				run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
					return toolSet.Run(ctx, spec.Name, args)
				},
			}
		}
	}

	return toolMap, nil
}

type exitTool struct{}

const (
	ExitToolName = "exit"
)

func (x *exitTool) Spec() *ToolSpec {
	return &ToolSpec{
		Name:        ExitToolName,
		Description: "Exit the session. When calling this tool, the session will be finished immediately and any other tool calls and text generation will be ignored.",
	}
}

func (x *exitTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}
