package gemini

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"math"
	"strings"
	"time"

	"github.com/m-mizutani/ctxlog"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"google.golang.org/api/option"
	"google.golang.org/genai"
)

const (
	DefaultModel          = "gemini-2.5-flash"
	DefaultEmbeddingModel = "text-embedding-004"
)

var (
	// geminiPromptScope is the logging scope for Gemini prompts
	geminiPromptScope = ctxlog.NewScope("gemini_prompt", ctxlog.EnabledBy("GOLLEM_LOGGING_GEMINI_PROMPT"))

	// geminiResponseScope is the logging scope for Gemini responses
	geminiResponseScope = ctxlog.NewScope("gemini_response", ctxlog.EnabledBy("GOLLEM_LOGGING_GEMINI_RESPONSE"))
)

// Client is a client for the Gemini API.
// It provides methods to interact with Google's Gemini models.
type Client struct {
	projectID string
	location  string

	// client is the underlying Gemini client.
	client *genai.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithModel option.
	defaultModel string

	// embeddingModel is the model to use for embeddings.
	// It can be overridden using WithEmbeddingModel option.
	embeddingModel string

	// gcpOptions are additional options for Google Cloud Platform.
	// They can be set using WithGoogleCloudOptions.
	gcpOptions []option.ClientOption

	// generationConfig contains the default generation parameters
	generationConfig *genai.GenerateContentConfig

	// systemPrompt is the system prompt to use for chat completions.
	systemPrompt string

	// contentType is the type of content to be generated.
	contentType gollem.ContentType
}

// Option is a configuration option for the Gemini client.
type Option func(*Client)

// WithModel sets the model to use for text generation.
// Default: "gemini-2.0-flash"
func WithModel(model string) Option {
	return func(c *Client) {
		c.defaultModel = model
	}
}

// WithEmbeddingModel sets the model to use for embeddings.
// Default: "text-embedding-004"
func WithEmbeddingModel(model string) Option {
	return func(c *Client) {
		c.embeddingModel = model
	}
}

// WithGoogleCloudOptions sets additional Google Cloud options.
// These can include authentication credentials, endpoint overrides, etc.
func WithGoogleCloudOptions(opts ...option.ClientOption) Option {
	return func(c *Client) {
		c.gcpOptions = append(c.gcpOptions, opts...)
	}
}

// WithTemperature sets the temperature parameter for text generation.
// Controls randomness in output generation.
// Range: 0.0 to 2.0
// Default: 1.0
func WithTemperature(temp float32) Option {
	return func(c *Client) {
		if c.generationConfig == nil {
			c.generationConfig = &genai.GenerateContentConfig{}
		}
		c.generationConfig.Temperature = &temp
	}
}

// WithTopP sets the top_p parameter for text generation.
// Controls diversity via nucleus sampling.
// Range: 0.0 to 1.0
// Default: 1.0
func WithTopP(topP float32) Option {
	return func(c *Client) {
		if c.generationConfig == nil {
			c.generationConfig = &genai.GenerateContentConfig{}
		}
		c.generationConfig.TopP = &topP
	}
}

