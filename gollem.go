package gollem

import (
	"context"
	"encoding/json"
	"errors"
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

	// currentSession holds the current session for continuous execution
	currentSession Session
}

func (x *Agent) Facilitator() Facilitator {
	return x.gollemConfig.facilitator
}

// Session returns the current session for the agent.
// This is the only way to access the session and its history.
func (x *Agent) Session() Session {
	return x.currentSession
}

const (
	DefaultLoopLimit  = 32
	DefaultRetryLimit = 8
)

type gollemConfig struct {
	loopLimit    int
	retryLimit   int
	systemPrompt string

	tools    []Tool
	toolSets []ToolSet

	loopHook         LoopHook
	messageHook      MessageHook
	toolRequestHook  ToolRequestHook
	toolResponseHook ToolResponseHook
	toolErrorHook    ToolErrorHook
	responseMode     ResponseMode
	logger           *slog.Logger

	facilitator Facilitator

	history *History
}

func (c *gollemConfig) Clone() *gollemConfig {
	return &gollemConfig{
		loopLimit:    c.loopLimit,
		retryLimit:   c.retryLimit,
		systemPrompt: c.systemPrompt,

		tools:    c.tools[:],
		toolSets: c.toolSets[:],

		loopHook:         c.loopHook,
		messageHook:      c.messageHook,
		toolRequestHook:  c.toolRequestHook,
		toolResponseHook: c.toolResponseHook,
		toolErrorHook:    c.toolErrorHook,
		responseMode:     c.responseMode,
		logger:           c.logger,

		facilitator: c.facilitator,

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
			systemPrompt: "",

			loopHook:         defaultLoopHook,
			messageHook:      defaultMessageHook,
			toolRequestHook:  defaultToolRequestHook,
			toolResponseHook: defaultToolResponseHook,
			toolErrorHook:    defaultToolErrorHook,
			responseMode:     ResponseModeBlocking,
			logger:           slog.New(slog.DiscardHandler),
			facilitator:      newDefaultFacilitator(llmClient),
		},
	}

	for _, opt := range options {
		opt(&s.gollemConfig)
	}

	s.logger.Info("gollem agent created",
		"loop_limit", s.gollemConfig.loopLimit,
		"retry_limit", s.gollemConfig.retryLimit,
		"system_prompt", s.gollemConfig.systemPrompt,
		"tools_count", len(s.gollemConfig.tools),
		"tool_sets_count", len(s.gollemConfig.toolSets),
		"response_mode", s.gollemConfig.responseMode,
		"has_message_hook", s.gollemConfig.messageHook != nil,
		"has_tool_request_hook", s.gollemConfig.toolRequestHook != nil,
		"has_tool_response_hook", s.gollemConfig.toolResponseHook != nil,
		"has_tool_error_hook", s.gollemConfig.toolErrorHook != nil,
		"has_history", s.gollemConfig.history != nil,
		"has_facilitator", s.gollemConfig.facilitator != nil,
	)

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

// WithFacilitator sets the facilitator for the gollem agent. The facilitator is used to control the session loop. If set nil, the session loop will be ended when the LLM generates a response with no tool call.
func WithFacilitator(tool Facilitator) Option {
	return func(s *gollemConfig) {
		s.facilitator = tool
	}
}

