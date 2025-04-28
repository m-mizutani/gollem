package gemini

import (
	"context"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollam"
	"google.golang.org/api/iterator"
	"google.golang.org/api/option"
)

// Client is a client for the Gemini API.
// It provides methods to interact with Google's Gemini models.
type Client struct {
	// client is the underlying Gemini client.
	client *genai.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithModel option.
	defaultModel string

	// gcpOptions are additional options for Google Cloud Platform.
	// They can be set using WithGoogleCloudOptions.
	gcpOptions []option.ClientOption

	// generationConfig contains the default generation parameters
	generationConfig genai.GenerationConfig
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithModel sets the default model to use for chat completions.
// The model name should be a valid Gemini model identifier.
// Default: "gemini-2.0-flash"
func WithModel(modelName string) Option {
	return func(c *Client) {
		c.defaultModel = modelName
	}
}

// WithGoogleCloudOptions sets additional options for Google Cloud Platform.
// These options are passed to the underlying Gemini client.
func WithGoogleCloudOptions(options ...option.ClientOption) Option {
	return func(c *Client) {
		c.gcpOptions = options
	}
}

// WithTemperature sets the temperature parameter for text generation.
// Higher values make the output more random, lower values make it more focused.
// Range: 0.0 to 1.0
func WithTemperature(temp float32) Option {
	return func(c *Client) {
		c.generationConfig.Temperature = &temp
	}
}

// WithTopP sets the top_p parameter for text generation.
// Controls diversity via nucleus sampling.
// Range: 0.0 to 1.0
func WithTopP(topP float32) Option {
	return func(c *Client) {
		c.generationConfig.TopP = &topP
	}
}

// WithTopK sets the top_k parameter for text generation.
// Controls diversity via top-k sampling.
// Range: 1 to 40
func WithTopK(topK int32) Option {
	return func(c *Client) {
		c.generationConfig.TopK = &topK
	}
}

// WithMaxTokens sets the maximum number of tokens to generate.
func WithMaxTokens(maxTokens int32) Option {
	return func(c *Client) {
		c.generationConfig.MaxOutputTokens = &maxTokens
	}
}

// WithStopSequences sets the stop sequences for text generation.
func WithStopSequences(stopSequences []string) Option {
	return func(c *Client) {
		c.generationConfig.StopSequences = stopSequences
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
		defaultModel: "gemini-2.0-flash",
	}

	for _, option := range options {
		option(client)
	}

	newClient, err := genai.NewClient(ctx, projectID, location, client.gcpOptions...)
	if err != nil {
		return nil, err
	}

	client.client = newClient

	return client, nil
}

// NewSession creates a new session for the Gemini API.
// It converts the provided tools to Gemini's tool format and initializes a new chat session.
func (c *Client) NewSession(ctx context.Context, tools []gollam.Tool, histories ...*gollam.History) (gollam.Session, error) {
	// Convert gollam.Tool to *genai.Tool
	genaiFunctions := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		genaiFunctions[i] = convertTool(tool)
	}

	var messages []*genai.Content
	for _, history := range histories {
		history, err := history.ToGemini()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to gemini.Content")
		}
		messages = append(messages, history...)
	}

	model := c.client.GenerativeModel(c.defaultModel)
	model.GenerationConfig = c.generationConfig

	if len(genaiFunctions) > 0 {
		model.Tools = []*genai.Tool{
			{
				FunctionDeclarations: genaiFunctions,
			},
		}
	}

	session := &Session{
		session: model.StartChat(),
	}
	if len(messages) > 0 {
		session.session.History = messages
	}

	return session, nil
}

func (s *Session) History() *gollam.History {
	return gollam.NewHistoryFromGemini(s.session.History)
}

// Session is a session for the Gemini chat.
// It maintains the conversation state and handles message generation.
type Session struct {
	// session is the underlying Gemini chat session.
	session *genai.ChatSession
}

// convertInputs converts gollam.Input to Gemini parts
func (s *Session) convertInputs(input ...gollam.Input) ([]genai.Part, error) {
	parts := make([]genai.Part, len(input))
	for i, in := range input {
		switch v := in.(type) {
		case gollam.Text:
			parts[i] = genai.Text(string(v))
		case gollam.FunctionResponse:
			if v.Error != nil {
				parts[i] = genai.FunctionResponse{
					Name: v.Name,
					Response: map[string]any{
						"error_message": v.Error.Error(),
					},
				}
			} else {
				parts[i] = genai.FunctionResponse{
					Name:     v.Name,
					Response: v.Data,
				}
			}
		default:
			return nil, goerr.Wrap(gollam.ErrInvalidParameter, "invalid input")
		}
	}
	return parts, nil
}

// processResponse converts Gemini response to gollam.Response
func processResponse(resp *genai.GenerateContentResponse) *gollam.Response {
	if len(resp.Candidates) == 0 {
		return &gollam.Response{}
	}

	response := &gollam.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollam.FunctionCall, 0),
	}

	for _, candidate := range resp.Candidates {
		for _, part := range candidate.Content.Parts {
			switch v := part.(type) {
			case genai.Text:
				response.Texts = append(response.Texts, string(v))
			case genai.FunctionCall:
				response.FunctionCalls = append(response.FunctionCalls, &gollam.FunctionCall{
					Name:      v.Name,
					Arguments: v.Args,
				})
			}
		}
	}

	return response
}

// GenerateContent processes the input and generates a response.
// It handles both text messages and function responses.
func (s *Session) GenerateContent(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
	parts, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	resp, err := s.session.SendMessage(ctx, parts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to send message")
	}

	return processResponse(resp), nil
}

// GenerateStream processes the input and generates a response stream.
// It handles both text messages and function responses, and returns a channel for streaming responses.
func (s *Session) GenerateStream(ctx context.Context, input ...gollam.Input) (<-chan *gollam.Response, error) {
	parts, err := s.convertInputs(input...)
	if err != nil {
		return nil, err
	}

	iter := s.session.SendMessageStream(ctx, parts...)
	responseChan := make(chan *gollam.Response)

	go func() {
		defer close(responseChan)

		for {
			resp, err := iter.Next()
			if err != nil {
				if err == iterator.Done {
					return
				}
				responseChan <- &gollam.Response{
					Error: goerr.Wrap(err, "failed to generate stream"),
				}
				return
			}

			responseChan <- processResponse(resp)
		}
	}()

	return responseChan, nil
}
