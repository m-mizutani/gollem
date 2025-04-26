package gemini

import (
	"context"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollam"
	"google.golang.org/api/option"
)

// Client is a client for the Gemini API.
// It provides methods to interact with Google's Gemini models.
type Client struct {
	// client is the underlying Gemini client.
	client *genai.Client

	// defaultModel is the model to use for chat completions.
	// It can be overridden using WithDefaultModel option.
	defaultModel string

	// gcpOptions are additional options for Google Cloud Platform.
	// They can be set using WithGoogleCloudOptions.
	gcpOptions []option.ClientOption
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithDefaultModel sets the default model to use for chat completions.
// The model name should be a valid Gemini model identifier.
func WithDefaultModel(modelName string) Option {
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
func (c *Client) NewSession(ctx context.Context, tools []gollam.Tool) (gollam.Session, error) {
	// Convert gollam.Tool to *genai.Tool
	genaiFunctions := make([]*genai.FunctionDeclaration, len(tools))
	for i, tool := range tools {
		genaiFunctions[i] = convertTool(tool)
	}

	model := c.client.GenerativeModel(c.defaultModel)
	model.Tools = []*genai.Tool{
		{
			FunctionDeclarations: genaiFunctions,
		},
	}
	session := &Session{
		session: model.StartChat(),
	}

	return session, nil
}

// Session is a session for the Gemini chat.
// It maintains the conversation state and handles message generation.
type Session struct {
	// session is the underlying Gemini chat session.
	session *genai.ChatSession
}

// Generate processes the input and generates a response.
// It handles both text messages and function responses.
func (s *Session) Generate(ctx context.Context, input ...gollam.Input) (*gollam.Response, error) {
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

	resp, err := s.session.SendMessage(ctx, parts...)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to send message")
	}

	if len(resp.Candidates) == 0 {
		return &gollam.Response{}, nil
	}

	response := &gollam.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*gollam.FunctionCall, 0),
	}

	for _, part := range resp.Candidates[0].Content.Parts {
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

	return response, nil
}
