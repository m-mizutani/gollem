// Package ollama provides a gollem.LLMClient for an Ollama server through its
// native API (/api/chat and /api/embed).
package ollama

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"time"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/goerr/v2"
)

// DefaultBaseURL is the address a local Ollama server listens on.
const DefaultBaseURL = "http://localhost:11434"

// generationParameters holds the generation options sent in "options". A nil
// field is not sent, so the server and model defaults apply.
type generationParameters struct {
	temperature *float64
	topP        *float64
	topK        *int
	maxTokens   *int
	numCtx      *int
}

// Client is a client for an Ollama server.
type Client struct {
	api            *apiClient
	model          string
	embeddingModel string
	systemPrompt   string
	params         generationParameters
	think          *thinkValue
	keepAlive      *time.Duration

	// issuerScope distinguishes this client's provider-bound data from data
	// issued by another client of the same model. See WithIssuerScope.
	issuerScope string

	// Set by options and consumed by New to build api.
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// Option is a function that configures a Client.
type Option func(*Client)

// WithBaseURL sets the address of the Ollama server. Default: DefaultBaseURL.
// Use "https://ollama.com" with WithAPIKey for Ollama Cloud.
func WithBaseURL(baseURL string) Option {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

// WithAPIKey sets the API key sent as "Authorization: Bearer <apiKey>". A
// local Ollama server does not need one; Ollama Cloud does.
func WithAPIKey(apiKey string) Option {
	return func(c *Client) {
		c.apiKey = apiKey
	}
}

// WithHTTPClient sets the HTTP client used for requests. The default client
// has no timeout so that a long streamed response is not cut off; cancel a
// request through its context instead.
func WithHTTPClient(httpClient *http.Client) Option {
	return func(c *Client) {
		c.httpClient = httpClient
	}
}

// WithEmbeddingModel sets the model used by GenerateEmbedding. It has no
// default because only models pulled to the server can be used.
func WithEmbeddingModel(model string) Option {
	return func(c *Client) {
		c.embeddingModel = model
	}
}

// WithSystemPrompt sets the system prompt used when a session does not set
// one with gollem.WithSessionSystemPrompt.
func WithSystemPrompt(prompt string) Option {
	return func(c *Client) {
		c.systemPrompt = prompt
	}
}

// WithTemperature sets the "temperature" generation option.
func WithTemperature(temperature float64) Option {
	return func(c *Client) {
		c.params.temperature = &temperature
	}
}

// WithTopP sets the "top_p" generation option.
func WithTopP(topP float64) Option {
	return func(c *Client) {
		c.params.topP = &topP
	}
}

// WithTopK sets the "top_k" generation option.
func WithTopK(topK int) Option {
	return func(c *Client) {
		c.params.topK = &topK
	}
}

// WithMaxTokens sets the "num_predict" generation option, the maximum number
// of tokens to generate.
func WithMaxTokens(maxTokens int) Option {
	return func(c *Client) {
		c.params.maxTokens = &maxTokens
	}
}

// WithNumCtx sets the "num_ctx" generation option, the context window size in
// tokens. A request whose prompt exceeds it fails with an error tagged
// gollem.ErrTagTokenExceeded.
func WithNumCtx(numCtx int) Option {
	return func(c *Client) {
		c.params.numCtx = &numCtx
	}
}

// WithThink turns thinking on or off for models that support it. Without this
// option or WithThinkLevel the model default applies. The later of the two
// options wins.
func WithThink(enabled bool) Option {
	return func(c *Client) {
		c.think = &thinkValue{enabled: enabled}
	}
}

// WithThinkLevel sets a named thinking level such as "low", "medium", "high"
// or "max". Use a value listed in thinking.values of the server's /api/show
// response for the model. The client does not validate the level; depending on
// the model, the server either rejects an unsupported level or applies the
// model default instead. The later of WithThink and WithThinkLevel wins.
func WithThinkLevel(level string) Option {
	return func(c *Client) {
		c.think = &thinkValue{level: level}
	}
}

// WithKeepAlive sets how long the server keeps the model loaded after a
// request. A negative value keeps it loaded indefinitely.
func WithKeepAlive(d time.Duration) Option {
	return func(c *Client) {
		c.keepAlive = &d
	}
}

// WithIssuerScope sets the scope recorded on the provider-bound data this
// client creates, such as thinking. Thinking is sent back only to a session
// whose client has the same model and the same scope. Set a distinct scope when
// clients of the same model name must not exchange that data, for example
// clients of different servers whose models share a name.
// Default: "" (empty).
func WithIssuerScope(scope string) Option {
	return func(c *Client) {
		c.issuerScope = scope
	}
}

// New creates a client for the Ollama server. model is the name of a model
// pulled to the server, such as "qwen3:8b"; there is no default because no
// model is available on every server.
func New(ctx context.Context, model string, options ...Option) (*Client, error) {
	client := &Client{
		model:      model,
		baseURL:    DefaultBaseURL,
		httpClient: &http.Client{},
	}
	for _, option := range options {
		option(client)
	}

	if client.model == "" {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "model is required")
	}
	if client.httpClient == nil {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "HTTP client must not be nil")
	}
	baseURL, err := url.Parse(client.baseURL)
	if err != nil {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "invalid base URL",
			goerr.V("base_url", client.baseURL), goerr.V("error", err.Error()))
	}
	if (baseURL.Scheme != "http" && baseURL.Scheme != "https") || baseURL.Host == "" {
		return nil, goerr.Wrap(gollem.ErrInvalidParameter, "base URL must be an http or https URL with a host",
			goerr.V("base_url", client.baseURL))
	}

	client.api = &apiClient{
		baseURL:    baseURL,
		apiKey:     client.apiKey,
		httpClient: client.httpClient,
	}
	return client, nil
}

