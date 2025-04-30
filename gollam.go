package gollam

import (
	"context"
	"errors"
	"log/slog"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
)

type ResponseMode int

const (
	ResponseModeBlocking ResponseMode = iota
	ResponseModeStreaming
)

func (x ResponseMode) String() string {
	return []string{"blocking", "streaming"}[x]
}

const (
	DefaultInitPrompt = `You are a helpful assistant. Call "exit" tool when you determine that the task for user prompt is completed.`
)

// Agent is core structure of the package.
type Agent struct {
	llm LLMClient

	gollamConfig
}

const (
	DefaultLoopLimit  = 32
	DefaultRetryLimit = 8
)

type gollamConfig struct {
	loopLimit    int
	retryLimit   int
	initPrompt   string
	systemPrompt string

	tools    []Tool
	toolSets []ToolSet

	msgCallback  MsgCallback
	toolCallback ToolCallback
	errCallback  ErrCallback
	responseMode ResponseMode
	logger       *slog.Logger

	history *History
}

// New creates a new gollam instance.
func New(llmClient LLMClient, options ...Option) *Agent {
	s := &Agent{
		llm: llmClient,
		gollamConfig: gollamConfig{
			loopLimit:  DefaultLoopLimit,
			retryLimit: DefaultRetryLimit,
			initPrompt: DefaultInitPrompt,

			msgCallback:  defaultMsgCallback,
			toolCallback: defaultToolCallback,
			errCallback:  defaultErrCallback,
			responseMode: ResponseModeBlocking,
			logger:       slog.New(slog.DiscardHandler),
		},
	}

	for _, opt := range options {
		opt(&s.gollamConfig)
	}

	return s
}

// Option is the type for the options of the gollam instance.
type Option func(*gollamConfig)

// WithLoopLimit sets the maximum number of loops for the gollam session iteration (ask LLM and execute tools is one loop).
func WithLoopLimit(loopLimit int) Option {
	return func(s *gollamConfig) {
		s.loopLimit = loopLimit
	}
}

// WithRetryLimit sets the maximum number of retries for the gollam session. This is counted for error response from Tool. When reaching the limit, the session is finished immediately.
func WithRetryLimit(retryLimit int) Option {
	return func(s *gollamConfig) {
		s.retryLimit = retryLimit
	}
}

// WithInitPrompt sets the initial prompt for the gollam instance.
func WithInitPrompt(initPrompt string) Option {
	return func(s *gollamConfig) {
		s.initPrompt = initPrompt
	}
}

// WithTools sets the tools for the gollam instance.
func WithTools(tools ...Tool) Option {
	return func(s *gollamConfig) {
		s.tools = append(s.tools, tools...)
	}
}

