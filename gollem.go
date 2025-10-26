package gollem

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
)

// ResponseMode is the type for the response mode of the gollem agent.
type ResponseMode string

const (
	// ResponseModeBlocking is the response mode that blocks the prompt until the LLM generates a response. The agent will wait until all responses are ready.
	ResponseModeBlocking ResponseMode = "blocking"

	// ResponseModeStreaming is the response mode that streams the response from the LLM. The agent receives responses token by token.
	ResponseModeStreaming ResponseMode = "streaming"
)

func (x ResponseMode) String() string {
	return string(x)
}

// Agent is core structure of the package.
// Note: Agent is not thread-safe. Each instance should be used by a single goroutine
// or proper synchronization must be implemented by the caller.
type Agent struct {
	llm LLMClient

	gollemConfig

	// currentSession holds the current session for continuous execution
	// This field should only be accessed through session management methods
	// WARNING: Direct access is not thread-safe
	currentSession Session
}

// Session returns the current session for the agent.
// This is the only way to access the session and its history.
// If no session exists, this will return nil.
func (x *Agent) Session() Session {
	return x.currentSession
}

const (
	DefaultLoopLimit = 128
)

type gollemConfig struct {
	loopLimit    int
	systemPrompt string

	tools    []Tool
	toolSets []ToolSet

	responseMode ResponseMode
	logger       *slog.Logger
	history      *History
	strategy     Strategy

	// Content type and response schema for agent-level configuration
	contentType    ContentType
	responseSchema *Parameter

	// Middleware for content generation
	contentBlockMiddlewares  []ContentBlockMiddleware
	contentStreamMiddlewares []ContentStreamMiddleware

	// Middleware for tool execution
	toolMiddlewares []ToolMiddleware
}

func (c *gollemConfig) Clone() *gollemConfig {
	return &gollemConfig{
		loopLimit:    c.loopLimit,
		systemPrompt: c.systemPrompt,

		tools:    c.tools[:],
		toolSets: c.toolSets[:],

		responseMode: c.responseMode,
		logger:       c.logger,

		history:  c.history,
		strategy: c.strategy,

		contentType:    c.contentType,
		responseSchema: c.responseSchema,

		contentBlockMiddlewares:  c.contentBlockMiddlewares[:],
		contentStreamMiddlewares: c.contentStreamMiddlewares[:],
		toolMiddlewares:          c.toolMiddlewares[:],
	}
}