// WithLoopHook sets a callback function for the loop. The callback function is called when the loop is started. If the function returns an error, the Prompt() method will be aborted immediately.
// Usage:
//
//	gollem.WithLoopHook(func(ctx context.Context, loop int, input []Input) error {
//		println("loop: " + strconv.Itoa(loop))
//		return nil
//	})
func WithLoopHook(callback func(ctx context.Context, loop int, input []Input) error) Option {
	return func(s *gollemConfig) {
		s.loopHook = callback
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

// WithToolRequestHook sets a callback function that is called just before executing a tool. The callback is invoked even if the requested tool is not found. If the callback returns an error, the Prompt() method will be aborted immediately.
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

func setupTools(ctx context.Context, cfg *gollemConfig) (map[string]Tool, []Tool, error) {
	allTools := cfg.tools[:]
	if cfg.facilitator != nil {
		allTools = append(allTools, cfg.facilitator)
	}
	toolMap, err := buildToolMap(ctx, allTools, cfg.toolSets)
	if err != nil {
		return nil, nil, err
	}

	toolList := make([]Tool, 0, len(toolMap))
	toolNames := make([]string, 0, len(toolMap))
	for _, tool := range toolMap {
		toolList = append(toolList, tool)
		toolNames = append(toolNames, tool.Spec().Name)
	}
	logger := LoggerFromContext(ctx)
	logger.Debug("gollem tool list", "names", toolNames)

	return toolMap, toolList, nil
}

// Execute performs the agent task with the given prompt. This method manages the session state internally,
// allowing for continuous conversation without manual history management.
// Use this method instead of Prompt for better agent-like behavior.
func (g *Agent) Execute(ctx context.Context, prompt string, options ...Option) error {
	cfg := g.gollemConfig.Clone()
	for _, opt := range options {
		opt(cfg)
	}

	logger := cfg.logger.With("gollem.request_id", uuid.New().String())
	ctx = ctxWithLogger(ctx, logger)
	logger.Info("starting gollem execution",
		"prompt", prompt,
		"has_existing_session", g.currentSession != nil,
	)

	// Setup tools for the current execution
	toolMap, toolList, err := setupTools(ctx, cfg)
	if err != nil {
		return err
	}

	// If no current session exists, create a new one
	if g.currentSession == nil {
		sessionOptions := []SessionOption{
			WithSessionSystemPrompt(cfg.systemPrompt),
		}

		if cfg.history != nil {
			sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
		}
		if len(toolList) > 0 {
			sessionOptions = append(sessionOptions, WithSessionTools(toolList...))
		}

		ssn, err := g.llm.NewSession(ctx, sessionOptions...)
		if err != nil {
			return err
		}
		g.currentSession = ssn
	}

	input := []Input{Text(prompt)}

	for i := 0; i < cfg.loopLimit; i++ {
		if err := cfg.loopHook(ctx, i, input); err != nil {
			return err
		}

		if len(input) == 0 {
			if cfg.facilitator == nil {
				// If no facilitator is set, the session is ended when the LLM generates a response with no tool call.
				return nil
			}

			resp, err := cfg.facilitator.Facilitate(ctx, g.currentSession.History())
			if err != nil {
				return err
			}

			if cfg.facilitator.IsCompleted() {
				return nil
			}

			switch resp.Action {
			case ActionComplete:
				return nil

			case ActionContinue:
				if resp.NextStep == "" {
					return goerr.Wrap(ErrExitConversation, "conversation exit by no next step", goerr.V("facilitate", resp))
				}

				input = []Input{Text(resp.NextStep)}

			default:
				return goerr.Wrap(ErrExitConversation, "conversation exit by invalid action", goerr.V("facilitate", resp))
			}
		}

		logger.Debug("gollem input", "input", input, "loop", i)

		switch cfg.responseMode {
		case ResponseModeBlocking:
			output, err := g.currentSession.GenerateContent(ctx, input...)
			if err != nil {
				return err
			}

			newInput, err := handleResponse(ctx, *cfg, output, toolMap)
			if err != nil {
				// Check if the error is ErrExitConversation
				if errors.Is(err, ErrExitConversation) {
					logger.Info("conversation exited by tool")
					return nil
				}
				return err
			}
			input = newInput

		case ResponseModeStreaming:
			stream, err := g.currentSession.GenerateStream(ctx, input...)
			if err != nil {
				return err
			}
			input = make([]Input, 0)

			for output := range stream {
				logger.Debug("recv response", "output", output)
				newInput, err := handleResponse(ctx, *cfg, output, toolMap)
				if err != nil {
					// Check if the error is ErrExitConversation
					if errors.Is(err, ErrExitConversation) {
						logger.Info("conversation exited by tool")
						return nil
					}
					return err
				}
				input = append(input, newInput...)
			}
		}

		if cfg.facilitator != nil {
			if cfg.facilitator.IsCompleted() {
				return nil
			}
		} else {
			if len(input) == 0 {
				return nil
			}
		}
	}

	return goerr.Wrap(ErrLoopLimitExceeded, "session stopped", goerr.V("loop_limit", cfg.loopLimit))
}

func handleResponse(ctx context.Context, cfg gollemConfig, output *Response, toolMap map[string]Tool) ([]Input, error) {
	logger := LoggerFromContext(ctx)

	newInput := make([]Input, 0)
	var exitConversationErr error

	// Call the MessageHook for all texts
	for _, text := range output.Texts {
		if err := cfg.messageHook(ctx, text); err != nil {
			return nil, goerr.Wrap(err, "failed to call MessageHook")
		}
	}

	// Call the ToolRequestHook for all tool calls
	for _, toolCall := range output.FunctionCalls {
		logger.Debug("gollem received tool request", "tool", toolCall.Name, "args", toolCall.Arguments)

		// Check if this tool is a Facilitator by checking the tool name
		isFacilitator := false
		if cfg.facilitator != nil {
			isFacilitator = toolCall.Name == cfg.facilitator.Spec().Name
		}

		// Call the ToolRequestHook only if this is not a Facilitator tool
		if !isFacilitator {
			if err := cfg.toolRequestHook(ctx, *toolCall); err != nil {
				return nil, goerr.Wrap(err, "failed to call ToolRequestHook")
			}
		}

		tool, ok := toolMap[toolCall.Name]
		if !ok {
			logger.Info("gollem tool not found", "call", toolCall)
			newInput = append(newInput, FunctionResponse{
				Name:  toolCall.Name,
				ID:    toolCall.ID,
				Error: goerr.New(toolCall.Name+" is not found", goerr.V("call", toolCall)),
			})
			continue
		}

		result, err := tool.Run(ctx, toolCall.Arguments)
		logger.Debug("gollem tool result", "tool", toolCall.Name, "result", result)
		if err != nil {
			// Check if the error is ErrExitConversation
			if errors.Is(err, ErrExitConversation) {
				// Store the error but continue processing other tools
				exitConversationErr = goerr.Wrap(ErrExitConversation, "conversation exit requested by tool", goerr.V("tool", toolCall.Name))
				logger.Info("tool requested conversation exit, continuing to process remaining tools", "tool", toolCall.Name)
				continue
			}

			if cbErr := cfg.toolErrorHook(ctx, err, *toolCall); cbErr != nil {
				return nil, goerr.Wrap(cbErr, "failed to call ToolErrorHook")
			}

			logger.Info("gollem tool error", "call", toolCall, "error", err)

			newInput = append(newInput, FunctionResponse{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Error: goerr.Wrap(err, toolCall.Name+" failed to run", goerr.V("call", toolCall)),
			})
			continue
		}

		// Call the ToolResponseHook only if this is not a Facilitator tool
		if !isFacilitator {
			if cbErr := cfg.toolResponseHook(ctx, *toolCall, result); cbErr != nil {
				return nil, goerr.Wrap(cbErr, "failed to call ToolResponseHook")
			}
		}

		logger.Info("gollem tool response", "call", toolCall, "result", result)

		// Sanitize result to ensure a generic JSON-compatible structure for LLM processing.
		if result != nil {
			marshaled, err := json.Marshal(result)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal result")
			}
			var unmarshaled map[string]any
			if err := json.Unmarshal(marshaled, &unmarshaled); err != nil {
				return nil, goerr.Wrap(err, "failed to unmarshal result")
			}
			result = unmarshaled
		}

		newInput = append(newInput, FunctionResponse{
			ID:   toolCall.ID,
			Name: toolCall.Name,
			Data: result,
		})
	}

	// Return the exit conversation error after processing all tool calls
	if exitConversationErr != nil {
		return nil, exitConversationErr
	}

	return newInput, nil
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
