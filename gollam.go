package gollem

import (
	"context"
	"log/slog"

	"github.com/google/uuid"
	"github.com/m-mizutani/goerr/v2"
)

// ResponseMode is the type for the response mode of the gollem agent.
type ResponseMode int

const (
	// ResponseModeBlocking is the response mode that blocks the prompt until the LLM generates a response. The agent will wait until all responses are ready.
	ResponseModeBlocking ResponseMode = iota

	// ResponseModeStreaming is the response mode that streams the response from the LLM. The agent receives responses token by token.
	ResponseModeStreaming
)

// String returns the string representation of the response mode.
func (x ResponseMode) String() string {
	return []string{"blocking", "streaming"}[x]
}

// Agent is core structure of the package.
type Agent struct {
	llm LLMClient

	gollemConfig
}

const (
	DefaultLoopLimit  = 32
	DefaultRetryLimit = 8
)

type gollemConfig struct {
	loopLimit    int
	retryLimit   int
	initPrompt   string
	systemPrompt string

	tools    []Tool
	toolSets []ToolSet

	messageHook      MessageHook
	toolRequestHook  ToolRequestHook
	toolResponseHook ToolResponseHook
	toolErrorHook    ToolErrorHook
	responseMode     ResponseMode
	logger           *slog.Logger

	history *History
}

func (c *gollemConfig) Clone() *gollemConfig {
	return &gollemConfig{
		loopLimit:    c.loopLimit,
		retryLimit:   c.retryLimit,
		initPrompt:   c.initPrompt,
		systemPrompt: c.systemPrompt,

		tools:    c.tools[:],
		toolSets: c.toolSets[:],

		messageHook:      c.messageHook,
		toolRequestHook:  c.toolRequestHook,
		toolResponseHook: c.toolResponseHook,
		toolErrorHook:    c.toolErrorHook,
		responseMode:     c.responseMode,
		logger:           c.logger,

		history: c.history,
	}
}

// New creates a new gollem agent.
func New(llmClient LLMClient, options ...Option) *Agent {
	s := &Agent{
		llm: llmClient,
		gollemConfig: gollemConfig{
			loopLimit:    DefaultLoopLimit,
			retryLimit:   DefaultRetryLimit,
			initPrompt:   "",
			systemPrompt: "",

			messageHook:      defaultMessageHook,
			toolRequestHook:  defaultToolRequestHook,
			toolResponseHook: defaultToolResponseHook,
			toolErrorHook:    defaultToolErrorHook,
			responseMode:     ResponseModeBlocking,
			logger:           slog.New(slog.DiscardHandler),
		},
	}

	for _, opt := range options {
		opt(&s.gollemConfig)
	}

	return s
}

// Option is the type for the options of the gollem agent.
type Option func(*gollemConfig)

// WithLoopLimit sets the maximum number of loops for the gollem session iteration (ask LLM and execute tools is one loop).
func WithLoopLimit(loopLimit int) Option {
	return func(s *gollemConfig) {
		s.loopLimit = loopLimit
	}
}

// WithRetryLimit sets the maximum number of retries for the gollem session. This is counted for error response from Tool. When reaching the limit, the session is finished immediately.
func WithRetryLimit(retryLimit int) Option {
	return func(s *gollemConfig) {
		s.retryLimit = retryLimit
	}
}

// WithInitPrompt sets the initial prompt for the gollem agent. The initial prompt is used when there is no history. If you want to use the system prompt, use WithSystemPrompt instead.
func WithInitPrompt(initPrompt string) Option {
	return func(s *gollemConfig) {
		s.initPrompt = initPrompt
	}
}

// WithSystemPrompt sets the system prompt for the gollem agent. Default is no system prompt.
func WithSystemPrompt(systemPrompt string) Option {
	return func(s *gollemConfig) {
		s.systemPrompt = systemPrompt
	}
}

// WithTools sets the tools for the gollem agent.
func WithTools(tools ...Tool) Option {
	return func(s *gollemConfig) {
		s.tools = append(s.tools, tools...)
	}
}

// WithToolSets sets the tool sets for the gollem agent.
func WithToolSets(toolSets ...ToolSet) Option {
	return func(s *gollemConfig) {
		s.toolSets = append(s.toolSets, toolSets...)
	}
}

// WithMessageHook sets a callback function for the message. The callback function is called when receiving a generated text message from the LLM. If the function returns an error, the Prompt() method will be aborted immediately.
// Usage:
//
//	gollem.WithMessageHook(func(ctx context.Context, msg string) error {
//		println(msg)
//		return nil
//	})
func WithMessageHook(callback func(ctx context.Context, msg string) error) Option {
	return func(s *gollemConfig) {
		s.messageHook = callback
	}
}

