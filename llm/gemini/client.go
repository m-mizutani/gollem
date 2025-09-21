package gemini

import (
	"context"
	"fmt"
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
	DefaultModel          = "gemini-2.0-flash"
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

	client := &Client{
		projectID:        projectID,
		location:         location,
		defaultModel:     DefaultModel,
		embeddingModel:   DefaultEmbeddingModel,
		contentType:      gollem.ContentTypeText,
		generationConfig: &genai.GenerateContentConfig{},
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

	// Prepare history if provided
	var initialHistory []*genai.Content
	if cfg.History() != nil {
		var err error
		initialHistory, err = cfg.History().ToGemini()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to gemini.Content")
		}
	}

	// Create chat with history
	chat, err := c.client.Chats.Create(ctx, c.defaultModel, config, initialHistory)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create chat")
	}

	session := &Session{
		client: c,
		chat:   chat,
		config: config,
	}

	return session, nil
}

// Session is a session for the Gemini chat.
// It maintains the conversation state and handles message generation.
type Session struct {
	client *Client
	chat   *genai.Chat
	config *genai.GenerateContentConfig
}

func (s *Session) History() (*gollem.History, error) {
	// Convert new format history to gollem.History
	return convertNewHistoryToGollem(s.chat.History(false))
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
	// Convert inputs
	parts, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	// Log prompt if enabled
	promptLogger := ctxlog.From(ctx, geminiPromptScope)
	// Build messages for logging
	var messages []map[string]any
	for _, part := range parts {
		if part.Text != "" {
			messages = append(messages, map[string]any{
				"type":    "text",
				"content": part.Text,
			})
		}
		if part.FunctionResponse != nil {
			messages = append(messages, map[string]any{
				"type":     "function_response",
				"name":     part.FunctionResponse.Name,
				"response": part.FunctionResponse.Response,
			})
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

	// Convert parts slice to individual arguments
	partsArgs := make([]genai.Part, len(parts))
	for i, p := range parts {
		partsArgs[i] = *p
	}

	// Send message
	result, err := s.chat.SendMessage(ctx, partsArgs...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to generate content")
	}

	response, err := processResponse(result)
	if err != nil {
		return nil, err
	}

	// Log responses if GOLLEM_LOGGING_GEMINI_RESPONSE is set
	responseLogger := ctxlog.From(ctx, geminiResponseScope)
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

	return response, nil
}

// GenerateStream generates content based on the input and returns a stream of responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	// Convert inputs
	parts, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	respChan := make(chan *gollem.Response)

	go func() {
		defer close(respChan)

		// Convert parts slice to individual arguments
		partsArgs := make([]genai.Part, len(parts))
		for i, p := range parts {
			partsArgs[i] = *p
		}

		// Accumulate streaming response for logging
		var allTexts []string
		var allFunctionCalls []*gollem.FunctionCall
		var totalInputTokens int
		var totalOutputTokens int
		var lastFinishReason string

		// Use SendMessageStream for streaming
		for result, err := range s.chat.SendMessageStream(ctx, partsArgs...) {
			if err != nil {
				// Send error response
				respChan <- &gollem.Response{
					Error: err,
				}
				return
			}

			resp, err := processResponse(result)
			if err != nil {
				respChan <- &gollem.Response{
					Error: err,
				}
				return
			}

			// Accumulate for logging
			allTexts = append(allTexts, resp.Texts...)
			allFunctionCalls = append(allFunctionCalls, resp.FunctionCalls...)
			if resp.InputToken > 0 {
				totalInputTokens = resp.InputToken
			}
			if resp.OutputToken > 0 {
				totalOutputTokens = resp.OutputToken
			}
			if len(result.Candidates) > 0 {
				lastFinishReason = string(result.Candidates[0].FinishReason)
			}

			respChan <- resp
		}

		// Log streaming response if GOLLEM_LOGGING_GEMINI_RESPONSE is set
		responseLogger := ctxlog.From(ctx, geminiResponseScope)
		var logContent []map[string]any
		for _, text := range allTexts {
			logContent = append(logContent, map[string]any{
				"type": "text",
				"text": text,
			})
		}
		for _, funcCall := range allFunctionCalls {
			logContent = append(logContent, map[string]any{
				"type":      "function_call",
				"id":        funcCall.ID,
				"name":      funcCall.Name,
				"arguments": funcCall.Arguments,
			})
		}
		responseLogger.Info("Gemini streaming response",
			"finish_reason", lastFinishReason,
			"usage", map[string]any{
				"prompt_tokens":     totalInputTokens,
				"candidates_tokens": totalOutputTokens,
			},
			"content", logContent,
		)
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

// IsCompatibleHistory checks if the given history is compatible with the Gemini client.
func (c *Client) IsCompatibleHistory(ctx context.Context, history *gollem.History) error {
	if history == nil {
		return nil
	}
	if history.LLType != gollem.LLMTypeGemini {
		return goerr.New("history is not compatible with Gemini", goerr.V("expected", gollem.LLMTypeGemini), goerr.V("actual", history.LLType))
	}
	if history.Version != gollem.HistoryVersion {
		return goerr.New("history version is not supported", goerr.V("expected", gollem.HistoryVersion), goerr.V("actual", history.Version))
	}
	return nil
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