// WithToolSets sets the tool sets for the gollam instance.
func WithToolSets(toolSets ...ToolSet) Option {
	return func(s *gollamConfig) {
		s.toolSets = append(s.toolSets, toolSets...)
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
	return func(s *gollamConfig) {
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
	return func(s *gollamConfig) {
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
	return func(s *gollamConfig) {
		s.errCallback = callback
	}
}

// WithResponseMode sets the response mode for the gollam instance. Default is ResponseModeBlocking.
func WithResponseMode(responseMode ResponseMode) Option {
	return func(s *gollamConfig) {
		s.responseMode = responseMode
	}
}

// WithLogger sets the logger for the gollam instance. Default is discard logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *gollamConfig) {
		s.logger = logger
	}
}

// WithHistory sets the history for the gollam instance.
func WithHistory(history *History) Option {
	return func(s *gollamConfig) {
		s.history = history
	}
}

// Instruct is the main function to start the gollam instance. In the first loop, the LLM generates a response with the prompt. After that, the LLM generates a response with the tool call and tool call arguments. The call loop continues until the exit tool is called or the LoopLimit is reached.
func (g *Agent) Prompt(ctx context.Context, prompt string, options ...Option) (*History, error) {
	cfg := g.gollamConfig
	for _, opt := range options {
		opt(&cfg)
	}

	orderID := uuid.New().String()
	logger := cfg.logger.With("gollam.order_id", orderID)
	ctx = ctxWithLogger(ctx, logger)
	logger.Info("start order", "prompt", prompt)

	tools := append(cfg.tools, &exitTool{})
	toolMap, err := buildToolMap(ctx, tools, cfg.toolSets)
	if err != nil {
		return nil, err
	}

	toolList := make([]Tool, 0, len(toolMap))
	toolNames := make([]string, 0, len(toolMap))
	for _, tool := range toolMap {
		toolList = append(toolList, tool)
		toolNames = append(toolNames, tool.Spec().Name)
	}
	logger.Debug("tool list", "names", toolNames)

	var sessionOptions []SessionOption
	if cfg.history != nil && cfg.history.history.Messages != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	}
	if cfg.systemPrompt != "" {
		sessionOptions = append(sessionOptions, WithSessionSystemPrompt(cfg.systemPrompt))
	}
	if len(cfg.tools) > 0 {
		sessionOptions = append(sessionOptions, WithSessionTools(toolList...))
	}

	ssn, err := g.llm.NewSession(ctx, sessionOptions...)
	if err != nil {
		return nil, err
	}

	initPrompt := `You are a helpful assistant. When you complete the task, send conclusion and call the exit tool.`
	input := []Input{Text(initPrompt), Text(prompt)}
	exitToolCalled := false

	for i := 0; !exitToolCalled && len(input) > 0; i++ {
		if i > cfg.loopLimit {
			return nil, goerr.Wrap(ErrLoopLimitExceeded, "order stopped", goerr.V("loop_limit", cfg.loopLimit))
		}

		logger.Debug("send request", "count", i, "input", input)

		switch cfg.responseMode {
		case ResponseModeBlocking:
			output, err := ssn.GenerateContent(ctx, input...)
			if err != nil {
				return nil, err
			}

			newInput, err := handleResponse(ctx, cfg, output, toolMap)
			if err != nil {
				if !errors.Is(err, errExitToolCalled) {
					return nil, err
				}
				exitToolCalled = true
			}
			input = newInput

		case ResponseModeStreaming:
			stream, err := ssn.GenerateStream(ctx, input...)
			if err != nil {
				return nil, err
			}
			input = make([]Input, 0)

			for output := range stream {
				logger.Debug("recv response", "output", output)
				newInput, err := handleResponse(ctx, cfg, output, toolMap)
				if err != nil {
					if !errors.Is(err, errExitToolCalled) {
						return nil, err
					}
					exitToolCalled = true
				}
				input = append(input, newInput...)
			}
		}
	}

	return ssn.History(), nil
}

var errExitToolCalled = goerr.New("exit tool called")

func handleResponse(ctx context.Context, cfg gollamConfig, output *Response, toolMap map[string]Tool) ([]Input, error) {
	newInput := make([]Input, 0)
	// Call the msgCallback for all texts
	for _, text := range output.Texts {
		if err := cfg.msgCallback(ctx, text); err != nil {
			return nil, goerr.Wrap(err, "failed to call msgCallback")
		}
	}

	var retErr error

	// Call the toolCallback for all tool calls
	for _, toolCall := range output.FunctionCalls {
		if toolCall.Name == ExitToolName {
			retErr = errExitToolCalled
			continue
		}

		if err := cfg.toolCallback(ctx, *toolCall); err != nil {
			return nil, goerr.Wrap(err, "failed to call toolCallback")
		}

		tool, ok := toolMap[toolCall.Name]
		if !ok {
			newInput = append(newInput, FunctionResponse{
				Name:  toolCall.Name,
				ID:    toolCall.ID,
				Error: goerr.New(toolCall.Name+" is not found", goerr.V("call", toolCall)),
			})
			continue
		}

		result, err := tool.Run(ctx, toolCall.Arguments)
		if err != nil {
			if cbErr := cfg.errCallback(ctx, err, *toolCall); cbErr != nil {
				return nil, goerr.Wrap(cbErr, "failed to call errCallback")
			}

			newInput = append(newInput, FunctionResponse{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Error: goerr.Wrap(err, toolCall.Name+" failed to run", goerr.V("call", toolCall)),
			})
			continue
		}

		newInput = append(newInput, FunctionResponse{
			ID:   toolCall.ID,
			Name: toolCall.Name,
			Data: result,
		})
	}

	return newInput, retErr
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

func buildToolMap(ctx context.Context, tools []Tool, toolSets []ToolSet) (map[string]Tool, error) {
	toolMap := map[string]Tool{}

	for _, tool := range tools {
		if _, ok := toolMap[tool.Spec().Name]; ok {
			return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict (builtin tools)", goerr.V("tool_name", tool.Spec().Name))
		}
		toolMap[tool.Spec().Name] = tool
	}

	for _, toolSet := range toolSets {
		specs, err := toolSet.Specs(ctx)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to get tool set specs")
		}

		for _, spec := range specs {
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
