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

// transportType represents the transport type for MCP client
type transportType string

const (
	transportTypeStdio          transportType = "stdio"
	transportTypeSSE            transportType = "sse"
	transportTypeStreamableHTTP transportType = "streamable-http"
)

type Client struct {
	// For local MCP server
	path    string
	args    []string
	envVars []string

	// For remote MCP server
	baseURL string
	headers map[string]string

	// Transport type
	transportType transportType

	// Common client
	client *client.Client

	initResult *mcp.InitializeResult
	initMutex  sync.Mutex
}

// Specs implements gollem.ToolSet interface
func (c *Client) Specs(ctx context.Context) ([]gollem.ToolSpec, error) {
	logger := gollem.LoggerFromContext(ctx)

	tools, err := c.listTools(ctx)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list tools")
	}

	specs := make([]gollem.ToolSpec, len(tools))
	toolNames := make([]string, len(tools))

	for i, tool := range tools {
		toolNames[i] = tool.Name

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

	logger.Debug("found MCP tools", "names", toolNames)

	return specs, nil
}

// Run implements gollem.ToolSet interface
func (c *Client) Run(ctx context.Context, name string, args map[string]any) (map[string]any, error) {
	logger := gollem.LoggerFromContext(ctx)

	logger.Debug("call MCP tool", "name", name, "args", args)

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
		path:          path,
		args:          args,
		transportType: transportTypeStdio,
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
		baseURL:       baseURL,
		transportType: transportTypeSSE,
	}
	for _, option := range options {
		option(client)
	}

	if err := client.init(ctx); err != nil {
		return nil, goerr.Wrap(err, "failed to initialize MCP client")
	}

	return client, nil
}

// StreamableHTTPOption is the option for the MCP client for remote MCP server via Streamable HTTP.
type StreamableHTTPOption func(*Client)

// WithStreamableHTTPHeaders sets the headers for the MCP client. It replaces the existing headers setting.
func WithStreamableHTTPHeaders(headers map[string]string) StreamableHTTPOption {
	return func(m *Client) {
		m.headers = headers
	}
}

// NewStreamableHTTP creates a new MCP client for remote MCP server via Streamable HTTP.
func NewStreamableHTTP(ctx context.Context, baseURL string, options ...StreamableHTTPOption) (*Client, error) {
	client := &Client{
		baseURL:       baseURL,
		transportType: transportTypeStreamableHTTP,
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

	logger := gollem.LoggerFromContext(ctx)

	if c.initResult != nil {
		return nil
	}

	var tp transport.Interface
	if c.path != "" {
		tp = transport.NewStdio(c.path, c.envVars, c.args...)
	}

	if c.baseURL != "" {
		switch c.transportType {
		case transportTypeSSE:
			sse, err := transport.NewSSE(c.baseURL, transport.WithHeaders(c.headers))
			if err != nil {
				return goerr.Wrap(err, "failed to create SSE transport")
			}
			tp = sse
		case transportTypeStreamableHTTP:
			streamableHttp, err := transport.NewStreamableHTTP(c.baseURL, transport.WithHTTPHeaders(c.headers))
			if err != nil {
				return goerr.Wrap(err, "failed to create Streamable HTTP transport")
			}
			tp = streamableHttp
		default:
			// Default to SSE for backward compatibility
			sse, err := transport.NewSSE(c.baseURL, transport.WithHeaders(c.headers))
			if err != nil {
				return goerr.Wrap(err, "failed to create SSE transport")
			}
			tp = sse
		}
	}

	if tp == nil {
		return goerr.New("no transport")
	}

	c.client = client.NewClient(tp)

	logger.Debug("starting MCP client", "path", c.path, "url", c.baseURL)
	if err := c.client.Start(ctx); err != nil {
		return goerr.Wrap(err, "failed to start MCP client")
	}

	var initRequest mcp.InitializeRequest
	initRequest.Params.ProtocolVersion = mcp.LATEST_PROTOCOL_VERSION
	initRequest.Params.ClientInfo = mcp.Implementation{
		Name:    "gollem",
		Version: "0.0.1",
	}

	logger.Debug("initializing MCP client")
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

	var minimum, maximum *float64
	if schema.Minimum != nil {
		min, _ := (*schema.Minimum).Float64()
		minimum = &min
	}
	if schema.Maximum != nil {
		max, _ := (*schema.Maximum).Float64()
		maximum = &max
	}

	var minLength, maxLength *int
	if schema.MinLength != nil {
		min := int(*schema.MinLength)
		minLength = &min
	}
	if schema.MaxLength != nil {
		max := int(*schema.MaxLength)
		maxLength = &max
	}

	var minItems, maxItems *int
	if schema.MinItems != nil {
		min := int(*schema.MinItems)
		minItems = &min
	}
	if schema.MaxItems != nil {
		max := int(*schema.MaxItems)
		maxItems = &max
	}

	var pattern string
	if schema.Pattern != nil {
		pattern = schema.Pattern.String()
	}

	return &gollem.Parameter{
		Type:        gollem.ParameterType(schema.Types.ToStrings()[0]),
		Title:       schema.Title,
		Description: schema.Description,
		Required:    schema.Required,
		Enum:        enum,
		Properties:  properties,
		Items:       items,
		Minimum:     minimum,
		Maximum:     maximum,
		MinLength:   minLength,
		MaxLength:   maxLength,
		Pattern:     pattern,
		MinItems:    minItems,
		MaxItems:    maxItems,
		Default:     schema.Default,
	}
}

func mcpContentToMap(contents []mcp.Content) map[string]any {
	if len(contents) == 0 {
		return nil
	}

	if len(contents) == 1 {
		if content, ok := contents[0].(mcp.TextContent); ok {
			var v any
			if err := json.Unmarshal([]byte(content.Text), &v); err == nil {
				if mapData, ok := v.(map[string]any); ok {
					return mapData
				}
			}
			return map[string]any{
				"result": content.Text,
			}
		}
		return nil
	}

	result := map[string]any{}
	for i, c := range contents {
		if content, ok := c.(mcp.TextContent); ok {
			result[fmt.Sprintf("content_%d", i+1)] = content.Text
		}
	}
	return result
}
