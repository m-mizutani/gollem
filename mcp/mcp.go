package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

const (
	// DefaultClientName is the default name for MCP client
	DefaultClientName = "gollem"
	// DefaultClientVersion is the default version for MCP client
	// Empty string means no specific version is advertised
	DefaultClientVersion = ""
)

type Client struct {
	// 公式SDKクライアント
	mcpClient *mcp.Client
	session   *mcp.ClientSession

	// 設定保持
	name    string
	version string

	// トランスポート関連
	transport mcp.Transport

	// オプション
	envVars []string
	headers map[string]string

	// 接続管理
	initMutex sync.Mutex
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

		param, err := convertToolToSpec(tool)
		if err != nil {
			return nil, goerr.Wrap(err,
				"failed to convert tool to spec",
				goerr.V("tool.name", tool.Name),
			)
		}

		specs[i] = param
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

	return convertContentToMap(resp.Content), nil
}

// StdioOption is the option for the MCP client for local MCP executable server via stdio.
type StdioOption func(*Client)

// WithEnvVars sets the environment variables for the MCP client. It appends the environment variables to the existing ones.
func WithEnvVars(envVars []string) StdioOption {
	return func(m *Client) {
		m.envVars = append(m.envVars, envVars...)
	}
}

// WithStdioClientInfo sets the client name and version for the MCP client.
func WithStdioClientInfo(name, version string) StdioOption {
	return func(m *Client) {
		m.name = name
		m.version = version
	}
}

// NewStdio creates a new MCP client for local MCP executable server via stdio.
func NewStdio(ctx context.Context, path string, args []string, options ...StdioOption) (*Client, error) {
	client := &Client{
		name:    DefaultClientName,
		version: DefaultClientVersion,
	}
	for _, option := range options {
		option(client)
	}

	// Create command with environment variables
	cmd := exec.Command(path, args...)
	if len(client.envVars) > 0 {
		cmd.Env = append(cmd.Env, client.envVars...)
	}

	// Create transport
	transport := mcp.NewStdioTransport()
	client.transport = transport

	if err := client.init(ctx, cmd); err != nil {
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

// WithSSEClientInfo sets the client name and version for the MCP client.
func WithSSEClientInfo(name, version string) SSEOption {
	return func(m *Client) {
		m.name = name
		m.version = version
	}
}

// NewSSE creates a new MCP client for remote MCP server via HTTP SSE.
func NewSSE(ctx context.Context, baseURL string, options ...SSEOption) (*Client, error) {
	client := &Client{
		name:    DefaultClientName,
		version: DefaultClientVersion,
		headers: make(map[string]string),
	}
	for _, option := range options {
		option(client)
	}

	// TODO: SSE transport implementation when available in official SDK
	return nil, goerr.New("SSE transport not yet implemented with official SDK")
}

// StreamableHTTPOption is the option for the MCP client for remote MCP server via Streamable HTTP.
type StreamableHTTPOption func(*Client)

// WithStreamableHTTPHeaders sets the headers for the MCP client. It replaces the existing headers setting.
func WithStreamableHTTPHeaders(headers map[string]string) StreamableHTTPOption {
	return func(m *Client) {
		m.headers = headers
	}
}

// WithStreamableHTTPClientInfo sets the client name and version for the MCP client.
func WithStreamableHTTPClientInfo(name, version string) StreamableHTTPOption {
	return func(m *Client) {
		m.name = name
		m.version = version
	}
}

// NewStreamableHTTP creates a new MCP client for remote MCP server via Streamable HTTP.
func NewStreamableHTTP(ctx context.Context, baseURL string, options ...StreamableHTTPOption) (*Client, error) {
	client := &Client{
		name:    DefaultClientName,
		version: DefaultClientVersion,
		headers: make(map[string]string),
	}
	for _, option := range options {
		option(client)
	}

	// TODO: StreamableHTTP transport implementation when available in official SDK
	return nil, goerr.New("StreamableHTTP transport not yet implemented with official SDK")
}

func (c *Client) init(ctx context.Context, cmd *exec.Cmd) error {
	c.initMutex.Lock()
	defer c.initMutex.Unlock()

	logger := gollem.LoggerFromContext(ctx)

	if c.session != nil {
		return nil
	}

	// Create client with official SDK
	c.mcpClient = mcp.NewClient(c.name, c.version, nil)

	// Connect using stdio transport with command
	if cmd != nil {
		transport := mcp.NewCommandTransport(cmd)
		session, err := c.mcpClient.Connect(ctx, transport)
		if err != nil {
			return goerr.Wrap(err, "failed to connect to MCP server")
		}
		c.session = session
	}

	logger.Debug("MCP client initialized", "name", c.name, "version", c.version)

	return nil
}

func (c *Client) listTools(ctx context.Context) ([]*mcp.Tool, error) {
	if c.session == nil {
		return nil, goerr.New("session not initialized")
	}

	resp, err := c.session.ListTools(ctx, &mcp.ListToolsParams{})
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list tools")
	}

	return resp.Tools, nil
}

func (c *Client) callTool(ctx context.Context, name string, args map[string]any) (*mcp.CallToolResult, error) {
	if c.session == nil {
		return nil, goerr.New("session not initialized")
	}

	params := &mcp.CallToolParams{
		Name:      name,
		Arguments: args,
	}

	resp, err := c.session.CallTool(ctx, params)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to call tool")
	}

	return resp, nil
}

