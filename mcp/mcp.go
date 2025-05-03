package mcp

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/santhosh-tekuri/jsonschema/v6"
)

type Client struct {
	// For local MCP server
	path    string
	args    []string
	envVars []string

	// For remote MCP server
	baseURL string
	headers map[string]string

	// Common client
	client *client.Client

	initResult *mcp.InitializeResult
	initMutex  sync.Mutex
}

// Specs implements gollem.ToolSet interface
func (c *Client) Specs(ctx context.Context) ([]gollem.ToolSpec, error) {
	tools, err := c.listTools(ctx)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list tools")
	}

	specs := make([]gollem.ToolSpec, len(tools))
	for i, tool := range tools {
		param, err := inputSchemaToParameter(tool.InputSchema)
		if err != nil {
			return nil, goerr.Wrap(err,
				"failed to convert input schema to parameter",
				goerr.V("tool.name", tool.Name),
				goerr.V("tool.inputSchema", tool.InputSchema),
			)
		}

		specs[i] = gollem.ToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  param.Properties,
			Required:    param.Required,
		}
	}

	return specs, nil
}

// Run implements gollem.ToolSet interface
func (c *Client) Run(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	resp, err := c.callTool(ctx, name, args)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to call tool")
	}

	return mcpContentToMap(resp.Content), nil
}

// StdioOption is the option for the MCP client for local MCP executable server via stdio.
type StdioOption func(*Client)

// WithEnvVars sets the environment variables for the MCP client. It appends the environment variables to the existing ones.
func WithEnvVars(envVars []string) StdioOption {
	return func(m *Client) {
		m.envVars = append(m.envVars, envVars...)
	}
}

// NewStdio creates a new MCP client for local MCP executable server via stdio.
func NewStdio(ctx context.Context, path string, args []string, options ...StdioOption) (*Client, error) {
	client := &Client{
		path: path,
		args: args,
	}
	for _, option := range options {
		option(client)
	}

	if err := client.init(ctx); err != nil {
		return nil, goerr.Wrap(err, "failed to initialize MCP client")
	}

	return client, nil
}

// SSEOption is the option for the MCP client for remote MCP server via HTTP SSE.
type SSEOption func(*Client)

// WithHeaders sets the headers for the MCP client. It replaces the existing headers setting.
func WithHeaders(headers map[string]string) SSEOption {
	return func(m *Client) {
		m.headers = headers
	}
}

// NewSSE creates a new MCP client for remote MCP server via HTTP SSE.
func NewSSE(ctx context.Context, baseURL string, options ...SSEOption) (*Client, error) {
	client := &Client{
		baseURL: baseURL,
	}
	for _, option := range options {
		option(client)
	}

	if err := client.init(ctx); err != nil {
		return nil, goerr.Wrap(err, "failed to initialize MCP client")
	}

	return client, nil
}

func (c *Client) init(ctx context.Context) error {
	c.initMutex.Lock()
	defer c.initMutex.Unlock()

	if c.initResult != nil {
		return nil
	}

	var tp transport.Interface
	if c.path != "" {
		tp = transport.NewStdio(c.path, c.envVars, c.args...)
	}

	if c.baseURL != "" {
		sse, err := transport.NewSSE(c.baseURL, transport.WithHeaders(c.headers))
		if err != nil {
			return goerr.Wrap(err, "failed to create SSE transport")
		}
		tp = sse
	}

	if tp == nil {
		return goerr.New("no transport")
	}

	c.client = client.NewClient(tp)

	if err := c.client.Start(ctx); err != nil {
		return goerr.Wrap(err, "failed to start MCP client")
	}

	var initRequest mcp.InitializeRequest
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "gollem",
		Version: "0.0.1",
	}

	if resp, err := c.client.Initialize(ctx, initRequest); err != nil {
		return goerr.Wrap(err, "failed to initialize MCP client")
	} else {
		c.initResult = resp
	}

	return nil
}

func (c *Client) listTools(ctx context.Context) ([]mcp.Tool, error) {
	// ListTools is thread safe
	resp, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list tools")
	}

	return resp.Tools, nil
}

func (c *Client) callTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	resp, err := c.client.CallTool(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to call tool")
	}

	return resp, nil
}

func (c *Client) Close() error {
	if err := c.client.Close(); err != nil {
		return goerr.Wrap(err, "failed to close MCP client")
	}
	return nil
}

func inputSchemaToParameter(inputSchema mcp.ToolInputSchema) (*gollem.Parameter, error) {
	parameters := map[string]*gollem.Parameter{}
	jsonSchema, err := json.Marshal(inputSchema)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to marshal input schema")
	}

	rawSchema, err := jsonschema.UnmarshalJSON(bytes.NewReader(jsonSchema))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to compile input schema")
	}

	c := jsonschema.NewCompiler()
	if err := c.AddResource("schema.json", rawSchema); err != nil {
		return nil, goerr.Wrap(err, "failed to add resource to compiler")
	}
	schema, err := c.Compile("schema.json")
	if err != nil {
		return nil, goerr.Wrap(err, "failed to compile input schema")
	}

	schemaType := schema.Types.ToStrings()
	if len(schemaType) != 1 || schemaType[0] != "object" {
		return nil, goerr.Wrap(gollem.ErrInvalidTool, "invalid input schema", goerr.V("schema", schema))
	}

	for name, property := range schema.Properties {
		parameters[name] = jsonSchemaToParameter(property)
	}

	return &gollem.Parameter{
		Type:        gollem.ParameterType(schema.Types.ToStrings()[0]),
		Title:       schema.Title,
		Description: schema.Description,
		Required:    schema.Required,
		Properties:  parameters,
	}, nil
}

func jsonSchemaToParameter(schema *jsonschema.Schema) *gollem.Parameter {
	var enum []string
	if schema.Enum != nil {
		for _, v := range schema.Enum.Values {
			enum = append(enum, fmt.Sprintf("%v", v))
		}
	}

	properties := map[string]*gollem.Parameter{}
	for name, property := range schema.Properties {
		properties[name] = jsonSchemaToParameter(property)
	}

	var items *gollem.Parameter
	if schema.Items != nil {
		switch v := schema.Items.(type) {
		case *jsonschema.Schema:
			items = jsonSchemaToParameter(v)
		}
	}

	return &gollem.Parameter{
		Type:        gollem.ParameterType(schema.Types.ToStrings()[0]),
		Title:       schema.Title,
		Description: schema.Description,
		Required:    schema.Required,
		Enum:        enum,
		Properties:  properties,
		Items:       items,
	}
}

func mcpContentToMap(contents []mcp.Content) map[string]any {
	for _, c := range contents {
		if txt, ok := c.(*mcp.TextContent); ok {
			var v any
			if err := json.Unmarshal([]byte(txt.Text), &v); err == nil {
				if mapData, ok := v.(map[string]any); ok {
					return mapData
				}

				return map[string]any{
					"result": v,
				}
			}

			return map[string]any{
				"result": txt.Text,
			}
		}
	}

	// No appropriate content found
	return map[string]any{}
}
