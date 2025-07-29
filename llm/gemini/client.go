package gemini

import (
	"context"
	"fmt"
	"math"
	"strings"
	"time"

	oldgenai "cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"google.golang.org/api/option"
	"google.golang.org/genai"
)

const (
	DefaultModel          = "gemini-2.0-flash"
	DefaultEmbeddingModel = "text-embedding-004"
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
		oldHistory, err := cfg.History().ToGemini()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to gemini.Content")
		}
		// Convert old Content format to new format
		initialHistory = make([]*genai.Content, len(oldHistory))
		for i, msg := range oldHistory {
			initialHistory[i] = convertOldContentToNew(msg)
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

func (s *Session) History() *gollem.History {
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

	return processResponse(result)
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

// IsCompatibleHistory checks if the given history is compatible with the Gemini client.
func (c *Client) IsCompatibleHistory(ctx context.Context, history *gollem.History) error {
	if history == nil {
		return nil
	}
	if history.LLType != "gemini" {
		return goerr.New("history is not compatible with Gemini", goerr.V("expected", "gemini"), goerr.V("actual", history.LLType))
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

	// Convert to new SDK format
	newContents := make([]*genai.Content, len(contents))
	for i, content := range contents {
		newContents[i] = convertOldContentToNew(content)
	}

	// Count tokens using the model
	result, err := c.client.Models.CountTokens(ctx, c.defaultModel, newContents, nil)
	if err != nil {
		return 0, goerr.Wrap(err, "failed to count tokens")
	}

	return int(result.TotalTokens), nil
}

// Helper functions to convert between old and new SDK formats

func convertOldContentToNew(old *oldgenai.Content) *genai.Content {
	if old == nil {
		return nil
	}

	newParts := make([]*genai.Part, len(old.Parts))
	for i, part := range old.Parts {
		newParts[i] = convertOldPartToNew(part)
	}

	return &genai.Content{
		Role:  old.Role,
		Parts: newParts,
	}
}

func convertOldPartToNew(old oldgenai.Part) *genai.Part {
	switch p := old.(type) {
	case oldgenai.Text:
		return &genai.Part{Text: string(p)}
	case oldgenai.Blob:
		return &genai.Part{
			InlineData: &genai.Blob{
				MIMEType: p.MIMEType,
				Data:     p.Data,
			},
		}
	case oldgenai.FileData:
		return &genai.Part{
			FileData: &genai.FileData{
				MIMEType: p.MIMEType,
				FileURI:  p.FileURI,
			},
		}
	case oldgenai.FunctionCall:
		return &genai.Part{
			FunctionCall: &genai.FunctionCall{
				Name: p.Name,
				Args: p.Args,
			},
		}
	case oldgenai.FunctionResponse:
		return &genai.Part{
			FunctionResponse: &genai.FunctionResponse{
				Name:     p.Name,
				Response: p.Response,
			},
		}
	default:
		// Return empty text for unknown types
		return &genai.Part{Text: ""}
	}
}

func convertNewHistoryToGollem(history []*genai.Content) *gollem.History {
	// Convert new format history back to gollem.History
	if len(history) == 0 {
		return &gollem.History{}
	}

	// Convert to old format first
	oldContents := make([]*oldgenai.Content, len(history))
	for i, content := range history {
		oldContents[i] = convertNewContentToOld(content)
	}

	return gollem.NewHistoryFromGemini(oldContents)
}

func convertNewContentToOld(new *genai.Content) *oldgenai.Content {
	if new == nil {
		return nil
	}

	oldParts := make([]oldgenai.Part, len(new.Parts))
	for i, part := range new.Parts {
		oldParts[i] = convertNewPartToOld(part)
	}

	return &oldgenai.Content{
		Role:  new.Role,
		Parts: oldParts,
	}
}

func convertNewPartToOld(new *genai.Part) oldgenai.Part {
	if new.Text != "" {
		return oldgenai.Text(new.Text)
	}
	if new.InlineData != nil {
		return oldgenai.Blob{
			MIMEType: new.InlineData.MIMEType,
			Data:     new.InlineData.Data,
		}
	}
	if new.FileData != nil {
		return oldgenai.FileData{
			MIMEType: new.FileData.MIMEType,
			FileURI:  new.FileData.FileURI,
		}
	}
	if new.FunctionCall != nil {
		return oldgenai.FunctionCall{
			Name: new.FunctionCall.Name,
			Args: new.FunctionCall.Args,
		}
	}
	if new.FunctionResponse != nil {
		return oldgenai.FunctionResponse{
			Name:     new.FunctionResponse.Name,
			Response: new.FunctionResponse.Response,
		}
	}
	// Return empty text for unknown types
	return oldgenai.Text("")
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