// WithTopK sets the top_k parameter for text generation.
// Controls diversity via top-k sampling.
// Range: 1 to 40
func WithTopK(topK float32) Option {
	return func(c *Client) {
		if c.generationConfig == nil {
			c.generationConfig = &genai.GenerateContentConfig{}
		}
		topKFloat32 := topK
		c.generationConfig.TopK = &topKFloat32
	}
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(maxTokens int32) Option {
	return func(c *Client) {
		if c.generationConfig == nil {
			c.generationConfig = &genai.GenerateContentConfig{}
		}
		c.generationConfig.MaxOutputTokens = maxTokens
	}
}

// WithStopSequences sets the stop sequences for text generation.
func WithStopSequences(stopSequences []string) Option {
	return func(c *Client) {
		if c.generationConfig == nil {
			c.generationConfig = &genai.GenerateContentConfig{}
		}
		c.generationConfig.StopSequences = stopSequences
	}
}

// WithThinkingBudget sets the thinking budget for text generation.
// A value of -1 enables automatic thinking budget allocation.
func WithThinkingBudget(budget int32) Option {
	return func(c *Client) {
		if c.generationConfig == nil {
			c.generationConfig = &genai.GenerateContentConfig{}
		}
		if c.generationConfig.ThinkingConfig == nil {
			c.generationConfig.ThinkingConfig = &genai.ThinkingConfig{}
		}
		c.generationConfig.ThinkingConfig.ThinkingBudget = &budget
	}
}

// WithSystemPrompt sets the system prompt to use for chat completions.
func WithSystemPrompt(prompt string) Option {
	return func(c *Client) {
		c.systemPrompt = prompt
	}
}

// WithContentType sets the content type for text generation.
// This determines the format of the generated content.
func WithContentType(contentType gollem.ContentType) Option {
	return func(c *Client) {
		c.contentType = contentType
	}
}

// New creates a new client for the Gemini API.
// It requires a project ID and location, and can be configured with additional options.
func New(ctx context.Context, projectID, location string, options ...Option) (*Client, error) {
	if projectID == "" {
		return nil, goerr.New("projectID is required")
	}
	if location == "" {
		return nil, goerr.New("location is required")
	}

	var budget int32 = 0

	client := &Client{
		projectID:      projectID,
		location:       location,
		defaultModel:   DefaultModel,
		embeddingModel: DefaultEmbeddingModel,
		contentType:    gollem.ContentTypeText,
		generationConfig: &genai.GenerateContentConfig{
			ThinkingConfig: &genai.ThinkingConfig{
				ThinkingBudget: &budget,
			},
		},
	}

	for _, option := range options {
		option(client)
	}

	// Create client configuration for Vertex AI backend
	config := &genai.ClientConfig{
		Project:  projectID,
		Location: location,
		Backend:  genai.BackendVertexAI,
	}

	newClient, err := genai.NewClient(ctx, config)
	if err != nil {
		return nil, err
	}

	client.client = newClient
	return client, nil
}

// NewSession creates a new session for the Gemini API.
// It converts the provided tools to Gemini's tool format and initializes a new chat session.
func (c *Client) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	cfg := gollem.NewSessionConfig(options...)

	// Prepare generation config
	config := &genai.GenerateContentConfig{}

	// Copy generation config from client
	if c.generationConfig != nil {
		*config = *c.generationConfig
	}

	// Override with session-specific content type
	switch cfg.ContentType() {
	case gollem.ContentTypeJSON:
		config.ResponseMIMEType = "application/json"
	case gollem.ContentTypeText:
		config.ResponseMIMEType = "text/plain"
	}

	// Set system prompt
	systemPrompt := cfg.SystemPrompt()
	if systemPrompt == "" {
		systemPrompt = c.systemPrompt
	}
	if systemPrompt != "" {
		config.SystemInstruction = &genai.Content{
			Role: "system",
			Parts: []*genai.Part{
				{Text: systemPrompt},
			},
		}
	}

	// Convert tools
	if len(cfg.Tools()) > 0 {
		tools := make([]*genai.Tool, 1)
		tools[0] = &genai.Tool{
			FunctionDeclarations: make([]*genai.FunctionDeclaration, len(cfg.Tools())),
		}
		for i, tool := range cfg.Tools() {
			tools[0].FunctionDeclarations[i] = convertToolToNewSDK(tool)
		}
		config.Tools = tools
	}

	// Initialize currentHistory from config or create new
	var currentHistory *gollem.History
	if cfg.History() != nil {
		currentHistory = cfg.History()
	} else {
		currentHistory = &gollem.History{
			LLType:  gollem.LLMTypeGemini,
			Version: gollem.HistoryVersion,
		}
	}

	session := &Session{
		apiClient:      &realAPIClient{client: c.client},
		model:          c.defaultModel,
		config:         config,
		currentHistory: currentHistory,
		cfg:            cfg,
	}

	return session, nil
}

// Session is a session for the Gemini chat.
// It maintains the conversation state and handles message generation.
type Session struct {
	// apiClient is the API client interface for dependency injection
	apiClient apiClient

	// model is the model name to use
	model string

	// config is the generation configuration
	config *genai.GenerateContentConfig

	// currentHistory maintains the gollem.History for middleware access
	currentHistory *gollem.History

	// cfg is the session configuration
	cfg gollem.SessionConfig
}

func (s *Session) History() (*gollem.History, error) {
	return s.currentHistory, nil
}