// Model returns the model name this client generates through. It is the name
// the client was configured with, so a caller can key its own tables by the
// same string it passed to New.
func (c *Client) Model() string { return c.model }

// NewSession creates a new session with the Ollama server.
func (c *Client) NewSession(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
	cfg := gollem.NewSessionConfig(options...)

	tools := make([]tool, len(cfg.Tools()))
	for i, t := range cfg.Tools() {
		tools[i] = convertTool(t)
	}

	issuer := gollem.Issuer{Provider: gollem.LLMTypeOllama, Model: c.model, Scope: c.issuerScope}

	historyMessages, err := toMessages(cfg.History(), issuer)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to convert history to Ollama format")
	}

	systemPrompt := cfg.SystemPrompt()
	if systemPrompt == "" {
		systemPrompt = c.systemPrompt
	}

	return &Session{
		api:             c.api,
		model:           c.model,
		systemPrompt:    systemPrompt,
		tools:           tools,
		params:          c.params,
		think:           c.think,
		keepAlive:       c.keepAlive,
		historyMessages: historyMessages,
		issuer:          issuer,
		cfg:             cfg,
	}, nil
}

// Session is a conversation with the Ollama server. It is not safe for
// concurrent use.
type Session struct {
	api *apiClient

	model string
	// systemPrompt is the session system prompt, or the client one when the
	// session sets none. It is prepended to every request and is not stored in
	// the history.
	systemPrompt string
	tools        []tool
	params       generationParameters
	think        *thinkValue
	keepAlive    *time.Duration

	historyMessages []message
	// issuer identifies this session as the issuer of provider-bound data.
	issuer gollem.Issuer
	cfg    gollem.SessionConfig
}

// History returns the conversation history. It does not contain the system
// prompt.
func (s *Session) History() (*gollem.History, error) {
	return newHistory(s.historyMessages, s.issuer)
}

// AppendHistory appends h to the conversation history.
func (s *Session) AppendHistory(h *gollem.History) error {
	if h == nil {
		return nil
	}
	messages, err := toMessages(h, s.issuer)
	if err != nil {
		return goerr.Wrap(err, "failed to convert history to Ollama format")
	}
	s.historyMessages = append(s.historyMessages, messages...)
	return nil
}

