package gollam

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
)

// Gollam is core structure of the package.
type Gollam struct {
	llm LLMClient

	loopLimit  int
	retryLimit int

	tools      []Tool
	toolSets   []ToolSet
	mcpClients []*MCPClient

	msgCallback  MsgCallback
	toolCallback ToolCallback
	errCallback  ErrCallback

	logger *slog.Logger
}

const (
	DefaultLoopLimit  = 32
	DefaultRetryLimit = 8
)

// New creates a new gollam instance.
func New(llmClient LLMClient, options ...Option) *Gollam {
	s := &Gollam{
		llm:          llmClient,
		loopLimit:    DefaultLoopLimit,
		retryLimit:   DefaultRetryLimit,
		msgCallback:  defaultMsgCallback,
		toolCallback: defaultToolCallback,
		errCallback:  defaultErrCallback,
		logger:       slog.New(slog.DiscardHandler),
	}

	for _, opt := range options {
		opt(s)
	}

	return s
}

// Option is the type for the options of the gollam instance.
type Option func(*Gollam)

// WithLoopLimit sets the maximum number of loops for the gollam session iteration (ask LLM and execute tools is one loop).
func WithLoopLimit(loopLimit int) Option {
	return func(s *Gollam) {
		s.loopLimit = loopLimit
	}
}

// WithRetryLimit sets the maximum number of retries for the gollam session. This is counted for error response from Tool. When reaching the limit, the session is finished immediately.
func WithRetryLimit(retryLimit int) Option {
	return func(s *Gollam) {
		s.retryLimit = retryLimit
	}
}

// WithTools sets the tools for the gollam instance.
func WithTools(tools ...Tool) Option {
	return func(s *Gollam) {
		s.tools = append(s.tools, tools...)
	}
}

// WithToolSets sets the tool sets for the gollam instance.
func WithToolSets(toolSets ...ToolSet) Option {
	return func(s *Gollam) {
		s.toolSets = append(s.toolSets, toolSets...)
	}
}

// WithMCPStdio sets the MCP client for local MCP executable server via stdio.
func WithMCPonStdio(path string, args []string, options ...MCPonStdioOption) Option {
	return func(s *Gollam) {
		c := &MCPClient{
			path: path,
			args: args,
		}
		for _, opt := range options {
			opt(c)
		}
		s.mcpClients = append(s.mcpClients, c)
	}
}

// WithMCPonSSE sets the MCP client for remote MCP server via HTTP SSE.
func WithMCPonSSE(baseURL string, options ...MCPonSSEOption) Option {
	return func(s *Gollam) {
		c := &MCPClient{
			baseURL: baseURL,
		}
		for _, opt := range options {
			opt(c)
		}
		s.mcpClients = append(s.mcpClients, c)
	}
}

// WithMsgCallback sets a callback function for the message. The callback function is called when receiving a generated text message from the LLM.
// Usage:
//
//	gollam.WithMsgCallback(func(ctx context.Context, msg string) error {
//		println(msg)
//		return nil
//	})
func WithMsgCallback(callback func(ctx context.Context, msg string) error) Option {
	return func(s *Gollam) {
		s.msgCallback = callback
	}
}

// WithToolCallback sets a callback function for the tool. The callback function is called just before executing the tool. If you want to abort the tool execution, return an error.
// Usage:
//
//	gollam.WithToolCallback(func(ctx context.Context, tool gollam.Tool) error {
//		println("running tool: " + tool.Spec().Name)
//		return nil
//	})
func WithToolCallback(callback func(ctx context.Context, tool FunctionCall) error) Option {
	return func(s *Gollam) {
		s.toolCallback = callback
	}
}

// WithErrCallback sets a callback function for the error of the tool execution. If the callback returns an error (can be same as argument of the callback), the tool execution is aborted. If the callback returns nil, the tool execution is continued.
// Usage:
//
//	gollam.WithErrCallback(func(ctx context.Context, err error, tool gollam.Tool) error {
//		if errors.Is(err, someErrorYouKnow) {
//			return err // Abort the tool execution
//		}
//		return nil // Continue the tool execution
//	})
func WithErrCallback(callback func(ctx context.Context, err error, tool FunctionCall) error) Option {
	return func(s *Gollam) {
		s.errCallback = callback
	}
}

// WithLogger sets the logger for the gollam instance. Default is discard logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *Gollam) {
		s.logger = logger
	}
}