// convertInputs converts gollem.Input to Gemini parts
func (s *Session) convertInputs(input ...gollem.Input) ([]*genai.Part, error) {
	parts := make([]*genai.Part, 0, len(input))

	for _, in := range input {
		switch v := in.(type) {
		case gollem.Text:
			parts = append(parts, &genai.Part{Text: string(v)})
		case gollem.Image:
			// Check if format is supported by Gemini (no GIF support)
			if v.MimeType() == string(gollem.ImageMimeTypeGIF) {
				return nil, goerr.New("GIF format is not supported by Gemini", goerr.V("mime_type", v.MimeType()))
			}

			parts = append(parts, &genai.Part{
				InlineData: &genai.Blob{
					MIMEType: v.MimeType(),
					Data:     v.Data(),
				},
			})
		case gollem.FunctionResponse:
			if v.Error != nil {
				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name: v.Name,
						Response: map[string]any{
							"error_message": fmt.Sprintf("%+v", v.Error),
						},
					},
				})
			} else {
				parts = append(parts, &genai.Part{
					FunctionResponse: &genai.FunctionResponse{
						Name:     v.Name,
						Response: v.Data,
					},
				})
			}
		default:
			return nil, goerr.Wrap(gollem.ErrInvalidParameter, "invalid input")
		}
	}
	return parts, nil
}

// processResponse converts Gemini response to gollem.Response
func processResponse(resp *genai.GenerateContentResponse) (*gollem.Response, error) {
	if len(resp.Candidates) == 0 {
		return &gollem.Response{}, nil
	}

	response := &gollem.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollem.FunctionCall, 0),
	}

	// Extract token counts from UsageMetadata if available
	if resp.UsageMetadata != nil {
		response.InputToken = int(resp.UsageMetadata.PromptTokenCount)
		response.OutputToken = int(resp.UsageMetadata.CandidatesTokenCount)
	}

	for _, candidate := range resp.Candidates {
		if candidate.FinishReason != "" {
			if strings.Contains(string(candidate.FinishReason), "MALFORMED_FUNCTION_CALL") {
				return nil, goerr.Wrap(gollem.ErrFunctionCallFormat, "malformed function call")
			}
			if strings.Contains(string(candidate.FinishReason), "PROHIBITED_CONTENT") {
				return nil, goerr.Wrap(gollem.ErrProhibitedContent, "prohibited content")
			}
		}

		if candidate.Content == nil {
			continue
		}

		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				response.Texts = append(response.Texts, part.Text)
			}

			if part.FunctionCall != nil {
				fc := &gollem.FunctionCall{
					ID:        fmt.Sprintf("%s_%d", part.FunctionCall.Name, time.Now().UnixNano()),
					Name:      part.FunctionCall.Name,
					Arguments: part.FunctionCall.Args,
				}
				response.FunctionCalls = append(response.FunctionCalls, fc)
			}
		}
	}

	return response, nil
}