// historyForMiddleware returns a copy of the history for the middleware chain,
// or nil when the history is empty.
func (s *Session) historyForMiddleware() (*gollem.History, error) {
	if len(s.historyMessages) == 0 {
		return nil, nil
	}
	h, err := newHistory(s.historyMessages, s.issuer)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create history copy for middleware")
	}
	return h, nil
}

// prepareTurn applies the history returned by the middleware chain and
// converts the inputs. The converted inputs are not added to the history
// until the call succeeds (see commitTurn), so a failed call leaves the
// history as it was and a retry sends the inputs only once.
func (s *Session) prepareTurn(req *gollem.ContentRequest) ([]message, error) {
	if req.History != nil {
		messages, err := toMessages(req.History, s.issuer)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert history from middleware")
		}
		s.historyMessages = messages
	}

	return convertInputs(req.Inputs)
}

// commitTurn adds the inputs of a successful call and the assistant reply to
// the history.
func (s *Session) commitTurn(newMessages []message, assistant message) {
	s.historyMessages = append(s.historyMessages, newMessages...)
	if hasContent(assistant) {
		s.historyMessages = append(s.historyMessages, assistant)
	}
}

// buildRequest builds a chat request from the session state, the inputs of
// this turn and the per-call overrides.
func (s *Session) buildRequest(stream bool, newMessages []message, opts ...gollem.GenerateOption) (*chatRequest, error) {
	genCfg := gollem.NewGenerateConfig(opts...)

	messages := make([]message, 0, len(s.historyMessages)+len(newMessages)+1)
	if s.systemPrompt != "" {
		messages = append(messages, message{Role: roleSystem, Content: s.systemPrompt})
	}
	messages = append(messages, s.historyMessages...)
	messages = append(messages, newMessages...)

	format, err := responseFormat(s.cfg.ContentType(), s.cfg.ResponseSchema(), genCfg.ResponseSchema())
	if err != nil {
		return nil, err
	}

	req := &chatRequest{
		Model:    s.model,
		Messages: messages,
		Format:   format,
		Options:  s.buildOptions(genCfg.MaxTokens()),
		Stream:   stream,
		Think:    s.think,
		Truncate: false,
		Shift:    false,
	}
	// Ollama has no parameter that forbids tool calls, so the definitions are
	// left out of this call instead.
	if !genCfg.ToolCallsDisabled() && len(s.tools) > 0 {
		req.Tools = s.tools
	}
	if s.keepAlive != nil {
		req.KeepAlive = keepAliveJSON(*s.keepAlive)
	}
	return req, nil
}

func (s *Session) buildOptions(maxTokens *int) map[string]any {
	options := make(map[string]any)
	if s.params.temperature != nil {
		options["temperature"] = *s.params.temperature
	}
	if s.params.topP != nil {
		options["top_p"] = *s.params.topP
	}
	if s.params.topK != nil {
		options["top_k"] = *s.params.topK
	}
	if v := pick(maxTokens, s.params.maxTokens); v != nil {
		options["num_predict"] = *v
	}
	if s.params.numCtx != nil {
		options["num_ctx"] = *s.params.numCtx
	}
	if len(options) == 0 {
		return nil
	}
	return options
}

// pick returns override when it is set, otherwise fallback.
func pick[T any](override, fallback *T) *T {
	if override != nil {
		return override
	}
	return fallback
}