// Order is the main function to start the gollam instance. In the first loop, the LLM generates a response with the prompt. After that, the LLM generates a response with the tool call and tool call arguments. The call loop continues until the exit tool is called or the LoopLimit is reached.
func (s *Gollam) Order(ctx context.Context, prompt string) error {
	orderID := uuid.New().String()
	logger := s.logger.With("gollam.order_id", orderID)
	ctx = ctxWithLogger(ctx, logger)
	logger.Info("start order", "prompt", prompt)

	tools := append(s.tools, &exitTool{})
	toolMap, err := buildToolMap(ctx, tools, s.toolSets, s.mcpClients)
	if err != nil {
		return err
	}

	toolList := make([]Tool, 0, len(toolMap))
	toolNames := make([]string, 0, len(toolMap))
	for _, tool := range toolMap {
		toolList = append(toolList, tool)
		toolNames = append(toolNames, tool.Spec().Name)
	}
	logger.Debug("tool list", "names", toolNames)

	ssn, err := s.llm.NewSession(ctx, toolList)
	if err != nil {
		return err
	}

	initPrompt := `You are a helpful assistant. When you complete the task, send conclusion and call the exit tool.`
	input := []Input{Text(initPrompt), Text(prompt)}
	exitToolCalled := false

	for i := 0; i < s.loopLimit && !exitToolCalled; i++ {
		logger.Debug("send request", "count", i, "input", input)
		output, err := ssn.Generate(ctx, input...)
		if err != nil {
			return err
		}
		input = nil

		logger.Debug("recv response", "output", output)

		// Call the msgCallback for all texts
		for _, text := range output.Texts {
			if err := s.msgCallback(ctx, text); err != nil {
				return goerr.Wrap(err, "failed to call msgCallback")
			}
		}

		// Call the toolCallback for all tool calls
		for _, toolCall := range output.FunctionCalls {
			if toolCall.Name == ExitToolName {
				exitToolCalled = true
				continue
			}

			if err := s.toolCallback(ctx, *toolCall); err != nil {
				return goerr.Wrap(err, "failed to call toolCallback")
			}

			tool, ok := toolMap[toolCall.Name]
			if !ok {
				input = append(input, FunctionResponse{
					Name:  toolCall.Name,
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
					Name:  toolCall.Name,
					Error: goerr.Wrap(err, toolCall.Name+" failed to run", goerr.V("call", toolCall)),
				})
				continue
			}

			input = append(input, FunctionResponse{
				ID:   toolCall.ID,
				Name: toolCall.Name,
				Data: result,
			})
		}
	}

	if !exitToolCalled {
		return goerr.Wrap(ErrLoopLimitExceeded, "")
	}

	return nil
}

type toolWrapper struct {
	spec ToolSpec
	run  func(ctx context.Context, args map[string]any) (map[string]any, error)
}

func (x *toolWrapper) Spec() ToolSpec {
	return x.spec
}

func (x *toolWrapper) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return x.run(ctx, args)
}

func buildToolMap(ctx context.Context, tools []Tool, toolSets []ToolSet, mcpClients []*MCPClient) (map[string]Tool, error) {
	toolMap := map[string]Tool{}

	for _, mcpClient := range mcpClients {
		if err := mcpClient.start(ctx); err != nil {
			return nil, goerr.Wrap(err, "failed to start MCP client")
		}

		tools, err := mcpClient.listTools(ctx)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to list tools")
		}

		for _, tool := range tools {
			if _, ok := toolMap[tool.Name]; ok {
				return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict (MCP)", goerr.V("tool_name", tool.Name))
			}

			wrappedTool, err := wrapMCPToolCall(mcpClient, tool)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to wrap MCP tool")
			}

			toolMap[tool.Name] = wrappedTool
		}
	}

	for _, tool := range tools {
		if _, ok := toolMap[tool.Spec().Name]; ok {
			return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict (builtin tools)", goerr.V("tool_name", tool.Spec().Name))
		}
		toolMap[tool.Spec().Name] = tool
	}

	for _, toolSet := range toolSets {
		for _, spec := range toolSet.Specs() {
			if _, ok := toolMap[spec.Name]; ok {
				return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict (builtintool sets)", goerr.V("tool_name", spec.Name))
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

func (x *exitTool) Spec() ToolSpec {
	return ToolSpec{
		Name:        ExitToolName,
		Description: "When you determine that the task for user prompt is completed, call this tool.",
	}
}

func (x *exitTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return nil, nil
}