func (c *Client) Close() error {
	if c.session != nil {
		if err := c.session.Close(); err != nil {
			return goerr.Wrap(err, "failed to close MCP session")
		}
	}
	return nil
}

// convertToolToSpec converts MCP Tool to gollem.ToolSpec
func convertToolToSpec(tool *mcp.Tool) (gollem.ToolSpec, error) {
	spec := gollem.ToolSpec{
		Name:        tool.Name,
		Description: tool.Description,
		Parameters:  make(map[string]*gollem.Parameter),
		Required:    []string{},
	}

	// Convert input schema if available
	if tool.InputSchema != nil {
		param, err := convertInputSchemaToParameter(tool.InputSchema)
		if err != nil {
			return spec, goerr.Wrap(err, "failed to convert input schema")
		}
		spec.Parameters = param.Properties
		spec.Required = param.Required
	}

	return spec, nil
}

// convertInputSchemaToParameter converts MCP input schema to gollem Parameter
func convertInputSchemaToParameter(schema any) (*gollem.Parameter, error) {
	// Convert schema to JSON and back to map for processing
	schemaBytes, err := json.Marshal(schema)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to marshal schema")
	}

	var schemaMap map[string]any
	if err := json.Unmarshal(schemaBytes, &schemaMap); err != nil {
		return nil, goerr.Wrap(err, "failed to unmarshal schema")
	}

	param := &gollem.Parameter{
		Type:       gollem.TypeObject,
		Properties: make(map[string]*gollem.Parameter),
		Required:   []string{},
	}

	// Extract properties
	if props, ok := schemaMap["properties"].(map[string]any); ok {
		for name, propSchema := range props {
			propParam, err := convertSchemaProperty(propSchema)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert property", goerr.V("property", name))
			}
			param.Properties[name] = propParam
		}
	}

	// Extract required fields
	if required, ok := schemaMap["required"].([]any); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				param.Required = append(param.Required, reqStr)
			}
		}
	}

	return param, nil
}

// convertSchemaProperty converts a single schema property to gollem Parameter
func convertSchemaProperty(propSchema any) (*gollem.Parameter, error) {
	propBytes, err := json.Marshal(propSchema)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to marshal property schema")
	}

	var propMap map[string]any
	if err := json.Unmarshal(propBytes, &propMap); err != nil {
		return nil, goerr.Wrap(err, "failed to unmarshal property schema")
	}

	param := &gollem.Parameter{}

	// Type
	if typeVal, ok := propMap["type"].(string); ok {
		param.Type = gollem.ParameterType(typeVal)
	}

	// Description
	if desc, ok := propMap["description"].(string); ok {
		param.Description = desc
	}

	// Title
	if title, ok := propMap["title"].(string); ok {
		param.Title = title
	}

	// Default value
	if defaultVal, ok := propMap["default"]; ok {
		param.Default = defaultVal
	}

	// Handle enum
	if enumVal, ok := propMap["enum"].([]any); ok {
		for _, e := range enumVal {
			param.Enum = append(param.Enum, fmt.Sprintf("%v", e))
		}
	}

	return param, nil
}

// convertContentToMap converts MCP Content to map[string]any
func convertContentToMap(contents []mcp.Content) map[string]any {
	if len(contents) == 0 {
		return nil
	}

	if len(contents) == 1 {
		if textContent, ok := contents[0].(*mcp.TextContent); ok {
			var v any
			if err := json.Unmarshal([]byte(textContent.Text), &v); err == nil {
				if mapData, ok := v.(map[string]any); ok {
					return mapData
				}
			}
			return map[string]any{
				"result": textContent.Text,
			}
		}
		return nil
	}

	result := map[string]any{}
	for i, c := range contents {
		if textContent, ok := c.(*mcp.TextContent); ok {
			result[fmt.Sprintf("content_%d", i+1)] = textContent.Text
		}
	}
	return result
}