// Generate sends the input and returns the whole response.
func (s *Session) Generate(ctx context.Context, input []gollem.Input, opts ...gollem.GenerateOption) (*gollem.Response, error) {
	historyCopy, err := s.historyForMiddleware()
	if err != nil {
		return nil, err
	}

	contentReq := &gollem.ContentRequest{
		Inputs:       input,
		History:      historyCopy,
		SystemPrompt: s.systemPrompt,
	}

	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
		newMessages, err := s.prepareTurn(req)
		if err != nil {
			return nil, err
		}

		chatReq, err := s.buildRequest(false, newMessages, opts...)
		if err != nil {
			return nil, err
		}

		var traceData *trace.LLMCallData
		var llmErr error
		if h := trace.HandlerFrom(ctx); h != nil {
			ctx = h.StartLLMCall(ctx)
			defer func() { h.EndLLMCall(ctx, traceData, llmErr) }()
		}

		resp, err := s.api.chat(ctx, chatReq)
		if err != nil {
			llmErr = err
			return nil, goerr.Wrap(err, "failed to call Ollama chat API",
				append(tokenLimitErrorOptions(err), goerr.V("model", s.model))...)
		}

		calls, assistant, err := convertResponseMessage(resp.Message)
		if err != nil {
			llmErr = err
			return nil, err
		}
		s.commitTurn(newMessages, assistant)

		traceData = buildTraceData(resp.Model, resp.PromptEvalCount, resp.EvalCount, resp.PromptEvalCachedCount,
			s.systemPrompt, newMessages, assistant)

		return &gollem.ContentResponse{
			Texts:               nonEmpty(assistant.Content),
			Thoughts:            nonEmpty(assistant.Thinking),
			FunctionCalls:       calls,
			InputToken:          resp.PromptEvalCount,
			OutputToken:         resp.EvalCount,
			CacheReadInputToken: resp.PromptEvalCachedCount,
			FinishReason:        resp.DoneReason,
		}, nil
	}

	handler := gollem.ContentBlockHandler(baseHandler)
	middlewares := s.cfg.ContentBlockMiddlewares()
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	contentResp, err := handler(ctx, contentReq)
	if err != nil {
		return nil, err
	}

	return &gollem.Response{
		Texts:               contentResp.Texts,
		Thoughts:            contentResp.Thoughts,
		FunctionCalls:       contentResp.FunctionCalls,
		InputToken:          contentResp.InputToken,
		OutputToken:         contentResp.OutputToken,
		CacheReadInputToken: contentResp.CacheReadInputToken,
		FinishReason:        contentResp.FinishReason,
	}, nil
}