// New creates a new gollem agent.
func New(llmClient LLMClient, options ...Option) *Agent {
	s := &Agent{
		llm: llmClient,
		gollemConfig: gollemConfig{
			loopLimit:    DefaultLoopLimit,
			systemPrompt: "",

			responseMode: ResponseModeBlocking,
			logger:       slog.New(slog.DiscardHandler),
			strategy:     newDefaultStrategy(),
		},
	}

	for _, opt := range options {
		opt(&s.gollemConfig)
	}

	s.logger.Info("gollem agent created",
		"loop_limit", s.gollemConfig.loopLimit,
		"system_prompt", s.gollemConfig.systemPrompt,
		"tools_count", len(s.gollemConfig.tools),
		"tool_sets_count", len(s.gollemConfig.toolSets),
		"response_mode", s.gollemConfig.responseMode,
		"has_history", s.gollemConfig.history != nil,
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

// WithContentBlockMiddleware adds a content block middleware to the agent.
// The middleware will be applied to all sessions created by this agent.
func WithContentBlockMiddleware(middleware ContentBlockMiddleware) Option {
	return func(s *gollemConfig) {
		s.contentBlockMiddlewares = append(s.contentBlockMiddlewares, middleware)
	}
}

// WithContentStreamMiddleware adds a content stream middleware to the agent.
// The middleware will be applied to all streaming sessions created by this agent.
func WithContentStreamMiddleware(middleware ContentStreamMiddleware) Option {
	return func(s *gollemConfig) {
		s.contentStreamMiddlewares = append(s.contentStreamMiddlewares, middleware)
	}
}

// WithToolMiddleware adds a tool middleware to the agent.
// The middleware will be applied to all tool executions by this agent.
func WithToolMiddleware(middleware ToolMiddleware) Option {
	return func(s *gollemConfig) {
		s.toolMiddlewares = append(s.toolMiddlewares, middleware)
	}
}

// WithStrategy sets the strategy for execution. Default is SimpleLoop.
func WithStrategy(strategy Strategy) Option {
	return func(s *gollemConfig) {
		s.strategy = strategy
	}
}

// WithContentType sets the content type for the agent.
// This will be applied to all sessions created by this agent.
func WithContentType(contentType ContentType) Option {
	return func(s *gollemConfig) {
		s.contentType = contentType
	}
}

// WithResponseSchema sets the response schema for the agent.
// This will be applied to all sessions created by this agent.
// This option should be used with WithContentType(ContentTypeJSON).
func WithResponseSchema(schema *Parameter) Option {
	return func(s *gollemConfig) {
		s.responseSchema = schema
	}
}

func setupTools(ctx context.Context, cfg *gollemConfig) (map[string]Tool, []Tool, error) {
	allTools := cfg.tools[:]

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
	logger := ctxlog.From(ctx)
	logger.Debug("gollem tool list", "names", toolNames)

	return toolMap, toolList, nil
}

// Execute performs the agent task with the given prompt. This method manages the session state internally,
// allowing for continuous conversation without manual history management.
// Returns (*ExecuteResponse, error) where ExecuteResponse contains the final conclusion.
// Use this method instead of Prompt for better agent-like behavior.
func (g *Agent) Execute(ctx context.Context, input ...Input) (*ExecuteResponse, error) {
	cfg := g.gollemConfig.Clone()
	logger := cfg.logger.With("gollem.exec_id", uuid.New().String())
	ctx = ctxlog.With(ctx, logger)

	logger.Debug("[start] gollem execution",
		"input", input,
		"has_existing_session", g.currentSession != nil,
	)
	defer logger.Debug("[end] gollem execution")

	// Initialize strategy
	if err := cfg.strategy.Init(ctx, input); err != nil {
		return nil, goerr.Wrap(err, "failed to initialize strategy")
	}

	// Setup tools for the current execution
	toolMap, toolList, err := setupTools(ctx, cfg)
	if err != nil {
		return nil, err
	}

	// Get strategy-specific tools and merge them
	strategyTools, err := cfg.strategy.Tools(ctx)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get strategy tools")
	}

	// Add strategy tools to the tool list
	for _, tool := range strategyTools {
		if _, ok := toolMap[tool.Spec().Name]; ok {
			return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict with strategy tool", goerr.V("tool_name", tool.Spec().Name))
		}
		toolList = append(toolList, tool)
		toolMap[tool.Spec().Name] = tool
	}

	// If no current session exists, create a new one
	if g.currentSession == nil {
		sessionOptions := []SessionOption{
			WithSessionSystemPrompt(cfg.systemPrompt),
		}

		// Add ContentType if specified
		if cfg.contentType != "" {
			sessionOptions = append(sessionOptions, WithSessionContentType(cfg.contentType))
		}

		// Add ResponseSchema if specified
		if cfg.responseSchema != nil {
			sessionOptions = append(sessionOptions, WithSessionResponseSchema(cfg.responseSchema))
		}

		if cfg.history != nil {
			sessionOptions = append(sessionOptions, WithSessionHistory(cfg.history))
		}
		if len(toolList) > 0 {
			sessionOptions = append(sessionOptions, WithSessionTools(toolList...))
		}

		// Add middleware from agent configuration
		for _, mw := range cfg.contentBlockMiddlewares {
			sessionOptions = append(sessionOptions, WithSessionContentBlockMiddleware(mw))
		}
		for _, mw := range cfg.contentStreamMiddlewares {
			sessionOptions = append(sessionOptions, WithSessionContentStreamMiddleware(mw))
		}

		ssn, err := g.llm.NewSession(ctx, sessionOptions...)
		if err != nil {
			return nil, err
		}
		if ssn == nil {
			return nil, goerr.New("LLMClient.NewSession returned nil session")
		}
		g.currentSession = ssn
	}

	strategy := g.gollemConfig.strategy

	var lastResponse *Response
	nextInput := input
	for i := 0; i < cfg.loopLimit; i++ {
		state := &StrategyState{
			Session:      g.currentSession,
			InitInput:    input,
			LastResponse: lastResponse,
			NextInput:    nextInput,
			Iteration:    i,
			Tools:        toolList,
			SystemPrompt: cfg.systemPrompt,
			History:      cfg.history.Clone(),
		}
		strategyInputs, executeResponse, err := strategy.Handle(ctx, state)
		if err != nil {
			return nil, err
		}

		logger.Debug("gollem input", "input", strategyInputs, "loop", i, "response", executeResponse)

		// ExecuteResponse priority processing
		if executeResponse != nil {
			// Input also specified? Log warning
			if len(strategyInputs) > 0 {
				logger.Warn("Strategy returned both ExecuteResponse and Input - Input will be ignored",
					"inputs_count", len(strategyInputs),
					"texts_count", len(executeResponse.Texts))
			}

			// Append user inputs to session history first
			// This is necessary when strategy returns ExecuteResponse without calling GenerateContent
			if len(executeResponse.UserInputs) > 0 {
				userHistory, err := convertInputsToHistory(executeResponse.UserInputs)
				if err != nil {
					return nil, goerr.Wrap(err, "failed to convert user inputs to history")
				}
				if userHistory != nil {
					if err := g.currentSession.AppendHistory(userHistory); err != nil {
						return nil, goerr.Wrap(err, "failed to append user inputs to session history")
					}
				}
			}

			// Append final response texts to session history as assistant message
			if len(executeResponse.Texts) > 0 {
				// Combine all texts into a single message
				var combinedText string
				for i, text := range executeResponse.Texts {
					if i > 0 {
						combinedText += "\n"
					}
					combinedText += text
				}

				textData, err := json.Marshal(map[string]string{"text": combinedText})
				if err != nil {
					return nil, goerr.Wrap(err, "failed to marshal text content")
				}

				// Create a history entry for the texts (as assistant message)
				textHistory := &History{
					Version: HistoryVersion,
					Messages: []Message{
						{
							Role: RoleAssistant,
							Contents: []MessageContent{
								{
									Type: MessageContentTypeText,
									Data: textData,
								},
							},
						},
					},
				}
				if err := g.currentSession.AppendHistory(textHistory); err != nil {
					return nil, goerr.Wrap(err, "failed to append texts to session history")
				}
			}

			// Return strategy's response immediately
			return executeResponse, nil
		}

		// Input processing
		if len(strategyInputs) == 0 {
			// Both nil: session terminated
			return nil, nil
		}

		switch cfg.responseMode {
		case ResponseModeBlocking:
			output, err := g.currentSession.GenerateContent(ctx, strategyInputs...)
			if err != nil {
				return nil, err
			}

			newInput, err := handleResponse(ctx, output, toolMap, cfg.toolMiddlewares)
			if err != nil {
				return nil, err
			}
			lastResponse = output
			nextInput = newInput

		case ResponseModeStreaming:
			stream, err := g.currentSession.GenerateStream(ctx, strategyInputs...)
			if err != nil {
				return nil, err
			}
			nextInput = []Input{}

			// Accumulate the complete response for lastResponse
			var streamedResponse Response
			for output := range stream {
				logger.Debug("recv response", "output", output)
				newInput, err := handleResponse(ctx, output, toolMap, cfg.toolMiddlewares)
				if err != nil {
					return nil, err
				}
				nextInput = append(nextInput, newInput...)

				// Accumulate streaming response
				streamedResponse.Texts = append(streamedResponse.Texts, output.Texts...)
				streamedResponse.FunctionCalls = append(streamedResponse.FunctionCalls, output.FunctionCalls...)
				streamedResponse.InputToken += output.InputToken
				streamedResponse.OutputToken += output.OutputToken
				if output.Error != nil {
					streamedResponse.Error = output.Error
				}
			}
			lastResponse = &streamedResponse
		}
	}

	return nil, goerr.Wrap(ErrLoopLimitExceeded, "session stopped", goerr.V("loop_limit", cfg.loopLimit))
}

func handleResponse(ctx context.Context, output *Response, toolMap map[string]Tool, toolMiddlewares []ToolMiddleware) ([]Input, error) {
	logger := ctxlog.From(ctx)

	newInput := make([]Input, 0)

	logger.Debug("[start] handling response", "function_calls", output.FunctionCalls)
	defer logger.Debug("[exit] handling response")

	// Call the ToolRequestHook for all tool calls
	for _, toolCall := range output.FunctionCalls {
		logger = logger.With("call", toolCall)

		tool, ok := toolMap[toolCall.Name]
		if !ok {
			logger.Info("gollem tool not found")
			newInput = append(newInput, FunctionResponse{
				Name:  toolCall.Name,
				ID:    toolCall.ID,
				Error: goerr.New(toolCall.Name+" is not found", goerr.V("call", toolCall)),
			})
			continue
		}

		// Create base tool handler
		baseHandler := func(ctx context.Context, req *ToolExecRequest) (*ToolExecResponse, error) {
			start := time.Now()
			result, err := tool.Run(ctx, req.Tool.Arguments)
			duration := time.Since(start).Milliseconds()

			return &ToolExecResponse{
				Result:   result,
				Error:    err,
				Duration: duration,
			}, nil
		}

		// Build middleware chain
		handler := buildToolChain(toolMiddlewares, baseHandler)

		// Execute tool with middleware
		toolSpec := tool.Spec()
		req := &ToolExecRequest{
			Tool:     toolCall,
			ToolSpec: &toolSpec,
		}

		resp, err := handler(ctx, req)
		if err != nil {
			logger.Info("gollem tool handler error", "error", err)
			newInput = append(newInput, FunctionResponse{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Error: goerr.With(err, goerr.V("call", toolCall)),
			})
			continue
		}

		result := resp.Result
		if resp.Error != nil {
			logger.Info("gollem tool error", "error", resp.Error)
			newInput = append(newInput, FunctionResponse{
				ID:    toolCall.ID,
				Name:  toolCall.Name,
				Error: goerr.With(resp.Error, goerr.V("call", toolCall)),
			})
			continue
		}

		logger.Debug("gollem tool result", "tool", toolCall.Name, "result", result, "duration_ms", resp.Duration)

		logger.Debug("gollem tool response", "call", toolCall, "result", result, "should_exit", err)

		// Sanitize result to ensure a generic JSON-compatible structure for LLM processing.
		if result != nil {
			marshaled, err := json.Marshal(result)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal result", goerr.V("result", result))
			}
			var unmarshaled map[string]any
			if err := json.Unmarshal(marshaled, &unmarshaled); err != nil {
				return nil, goerr.Wrap(err, "failed to unmarshal result", goerr.V("marshaled", string(marshaled)))
			}
			result = unmarshaled
		}

		newInput = append(newInput, FunctionResponse{
			ID:   toolCall.ID,
			Name: toolCall.Name,
			Data: result,
		})
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
				return nil, goerr.Wrap(ErrToolNameConflict, "tool name conflict (builtin tool sets)", goerr.V("tool_name", spec.Name))
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

// convertInputsToHistory converts a slice of Input to History with user role
func convertInputsToHistory(inputs []Input) (*History, error) {
	if len(inputs) == 0 {
		return nil, nil
	}

	var contents []MessageContent

	for _, input := range inputs {
		switch v := input.(type) {
		case Text:
			textData, err := json.Marshal(map[string]string{"text": string(v)})
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal text content")
			}
			contents = append(contents, MessageContent{
				Type: MessageContentTypeText,
				Data: textData,
			})

		case Image:
			imageData, err := json.Marshal(map[string]string{
				"data":      v.Base64(),
				"mime_type": v.MimeType(),
			})
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal image content")
			}
			contents = append(contents, MessageContent{
				Type: MessageContentTypeImage,
				Data: imageData,
			})

		case FunctionResponse:
			// FunctionResponse is not user input, skip it
			// It should be handled separately in the normal flow
			continue

		default:
			return nil, goerr.New("unsupported input type for user history")
		}
	}

	if len(contents) == 0 {
		return nil, nil
	}

	return &History{
		Version: HistoryVersion,
		Messages: []Message{
			{
				Role:     RoleUser,
				Contents: contents,
			},
		},
	}, nil
}