// GenerateContent generates content based on the input.
func (s *Session) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	// Build the content request for middleware
	// Create a copy of the current history to avoid middleware side effects
	var historyCopy *gollem.History
	if s.currentHistory != nil {
		historyCopy = &gollem.History{
			Version:  s.currentHistory.Version,
			LLType:   s.currentHistory.LLType,
			Messages: make([]gollem.Message, len(s.currentHistory.Messages)),
		}
		copy(historyCopy.Messages, s.currentHistory.Messages)
	}

	contentReq := &gollem.ContentRequest{
		Inputs:  input,
		History: historyCopy,
	}

	// Create the base handler that performs the actual API call
	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
		// Always update history from middleware (even if same address, content may have changed)
		if req.History != nil {
			s.currentHistory = req.History
		}

		// Build complete content list from history and inputs
		var contents []*genai.Content

		// Add history to contents if available
		if s.currentHistory != nil {
			historyContents, err := s.currentHistory.ToGemini()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert history to Gemini format")
			}
			contents = append(contents, historyContents...)
		}

		// Convert current inputs to parts
		parts, err := s.convertInputs(req.Inputs...)
		if err != nil {
			return nil, err
		}

		// Add current input as a new user message
		if len(parts) > 0 {
			userContent := &genai.Content{
				Role:  "user",
				Parts: parts,
			}
			contents = append(contents, userContent)
		}

		// Log prompt if enabled
		promptLogger := ctxlog.From(ctx, geminiPromptScope)
		if promptLogger.Enabled(ctx, slog.LevelInfo) {
			var messages []map[string]any
			for _, content := range contents {
				for _, part := range content.Parts {
					if part.Text != "" {
						messages = append(messages, map[string]any{
							"role":    content.Role,
							"type":    "text",
							"content": part.Text,
						})
					}
					if part.FunctionResponse != nil {
						messages = append(messages, map[string]any{
							"role":     content.Role,
							"type":     "function_response",
							"name":     part.FunctionResponse.Name,
							"response": part.FunctionResponse.Response,
						})
					}
				}
			}
			systemPrompt := ""
			if s.config != nil && s.config.SystemInstruction != nil && len(s.config.SystemInstruction.Parts) > 0 {
				if part := s.config.SystemInstruction.Parts[0]; part != nil {
					systemPrompt = part.Text
				}
			}
			promptLogger.Info("Gemini prompt",
				"system_prompt", systemPrompt,
				"messages", messages,
			)
		}

		// Call the API
		result, err := s.apiClient.GenerateContent(ctx, s.model, contents, s.config)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to generate content")
		}

		response, err := processResponse(result)
		if err != nil {
			return nil, err
		}

		// Log responses if GOLLEM_LOGGING_GEMINI_RESPONSE is set
		responseLogger := ctxlog.From(ctx, geminiResponseScope)
		if responseLogger.Enabled(ctx, slog.LevelInfo) {
			var logContent []map[string]any
			for _, text := range response.Texts {
				logContent = append(logContent, map[string]any{
					"type": "text",
					"text": text,
				})
			}
			for _, funcCall := range response.FunctionCalls {
				logContent = append(logContent, map[string]any{
					"type":      "function_call",
					"id":        funcCall.ID,
					"name":      funcCall.Name,
					"arguments": funcCall.Arguments,
				})
			}
			var finishReason string
			if len(result.Candidates) > 0 {
				finishReason = string(result.Candidates[0].FinishReason)
			}
			responseLogger.Info("Gemini response",
				"finish_reason", finishReason,
				"usage", map[string]any{
					"prompt_tokens":     response.InputToken,
					"candidates_tokens": response.OutputToken,
				},
				"content", logContent,
			)
		}

		// Update history with the response
		// Create assistant message from response
		assistantParts := make([]*genai.Part, 0)
		if len(response.Texts) > 0 || len(response.FunctionCalls) > 0 {
			for _, text := range response.Texts {
				assistantParts = append(assistantParts, &genai.Part{Text: text})
			}
			for _, fc := range response.FunctionCalls {
				assistantParts = append(assistantParts, &genai.Part{
					FunctionCall: &genai.FunctionCall{
						Name: fc.Name,
						Args: fc.Arguments,
					},
				})
			}

		}

		// Convert only the new messages (input + response) to gollem format and append to existing history
		var newContents []*genai.Content
		// Add current input as a new user message
		if len(parts) > 0 {
			userContent := &genai.Content{
				Role:  "user",
				Parts: parts,
			}
			newContents = append(newContents, userContent)
		}
		// Add assistant response if available
		if len(assistantParts) > 0 {
			assistantContent := &genai.Content{
				Role:  "model",
				Parts: assistantParts,
			}
			newContents = append(newContents, assistantContent)
		}

		// Convert new messages to gollem format and append to existing history
		if len(newContents) > 0 {
			newHistory, err := convertNewHistoryToGollem(newContents)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert new messages to history")
			}
			// Preserve middleware-modified history and append new messages
			s.currentHistory.Messages = append(s.currentHistory.Messages, newHistory.Messages...)
		}

		return &gollem.ContentResponse{
			Texts:         response.Texts,
			FunctionCalls: response.FunctionCalls,
			InputToken:    response.InputToken,
			OutputToken:   response.OutputToken,
		}, nil
	}

	// Build middleware chain
	handler := gollem.ContentBlockHandler(baseHandler)
	for i := len(s.cfg.ContentBlockMiddlewares()) - 1; i >= 0; i-- {
		handler = s.cfg.ContentBlockMiddlewares()[i](handler)
	}

	// Execute middleware chain
	contentResp, err := handler(ctx, contentReq)
	if err != nil {
		return nil, err
	}

	// Convert ContentResponse back to gollem.Response
	return &gollem.Response{
		Texts:         contentResp.Texts,
		FunctionCalls: contentResp.FunctionCalls,
		InputToken:    contentResp.InputToken,
		OutputToken:   contentResp.OutputToken,
	}, nil
}