// Stream sends the input and returns a channel that yields the response as it
// arrives. Texts and thoughts are sent as they are received; function calls
// and the final token counts are sent after the server finishes. An error,
// including cancellation of ctx, is sent as the last response before the
// channel is closed. Read the channel until it is closed: if the caller stops
// reading, the goroutines producing the responses block and are not released
// even when ctx is cancelled.
func (s *Session) Stream(ctx context.Context, input []gollem.Input, opts ...gollem.GenerateOption) (<-chan *gollem.Response, error) {
	historyCopy, err := s.historyForMiddleware()
	if err != nil {
		return nil, err
	}

	contentReq := &gollem.ContentRequest{
		Inputs:       input,
		History:      historyCopy,
		SystemPrompt: s.systemPrompt,
	}

	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
		newMessages, err := s.prepareTurn(req)
		if err != nil {
			return nil, err
		}

		chatReq, err := s.buildRequest(true, newMessages, opts...)
		if err != nil {
			return nil, err
		}

		traceHandler := trace.HandlerFrom(ctx)
		if traceHandler != nil {
			ctx = traceHandler.StartLLMCall(ctx)
		}

		stream, err := s.api.openChatStream(ctx, chatReq)
		if err != nil {
			if traceHandler != nil {
				traceHandler.EndLLMCall(ctx, nil, err)
			}
			return nil, goerr.Wrap(err, "failed to start Ollama chat stream",
				append(tokenLimitErrorOptions(err), goerr.V("model", s.model))...)
		}

		responseChan := make(chan *gollem.ContentResponse)
		go func() {
			defer close(responseChan)
			// Every chunk and any error have already been sent when this runs,
			// so a Close failure has no effect on what the caller received.
			defer func() { _ = stream.close() }()

			var traceData *trace.LLMCallData
			var streamErr error
			if traceHandler != nil {
				defer func() { traceHandler.EndLLMCall(ctx, traceData, streamErr) }()
			}

			fail := func(err error) {
				streamErr = err
				responseChan <- &gollem.ContentResponse{Error: err}
			}

			received := message{Role: roleAssistant}
			var inputTokens, outputTokens, cachedTokens int
			var doneReason string

			for {
				select {
				case <-ctx.Done():
					fail(goerr.Wrap(ctx.Err(), "context cancelled during streaming"))
					return
				default:
				}

				chunk, err := stream.recv()
				if errors.Is(err, io.EOF) {
					break
				}
				if err != nil {
					// A cancelled request surfaces as a read error on the body.
					if ctx.Err() != nil {
						fail(goerr.Wrap(ctx.Err(), "context cancelled during streaming"))
						return
					}
					fail(goerr.Wrap(err, "failed to receive Ollama chat stream",
						append(tokenLimitErrorOptions(err), goerr.V("model", s.model))...))
					return
				}

				if chunk.Message.Thinking != "" {
					received.Thinking += chunk.Message.Thinking
					responseChan <- &gollem.ContentResponse{Thoughts: []string{chunk.Message.Thinking}}
				}
				if chunk.Message.Content != "" {
					received.Content += chunk.Message.Content
					responseChan <- &gollem.ContentResponse{Texts: []string{chunk.Message.Content}}
				}
				// The server parses tool calls before sending them, so each
				// chunk carries complete calls.
				received.ToolCalls = append(received.ToolCalls, chunk.Message.ToolCalls...)
				if chunk.Done {
					inputTokens = chunk.PromptEvalCount
					outputTokens = chunk.EvalCount
					cachedTokens = chunk.PromptEvalCachedCount
					doneReason = chunk.DoneReason
				}
			}

			calls, assistant, err := convertResponseMessage(received)
			if err != nil {
				fail(err)
				return
			}
			if len(calls) > 0 {
				responseChan <- &gollem.ContentResponse{FunctionCalls: calls}
			}
			// The final chunk carries both the token counts and done_reason, so
			// they are sent together after the function calls it completed.
			if inputTokens > 0 || outputTokens > 0 || doneReason != "" {
				responseChan <- &gollem.ContentResponse{
					InputToken:          inputTokens,
					OutputToken:         outputTokens,
					CacheReadInputToken: cachedTokens,
					FinishReason:        doneReason,
				}
			}
			s.commitTurn(newMessages, assistant)

			traceData = buildTraceData(s.model, inputTokens, outputTokens, cachedTokens,
				s.systemPrompt, newMessages, assistant)
		}()

		return responseChan, nil
	}

	handler := gollem.ContentStreamHandler(baseHandler)
	middlewares := s.cfg.ContentStreamMiddlewares()
	for i := len(middlewares) - 1; i >= 0; i-- {
		handler = middlewares[i](handler)
	}

	streamChan, err := handler(ctx, contentReq)
	if err != nil {
		return nil, err
	}
	if streamChan == nil {
		return nil, goerr.New("middleware returned nil channel without error")
	}

	responseChan := make(chan *gollem.Response)
	go func() {
		defer close(responseChan)
		for streamResp := range streamChan {
			if streamResp.Error != nil {
				responseChan <- &gollem.Response{Error: streamResp.Error}
				continue
			}
			responseChan <- &gollem.Response{
				Texts:               streamResp.Texts,
				Thoughts:            streamResp.Thoughts,
				FunctionCalls:       streamResp.FunctionCalls,
				InputToken:          streamResp.InputToken,
				OutputToken:         streamResp.OutputToken,
				CacheReadInputToken: streamResp.CacheReadInputToken,
				FinishReason:        streamResp.FinishReason,
			}
		}
	}()

	return responseChan, nil
}

// CountToken is not supported: Ollama has no API that counts tokens, and the
// tokenizer differs by model, so a local estimate would not match the count
// the server applies against num_ctx. It always returns an error wrapping
// gollem.ErrUnsupportedOperation.
func (s *Session) CountToken(ctx context.Context, input ...gollem.Input) (int, error) {
	return 0, goerr.Wrap(gollem.ErrUnsupportedOperation, "Ollama has no API to count tokens")
}

