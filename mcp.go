package servantic

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
)

type MCPClient struct {
	// For local MCP server
	path    string
	args    []string
	envVars []string

	// For remote MCP server
	baseURL string
	headers map[string]string

	// Common client
	client     *client.Client
	initResult *mcp.InitializeResult

	initMutex sync.Mutex
}

// MCPonStdioOption is the option for the MCP client for local MCP executable server via stdio.
type MCPonStdioOption func(*MCPClient)

// WithEnvVars sets the environment variables for the MCP client. It appends the environment variables to the existing ones.
func WithEnvVars(envVars []string) MCPonStdioOption {
	return func(m *MCPClient) {
		m.envVars = append(m.envVars, envVars...)
	}
}

// MCPonSSEOption is the option for the MCP client for remote MCP server via HTTP SSE.
type MCPonSSEOption func(*MCPClient)

// WithHeaders sets the headers for the MCP client. It replaces the existing headers setting.
func WithHeaders(headers map[string]string) MCPonSSEOption {
	return func(m *MCPClient) {
		m.headers = headers
	}
}

func (c *MCPClient) start(ctx context.Context) error {
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
		Name:    "servantic",
		Version: "0.0.1",
	}

	if resp, err := c.client.Initialize(ctx, initRequest); err != nil {
		return goerr.Wrap(err, "failed to initialize MCP client")
	} else {
		c.initResult = resp
	}

	return nil
}

func (c *MCPClient) listTools(ctx context.Context) ([]mcp.Tool, error) {
	if c.initResult == nil {
		return nil, goerr.New("MCP client not initialized")
	}

	resp, err := c.client.ListTools(ctx, mcp.ListToolsRequest{})
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list tools")
	}

	return resp.Tools, nil
}

func (c *MCPClient) callTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	if c.initResult == nil {
		return nil, goerr.New("MCP client not initialized")
	}

	req := mcp.CallToolRequest{}
	req.Params.Name = name
	req.Params.Arguments = args
	resp, err := c.client.CallTool(ctx, req)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to call tool")
	}

	return resp, nil
}

func (c *MCPClient) close() error {
	if err := c.client.Close(); err != nil {
		return goerr.Wrap(err, "failed to close MCP client")
	}
	return nil
}

func valueOrEmpty[T any](v any) T {
	var empty T
	if v == nil {
		return empty
	}
	if v, ok := v.(T); ok {
		return v
	}
	return empty
}

func inputSchemaToParameter(inputSchema mcp.ToolInputSchema) (map[string]*Parameter, error) {
	parameters := map[string]*Parameter{}

	for name, property := range inputSchema.Properties {
		prop, ok := property.(map[string]any)
		if !ok {
			return nil, goerr.Wrap(ErrInvalidInputSchema, "invalid property", goerr.V("property", property))
		}

		parameter, err := propertyToParameter(name, prop)
		if err != nil {
			return nil, err
		}
		parameters[name] = parameter
	}

	return parameters, nil
}

func propertyToParameter(name string, prop map[string]any) (*Parameter, error) {
	var properties map[string]*Parameter
	var items *Parameter
	propType := valueOrEmpty[string](prop["type"])

	if propType == "object" {
		properties = map[string]*Parameter{}
		nestedProperty := valueOrEmpty[map[string]any](prop["properties"])

		for k, v := range nestedProperty {
			objParam, err := propertyToParameter(k, v.(map[string]any))
			if err != nil {
				return nil, err
			}
			properties[k] = objParam
		}
	}

	if propType == "array" {
		v, err := propertyToParameter(name, prop["items"].(map[string]any))
		if err != nil {
			return nil, err
		}
		items = v
	}

	return &Parameter{
		Name:        name,
		Type:        ParameterType(propType),
		Title:       valueOrEmpty[string](prop["title"]),
		Description: valueOrEmpty[string](prop["description"]),
		Required:    valueOrEmpty[bool](prop["required"]),
		Enum:        valueOrEmpty[[]string](prop["enum"]),
		Properties:  properties,
		Items:       items,
	}, nil
}

func wrapMCPToolCall(mcpClient *MCPClient, tool mcp.Tool) (*toolWrapper, error) {
	parameters, err := inputSchemaToParameter(tool.InputSchema)
	if err != nil {
		return nil, err
	}

	return &toolWrapper{
		spec: &ToolSpec{
			Name:        tool.Name,
			Description: tool.Description,
			Parameters:  parameters,
		},
		run: func(ctx context.Context, args map[string]any) (map[string]any, error) {
			resp, err := mcpClient.callTool(ctx, tool.Name, args)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to call tool")
			}

			return mcpContentToMap(resp.Content), nil
		},
	}, nil
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
