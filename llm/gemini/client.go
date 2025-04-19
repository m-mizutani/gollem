package gemini

import (
	"context"

	"cloud.google.com/go/vertexai/genai"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/servant"
	"google.golang.org/api/option"
)

// Client is a client for the Gemini API.
type Client struct {
	client       *genai.Client
	defaultModel string
	gcpOptions   []option.ClientOption
}

type Option func(*Client)

func WithDefaultModel(modelName string) Option {
	return func(c *Client) {
		c.defaultModel = modelName
	}
}

func WithGoogleCloudOptions(options ...option.ClientOption) Option {
	return func(c *Client) {
		c.gcpOptions = options
	}
}

// New creates a new client for the Gemini API.
func New(ctx context.Context, projectID, location string, options ...Option) (*Client, error) {
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
func (c *Client) NewSession(ctx context.Context, tools []servant.Tool) (servant.Session, error) {
	// Convert servant.Tool to *genai.Tool
	genaiTools := make([]*genai.Tool, len(tools))
	for i, tool := range tools {
		genaiTools[i] = convertTool(tool)
	}

	model := c.client.GenerativeModel(c.defaultModel)
	model.Tools = genaiTools
	session := &Session{
		session: model.StartChat(),
	}

	return session, nil
}

// Session is a session for the Gemini chat.
type Session struct {
	session *genai.ChatSession
}

func (s *Session) Generate(ctx context.Context, input ...servant.Input) (*servant.Response, error) {
	parts := make([]genai.Part, len(input))
	for i, in := range input {
		switch v := in.(type) {
		case servant.Text:
			parts[i] = genai.Text(string(v))
		case servant.FunctionResponse:
			parts[i] = genai.FunctionResponse{
				Name:     v.Name,
				Response: v.Data,
			}
		default:
			return nil, goerr.Wrap(servant.ErrInvalidParameter, "invalid input")
		}
	}

	resp, err := s.session.SendMessage(ctx, parts...)
	if err != nil {
		return nil, err
	}

	if len(resp.Candidates) == 0 {
		return &servant.Response{}, nil
	}

	response := &servant.Response{
		Texts:         make([]string, 0),
		FunctionCalls: make([]*servant.FunctionCall, 0),
	}

	for _, part := range resp.Candidates[0].Content.Parts {
		switch v := part.(type) {
		case genai.Text:
			response.Texts = append(response.Texts, string(v))
		case genai.FunctionCall:
			response.FunctionCalls = append(response.FunctionCalls, &servant.FunctionCall{
				Name:      v.Name,
				Arguments: v.Args,
			})
		}
	}

	return response, nil
}