func hasContent(m message) bool {
	return m.Content != "" || m.Thinking != "" || len(m.ToolCalls) > 0
}

func nonEmpty(s string) []string {
	if s == "" {
		return []string{}
	}
	return []string{s}
}

// exceedContextSizeErrorType is the error type reported when the prompt does
// not fit in num_ctx and truncation is disabled. The server returns HTTP 400
// with the runner's JSON error as the "error" string, for example:
//
//	{"error":"{\"error\":{\"code\":400,\"message\":\"request (4021 tokens) exceeds the available context size (512 tokens), try increasing it\",\"type\":\"exceed_context_size_error\",...}}"}
//
// The type is matched instead of the message because the message embeds the
// token counts.
const exceedContextSizeErrorType = "exceed_context_size_error"

// tokenLimitErrorOptions returns the option that tags err with
// gollem.ErrTagTokenExceeded when err reports that the prompt exceeds the
// context size, and nil otherwise.
func tokenLimitErrorOptions(err error) []goerr.Option {
	var apiErr *apiError
	if !errors.As(err, &apiErr) || apiErr.StatusCode != http.StatusBadRequest {
		return nil
	}

	var runnerErr struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal([]byte(apiErr.Message), &runnerErr) != nil {
		// Not the runner's JSON error, so it cannot be the overflow error.
		return nil
	}
	if runnerErr.Error.Type != exceedContextSizeErrorType {
		return nil
	}
	return []goerr.Option{goerr.Tag(gollem.ErrTagTokenExceeded)}
}

// buildTraceData builds the trace record of one chat call. messages are the
// messages added in this turn; earlier turns are recorded by earlier spans.
func buildTraceData(model string, inputTokens, outputTokens, cachedTokens int, systemPrompt string, messages []message, assistant message) *trace.LLMCallData {
	data := &trace.LLMCallData{
		InputTokens:          inputTokens,
		OutputTokens:         outputTokens,
		Model:                model,
		CacheReadInputTokens: cachedTokens,
		Request: &trace.LLMRequest{
			SystemPrompt: systemPrompt,
			Messages:     messagesToTraceMessages(messages),
		},
		Response: &trace.LLMResponse{},
	}
	if assistant.Content != "" {
		data.Response.Texts = []string{assistant.Content}
	}
	for _, call := range assistant.ToolCalls {
		data.Response.FunctionCalls = append(data.Response.FunctionCalls, &trace.FunctionCall{
			ID:        call.ID,
			Name:      call.Function.Name,
			Arguments: traceArguments(call.Function.Arguments),
		})
	}
	return data
}

// traceArguments decodes tool call arguments for the trace. Trace data must
// not fail the request that produced it, so arguments that are not a JSON
// object are recorded verbatim.
func traceArguments(raw json.RawMessage) map[string]any {
	args, err := decodeArguments(raw)
	if err != nil {
		return map[string]any{rawArgumentsKey: string(raw)}
	}
	return args
}

func messagesToTraceMessages(messages []message) []trace.Message {
	var result []trace.Message
	for _, msg := range messages {
		var blocks []trace.MessageContent
		if msg.Role == roleTool {
			block := trace.NewToolResponseContent(msg.ToolCallID, msg.ToolName, nil)
			block.Text = msg.Content
			blocks = append(blocks, block)
		} else {
			if msg.Thinking != "" {
				blocks = append(blocks, trace.NewThinkingContent(msg.Thinking))
			}
			if msg.Content != "" {
				blocks = append(blocks, trace.NewTextContent(msg.Content))
			}
			for range msg.Images {
				blocks = append(blocks, trace.NewMediaContent("image", ""))
			}
			for _, call := range msg.ToolCalls {
				blocks = append(blocks, trace.NewToolCallContent(call.ID, call.Function.Name, traceArguments(call.Function.Arguments)))
			}
		}
		if len(blocks) > 0 {
			result = append(result, trace.Message{Role: msg.Role, Contents: blocks})
		}
	}
	return result
}