// WithToolRequestHook sets a callback function for the tool. The callback function is called just before executing the tool. If the function returns an error, the Prompt() method will be aborted immediately.
// Usage:
//
//	gollem.WithToolRequestHook(func(ctx context.Context, tool gollem.Tool) error {
//		println("running tool: " + tool.Spec().Name)
//		return nil
//	})
func WithToolRequestHook(callback func(ctx context.Context, tool FunctionCall) error) Option {
	return func(s *gollemConfig) {
		s.toolRequestHook = callback
	}
}

// WithToolResponseHook sets a callback function for the response of the tool execution. The callback function is called when receiving a response from the tool. If the function returns an error, the Prompt() method will be aborted immediately.
// Usage:
//
//	gollem.WithToolResponseHook(func(ctx context.Context, tool gollem.Tool, response map[string]any) error {
//		println("tool response: " + tool.Spec().Name)
//		return nil
//	})
func WithToolResponseHook(callback func(ctx context.Context, tool FunctionCall, response map[string]any) error) Option {
	return func(s *gollemConfig) {
		s.toolResponseHook = callback
	}
}

// WithToolErrorHook sets a callback function for the error of the tool execution. If you want to stop Prompt(), return the same error as the original error.
// Usage:
//
//	gollem.WithToolErrorHook(func(ctx context.Context, err error, tool gollem.Tool) error {
//		if errors.Is(err, someErrorYouKnow) {
//			return err // Abort the tool execution
//		}
//		return nil // Continue the tool execution
//	})
func WithToolErrorHook(callback func(ctx context.Context, err error, tool FunctionCall) error) Option {
	return func(s *gollemConfig) {
		s.toolErrorHook = callback
	}
}

// WithResponseMode sets the response mode for the gollem agent. Default is ResponseModeBlocking.
func WithResponseMode(responseMode ResponseMode) Option {
	return func(s *gollemConfig) {
		s.responseMode = responseMode
	}
}

// WithLogger sets the logger for the gollem agent. Default is discard logger.
func WithLogger(logger *slog.Logger) Option {
	return func(s *gollemConfig) {
		s.logger = logger
	}
}

// WithHistory sets the history for the gollem agent.
func WithHistory(history *History) Option {
	return func(s *gollemConfig) {
		s.history = history
	}
}

// Prompt is the main function to start the gollem agent. In the first loop, the LLM generates a response with the prompt. After that, the LLM generates a response with the tool call and tool call arguments. The call loop continues until no tool call from LLM or the LoopLimit is reached.
func (g *Agent) Prompt(ctx context.Context, prompt string, options ...Option) (*History, error) {
	cfg := g.gollemConfig.Clone()
	for _, opt := range options {
		opt(cfg)
	}

	orderID := uuid.New().String()
	logger := cfg.logger.With("gollem.order_id", orderID)
	ctx = ctxWithLogger(ctx, logger)
	logger.Info("start order", "prompt", prompt)

	toolMap, err := buildToolMap(ctx, cfg.tools, cfg.toolSets)
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

	input := []Input{Text(prompt)}

	sessionOptions := []SessionOption{
		WithSessionSystemPrompt(cfg.systemPrompt),
	}

	if cfg.history != nil {
		sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
	} else if cfg.initPrompt != "" {
		input = append([]Input{Text(cfg.initPrompt)}, input...)
	}
	if len(toolList) > 0 {
		sessionOptions = append(sessionOptions, WithSessionTools(toolList...))
	}

	ssn, err := g.llm.NewSession(ctx, sessionOptions...)
	if err != nil {
		return nil, err
	}

	for i := 0; len(input) > 0; i++ {
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

			newInput, err := handleResponse(ctx, *cfg, output, toolMap)
			if err != nil {
				return nil, err
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
				newInput, err := handleResponse(ctx, *cfg, output, toolMap)
				if err != nil {
					return nil, err
				}
				input = append(input, newInput...)
			}
		}
	}

	return ssn.History(), nil
}

func handleResponse(ctx context.Context, cfg gollemConfig, output *Response, toolMap map[string]Tool) ([]Input, error) {
	newInput := make([]Input, 0)
	// Call the MessageHook for all texts
	for _, text := range output.Texts {
		if err := cfg.messageHook(ctx, text); err != nil {
			return nil, goerr.Wrap(err, "failed to call MessageHook")
		}
	}

	var retErr error

	// Call the ToolRequestHook for all tool calls
	for _, toolCall := range output.FunctionCalls {
		if err := cfg.toolRequestHook(ctx, *toolCall); err != nil {
			return nil, goerr.Wrap(err, "failed to call ToolRequestHook")
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
			if cbErr := cfg.toolErrorHook(ctx, err, *toolCall); cbErr != nil {
				return nil, goerr.Wrap(cbErr, "failed to call ToolErrorHook")
			}

			newInput = append(newInput, FunctionResponse{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Error: goerr.Wrap(err, toolCall.Name+" failed to run", goerr.V("call", toolCall)),
			})
			continue
		}

		if cbErr := cfg.toolResponseHook(ctx, *toolCall, result); cbErr != nil {
			return nil, goerr.Wrap(cbErr, "failed to call ToolResponseHook")
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