// GenerateStream generates content based on the input and returns a stream of responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	// Build the content request for middleware
	// Create a copy of the current history to avoid middleware side effects
	var historyCopy *gollem.History
	if s.currentHistory != nil {
		historyCopy = &gollem.History{
			Version:  s.currentHistory.Version,
			LLType:   s.currentHistory.LLType,
			Messages: make([]gollem.Message, len(s.currentHistory.Messages)),
		}
		copy(historyCopy.Messages, s.currentHistory.Messages)
	}

	contentReq := &gollem.ContentRequest{
		Inputs:  input,
		History: historyCopy,
	}

	// Create the base handler that performs the actual API call
	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
		// Always update history from middleware (even if same address, content may have changed)
		if req.History != nil {
			s.currentHistory = req.History
		}

		// Build complete content list from history and inputs
		var contents []*genai.Content

		// Add history to contents if available
		if s.currentHistory != nil {
			historyContents, err := s.currentHistory.ToGemini()
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert history to Gemini format")
			}
			contents = append(contents, historyContents...)
		}

		// Convert current inputs to parts
		parts, err := s.convertInputs(req.Inputs...)
		if err != nil {
			return nil, err
		}

		// Add current input as a new user message
		if len(parts) > 0 {
			userContent := &genai.Content{
				Role:  "user",
				Parts: parts,
			}
			contents = append(contents, userContent)
		}

		// Log prompt if enabled
		promptLogger := ctxlog.From(ctx, geminiPromptScope)
		if promptLogger.Enabled(ctx, slog.LevelInfo) {
			var messages []map[string]any
			for _, content := range contents {
				for _, part := range content.Parts {
					if part.Text != "" {
						messages = append(messages, map[string]any{
							"role":    content.Role,
							"type":    "text",
							"content": part.Text,
						})
					}
					if part.FunctionResponse != nil {
						messages = append(messages, map[string]any{
							"role":     content.Role,
							"type":     "function_response",
							"name":     part.FunctionResponse.Name,
							"response": part.FunctionResponse.Response,
						})
					}
				}
			}
			systemPrompt := ""
			if s.config != nil && s.config.SystemInstruction != nil && len(s.config.SystemInstruction.Parts) > 0 {
				if part := s.config.SystemInstruction.Parts[0]; part != nil {
					systemPrompt = part.Text
				}
			}
			promptLogger.Info("Gemini streaming prompt",
				"system_prompt", systemPrompt,
				"messages", messages,
			)
		}

		// Create streaming channel for middleware
		streamChan := make(chan *gollem.ContentResponse)

		// Start streaming in goroutine
		go func() {
			defer close(streamChan)

			// Get the streaming response from API
			apiStreamChan := s.apiClient.GenerateContentStream(ctx, s.model, contents, s.config)

			// Accumulate response data for history
			var accumulatedTexts []string
			var accumulatedFunctionCalls []*gollem.FunctionCall
			var totalInputTokens, totalOutputTokens int

			for streamResp := range apiStreamChan {
				if streamResp.Err != nil {
					streamChan <- &gollem.ContentResponse{
						Error: streamResp.Err,
					}
					return
				}

				// Process the response
				response, err := processResponse(streamResp.Resp)
				if err != nil {
					streamChan <- &gollem.ContentResponse{
						Error: err,
					}
					return
				}

				// Accumulate data
				accumulatedTexts = append(accumulatedTexts, response.Texts...)
				accumulatedFunctionCalls = append(accumulatedFunctionCalls, response.FunctionCalls...)
				totalInputTokens += response.InputToken
				totalOutputTokens += response.OutputToken

				// Send streaming response with delta
				streamChan <- &gollem.ContentResponse{
					Texts:         response.Texts,
					FunctionCalls: response.FunctionCalls,
					InputToken:    totalInputTokens,
					OutputToken:   totalOutputTokens,
				}
			}

			// Update history with accumulated response
			if len(accumulatedTexts) > 0 || len(accumulatedFunctionCalls) > 0 {
				// Convert inputs and response to history format
				inputMessage := gollem.Message{
					Role:     gollem.RoleUser,
					Contents: []gollem.MessageContent{},
				}
				for _, input := range req.Inputs {
					if text, ok := input.(gollem.Text); ok {
						textData, _ := json.Marshal(map[string]string{"text": string(text)})
						inputMessage.Contents = append(inputMessage.Contents, gollem.MessageContent{
							Type: gollem.MessageContentTypeText,
							Data: textData,
						})
					} else if funcResp, ok := input.(gollem.FunctionResponse); ok {
						funcData, _ := json.Marshal(funcResp)
						inputMessage.Contents = append(inputMessage.Contents, gollem.MessageContent{
							Type: gollem.MessageContentTypeFunctionResponse,
							Data: funcData,
						})
					}
				}

				assistantMessage := gollem.Message{
					Role:     gollem.RoleAssistant,
					Contents: []gollem.MessageContent{},
				}
				for _, text := range accumulatedTexts {
					textData, _ := json.Marshal(map[string]string{"text": text})
					assistantMessage.Contents = append(assistantMessage.Contents, gollem.MessageContent{
						Type: gollem.MessageContentTypeText,
						Data: textData,
					})
				}
				for _, fc := range accumulatedFunctionCalls {
					funcData, _ := json.Marshal(fc)
					assistantMessage.Contents = append(assistantMessage.Contents, gollem.MessageContent{
						Type: gollem.MessageContentTypeFunctionCall,
						Data: funcData,
					})
				}

				if len(inputMessage.Contents) > 0 {
					s.currentHistory.Messages = append(s.currentHistory.Messages, inputMessage)
				}
				if len(assistantMessage.Contents) > 0 {
					s.currentHistory.Messages = append(s.currentHistory.Messages, assistantMessage)
				}
			}
		}()

		return streamChan, nil
	}

	// Build middleware chain for streaming
	handler := gollem.ContentStreamHandler(baseHandler)
	for i := len(s.cfg.ContentStreamMiddlewares()) - 1; i >= 0; i-- {
		handler = s.cfg.ContentStreamMiddlewares()[i](handler)
	}

	// Execute middleware chain
	streamChan, err := handler(ctx, contentReq)
	if err != nil {
		return nil, err
	}

	// Convert ContentResponse stream to Response stream
	respChan := make(chan *gollem.Response)
	go func() {
		defer close(respChan)

		for contentResp := range streamChan {
			if contentResp.Error != nil {
				// Log error and continue (don't break the stream)
				continue
			}

			// Convert ContentResponse to Response
			resp := &gollem.Response{
				Texts:         contentResp.Texts,
				FunctionCalls: contentResp.FunctionCalls,
				InputToken:    contentResp.InputToken,
				OutputToken:   contentResp.OutputToken,
			}

			respChan <- resp
		}
	}()

	return respChan, nil
}

// GenerateEmbedding generates embeddings for the given input texts.
func (c *Client) GenerateEmbedding(ctx context.Context, dimension int, input []string) ([][]float64, error) {
	// Create content for embedding
	contents := make([]*genai.Content, len(input))
	for i, text := range input {
		contents[i] = &genai.Content{
			Parts: []*genai.Part{
				{Text: text},
			},
		}
	}

	// Create embedding config
	config := &genai.EmbedContentConfig{}
	if dimension > 0 && dimension <= math.MaxInt32 {
		outputDim := int32(dimension)
		config.OutputDimensionality = &outputDim
	}

	// Generate embeddings for the specified model
	result, err := c.client.Models.EmbedContent(ctx, c.embeddingModel, contents, config)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate embeddings")
	}

	if result == nil || len(result.Embeddings) == 0 {
		return nil, goerr.New("no embeddings returned")
	}

	embeddings := make([][]float64, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = make([]float64, len(emb.Values))
		for j, v := range emb.Values {
			embeddings[i][j] = float64(v)
		}
	}

	return embeddings, nil
}

// CountTokens counts the number of tokens in the given history.
func (c *Client) CountTokens(ctx context.Context, history *gollem.History) (int, error) {
	if history == nil {
		return 0, nil
	}

	// Convert history to new format
	contents, err := history.ToGemini()
	if err != nil {
		return 0, goerr.Wrap(err, "failed to convert history")
	}

	// Count tokens using the model (contents are already in new SDK format)
	result, err := c.client.Models.CountTokens(ctx, c.defaultModel, contents, nil)
	if err != nil {
		return 0, goerr.Wrap(err, "failed to count tokens")
	}

	return int(result.TotalTokens), nil
}

// Helper function to convert new SDK history to gollem.History

func convertNewHistoryToGollem(history []*genai.Content) (*gollem.History, error) {
	// Convert new format history directly to gollem.History
	if len(history) == 0 {
		return &gollem.History{}, nil
	}

	// Directly pass to NewHistoryFromGemini since it now accepts new genai types
	hist, err := gollem.NewHistoryFromGemini(history)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to convert Gemini history to gollem format")
	}
	return hist, nil
}

// convertToolToNewSDK converts gollem.Tool to new SDK's FunctionDeclaration
func convertToolToNewSDK(tool gollem.Tool) *genai.FunctionDeclaration {
	spec := tool.Spec()

	// Ensure Required is never nil - Gemini requires an empty slice, not nil
	required := spec.Required
	if required == nil {
		required = []string{}
	}

	parameters := &genai.Schema{
		Type:       genai.TypeObject,
		Properties: make(map[string]*genai.Schema),
		Required:   required,
	}

	for name, param := range spec.Parameters {
		parameters.Properties[name] = convertParameterToNewSchema(param)
	}

	return &genai.FunctionDeclaration{
		Name:        spec.Name,
		Description: spec.Description,
		Parameters:  parameters,
	}
}

// convertParameterToNewSchema converts gollem.Parameter to new SDK's schema
func convertParameterToNewSchema(param *gollem.Parameter) *genai.Schema {
	schema := &genai.Schema{
		Type:        getNewGeminiType(param.Type),
		Description: param.Description,
		Title:       param.Title,
	}

	if len(param.Enum) > 0 {
		schema.Enum = param.Enum
	}

	if param.Properties != nil {
		schema.Properties = make(map[string]*genai.Schema)
		for name, prop := range param.Properties {
			schema.Properties[name] = convertParameterToNewSchema(prop)
		}
		if len(param.Required) > 0 {
			schema.Required = param.Required
		} else {
			schema.Required = []string{}
		}
	}

	if param.Items != nil {
		schema.Items = convertParameterToNewSchema(param.Items)
	}

	// Add number constraints
	if param.Type == gollem.TypeNumber || param.Type == gollem.TypeInteger {
		if param.Minimum != nil {
			minVal := *param.Minimum
			schema.Minimum = &minVal
		}
		if param.Maximum != nil {
			maxVal := *param.Maximum
			schema.Maximum = &maxVal
		}
	}

	// Add string constraints
	if param.Type == gollem.TypeString {
		if param.MinLength != nil {
			minLen := int64(*param.MinLength)
			schema.MinLength = &minLen
		}
		if param.MaxLength != nil {
			maxLen := int64(*param.MaxLength)
			schema.MaxLength = &maxLen
		}
		if param.Pattern != "" {
			schema.Pattern = param.Pattern
		}
	}

	// Add array constraints
	if param.Type == gollem.TypeArray {
		if param.MinItems != nil {
			minItems := int64(*param.MinItems)
			schema.MinItems = &minItems
		}
		if param.MaxItems != nil {
			maxItems := int64(*param.MaxItems)
			schema.MaxItems = &maxItems
		}
	}

	return schema
}

func getNewGeminiType(paramType gollem.ParameterType) genai.Type {
	switch paramType {
	case gollem.TypeString:
		return genai.TypeString
	case gollem.TypeNumber:
		return genai.TypeNumber
	case gollem.TypeInteger:
		return genai.TypeInteger
	case gollem.TypeBoolean:
		return genai.TypeBoolean
	case gollem.TypeArray:
		return genai.TypeArray
	case gollem.TypeObject:
		return genai.TypeObject
	default:
		return genai.TypeString
	}
}
