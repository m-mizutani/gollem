package openai

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"sort"
	"strings"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/internal/convert"
	"github.com/gollem-dev/gollem/internal/jsonutil"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// Responses API item and content type names.
const (
	responseItemMessage            = "message"
	responseItemReasoning          = "reasoning"
	responseItemFunctionCall       = "function_call"
	responseItemFunctionCallOutput = "function_call_output"

	responseContentInputText  = "input_text"
	responseContentInputImage = "input_image"
	responseContentInputFile  = "input_file"
	responseContentOutputText = "output_text"
	responseContentRefusal    = "refusal"
	responseSummaryText       = "summary_text"
)

// responsesSession is a gollem.Session that calls the Responses API. It keeps
// the conversation in gollem's message format and sends all of it on every
// call (store=false), so a History saved from the session or rewritten by a
// middleware is the complete state of the conversation.
type responsesSession struct {
	apiClient responsesAPI
	model     string
	// systemPrompt is sent as instructions unless a middleware replaces it.
	systemPrompt string
	params       generationParameters
	cfg          gollem.SessionConfig
	// tools are kept in the Chat Completions form, which local token counting
	// reads, and converted with openai.NewResponseFunctionTool for each request.
	tools []openai.Tool
	// messages holds only content that FilterProviderData kept for issuer and
	// content this session created, so all of its provider-bound data was
	// issued by issuer.
	messages []gollem.Message
	// issuer identifies this session as the issuer of provider-bound data.
	issuer gollem.Issuer
}

// reasoningMeta is stored in MessageContent.Provider.Data of a thinking content
// that came from a Responses API reasoning item. A stateless request must send every
// reasoning item back with its ID and encrypted content, so these fields are
// what lets a restored History continue a reasoning conversation.
type reasoningMeta struct {
	ReasoningID      string   `json:"openai_reasoning_id"`
	EncryptedContent string   `json:"openai_encrypted_content,omitempty"`
	Summary          []string `json:"openai_reasoning_summary,omitempty"`
}

// messageMeta is stored in MessageContent.Provider.Data of a text content that
// came from a Responses API message item. The API asks for the phase of an assistant
// message ("commentary" or "final_answer") to be sent back unchanged, so that
// the model does not read commentary written before a tool call as a final
// answer.
type messageMeta struct {
	Phase string `json:"openai_phase,omitempty"`
}

// responseOutputItem adds the fields the SDK's ResponseOutputItem does not
// decode: the reasoning item's encrypted_content and the message item's phase.
type responseOutputItem struct {
	openai.ResponseOutputItem
	EncryptedContent string `json:"encrypted_content,omitempty"`
	Phase            string `json:"phase,omitempty"`
}

type responseReasoningItem struct {
	Type             string                       `json:"type"`
	ID               string                       `json:"id"`
	Summary          []openai.ResponseSummaryPart `json:"summary"`
	EncryptedContent string                       `json:"encrypted_content,omitempty"`
}

type responseFunctionCallItem struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// responseTurn is the result of one Responses API call.
type responseTurn struct {
	texts         []string
	thoughts      []string
	functionCalls []*gollem.FunctionCall
	// message is the assistant message to append to the history. It has no
	// contents when the response produced no output items.
	message gollem.Message
}

// responseUsage is the token usage of one call in gollem's terms.
type responseUsage struct {
	input      int
	output     int
	cacheRead  int
	cacheWrite int
}

func newResponsesSession(apiClient responsesAPI, model, clientSystemPrompt string, params generationParameters, cfg gollem.SessionConfig, issuer gollem.Issuer) (*responsesSession, error) {
	systemPrompt := cfg.SystemPrompt()
	if systemPrompt == "" {
		systemPrompt = clientSystemPrompt
	}

	tools := make([]openai.Tool, len(cfg.Tools()))
	for i, tool := range cfg.Tools() {
		tools[i] = convertTool(tool)
	}

	var messages []gollem.Message
	if h := cfg.History(); h != nil {
		messages = gollem.FilterProviderData(h.Clone().Messages, issuer)
		if _, err := toResponseInput(messages); err != nil {
			return nil, goerr.Wrap(err, "failed to convert history to Responses API input")
		}
	}

	return &responsesSession{
		apiClient:    apiClient,
		model:        model,
		systemPrompt: systemPrompt,
		params:       params,
		cfg:          cfg,
		tools:        tools,
		messages:     messages,
		issuer:       issuer,
	}, nil
}

// History returns a copy of the conversation, including the reasoning items
// the next request has to send back.
func (s *responsesSession) History() (*gollem.History, error) {
	h := &gollem.History{
		LLType:   gollem.LLMTypeOpenAI,
		Version:  gollem.HistoryVersion,
		Messages: s.messages,
	}
	clone := h.Clone()
	if clone.Messages == nil {
		clone.Messages = []gollem.Message{}
	}
	return clone, nil
}

func (s *responsesSession) AppendHistory(h *gollem.History) error {
	if h == nil {
		return nil
	}
	messages := gollem.FilterProviderData(h.Clone().Messages, s.issuer)
	if _, err := toResponseInput(messages); err != nil {
		return goerr.Wrap(err, "failed to convert history to Responses API input")
	}
	s.messages = append(s.messages, messages...)
	return nil
}

// contentRequest builds the request that the middleware chain receives.
func (s *responsesSession) contentRequest(input []gollem.Input) (*gollem.ContentRequest, error) {
	var historyCopy *gollem.History
	if len(s.messages) > 0 {
		h, err := s.History()
		if err != nil {
			return nil, err
		}
		historyCopy = h
	}
	return &gollem.ContentRequest{
		Inputs:       input,
		History:      historyCopy,
		SystemPrompt: s.systemPrompt,
	}, nil
}

// startTurn applies the history a middleware may have rewritten, appends the
// new inputs, and returns the messages added for this turn.
func (s *responsesSession) startTurn(req *gollem.ContentRequest) ([]gollem.Message, error) {
	if req.History != nil {
		s.messages = gollem.FilterProviderData(req.History.Clone().Messages, s.issuer)
	}
	newMessages, err := inputsToMessages(req.Inputs)
	if err != nil {
		return nil, err
	}
	s.messages = append(s.messages, newMessages...)
	return newMessages, nil
}

func (s *responsesSession) buildRequest(systemPrompt string, opts ...gollem.GenerateOption) (openai.CreateResponseRequest, error) {
	input, err := toResponseInput(s.messages)
	if err != nil {
		return openai.CreateResponseRequest{}, goerr.Wrap(err, "failed to convert history to Responses API input")
	}

	store := false
	req := openai.CreateResponseRequest{
		Model:           s.model,
		Input:           input,
		Instructions:    systemPrompt,
		Store:           &store,
		Include:         []openai.ResponseInclude{openai.ResponseIncludeReasoningEncryptedContent},
		MaxOutputTokens: s.params.MaxTokens,
	}
	if s.params.Temperature != 0 {
		temperature := s.params.Temperature
		req.Temperature = &temperature
	}
	if s.params.TopP != 0 {
		topP := s.params.TopP
		req.TopP = &topP
	}
	if s.params.ReasoningEffort != "" {
		req.Reasoning = &openai.ResponseReasoning{Effort: s.params.ReasoningEffort}
	}
	for _, tool := range s.tools {
		responseTool := openai.NewResponseFunctionTool(*tool.Function)
		// The Responses API treats a function tool without "strict" as strict,
		// which makes every optional parameter required. The tool schemas are
		// written for non-strict use, as on Chat Completions, so strict is sent
		// as false explicitly; NewResponseFunctionTool omits a false value.
		responseTool.Parameters["strict"] = false
		req.Tools = append(req.Tools, responseTool)
	}

	var format *openai.ResponseTextFormat
	if s.cfg.ContentType() == gollem.ContentTypeJSON {
		if s.cfg.ResponseSchema() != nil {
			format, err = responseSchemaFormat(s.cfg.ResponseSchema())
			if err != nil {
				return openai.CreateResponseRequest{}, goerr.Wrap(err, "failed to convert response schema")
			}
		} else {
			format = &openai.ResponseTextFormat{Type: "json_object"}
		}
	}

	genCfg := gollem.NewGenerateConfig(opts...)
	if m := genCfg.MaxTokens(); m != nil {
		req.MaxOutputTokens = *m
	}
	if schema := genCfg.ResponseSchema(); schema != nil {
		format, err = responseSchemaFormat(schema)
		if err != nil {
			return openai.CreateResponseRequest{}, goerr.Wrap(err, "failed to convert per-call response schema")
		}
	}
	// The tool definitions stay in the request; "none" only forbids calling
	// them. Without tools there is nothing to forbid, so tool_choice is not sent.
	if genCfg.ToolCallsDisabled() && len(req.Tools) > 0 {
		req.ToolChoice = "none"
	}

	if format != nil || s.params.Verbosity != "" {
		req.Text = &openai.ResponseTextConfig{Format: format, Verbosity: s.params.Verbosity}
	}

	return req, nil
}

// responseSchemaFormat converts a response schema to the Responses API
// text.format.
func responseSchemaFormat(param *gollem.Parameter) (*openai.ResponseTextFormat, error) {
	jsonSchema, err := convertResponseSchemaToOpenAI(param, false)
	if err != nil {
		return nil, err
	}
	return &openai.ResponseTextFormat{
		Type:        "json_schema",
		Name:        jsonSchema.Name,
		Description: jsonSchema.Description,
		Schema:      jsonSchema.Schema,
		Strict:      jsonSchema.Strict,
	}, nil
}

// Generate sends the conversation and the inputs to the Responses API.
func (s *responsesSession) Generate(ctx context.Context, input []gollem.Input, opts ...gollem.GenerateOption) (*gollem.Response, error) {
	contentReq, err := s.contentRequest(input)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create history copy for middleware")
	}

	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
		newMessages, err := s.startTurn(req)
		if err != nil {
			return nil, err
		}

		apiReq, err := s.buildRequest(req.SystemPrompt, opts...)
		if err != nil {
			return nil, err
		}

		var traceData *trace.LLMCallData
		var llmErr error
		if h := trace.HandlerFrom(ctx); h != nil {
			ctx = h.StartLLMCall(ctx)
			defer func() { h.EndLLMCall(ctx, traceData, llmErr) }()
		}

		resp, err := s.apiClient.CreateResponse(ctx, apiReq)
		if err != nil {
			llmErr = err
			return nil, goerr.Wrap(err, "failed to create response", tokenLimitErrorOptions(err)...)
		}
		if resp.Error != nil {
			llmErr = goerr.New("response failed",
				goerr.V("code", resp.Error.Code),
				goerr.V("message", resp.Error.Message),
				goerr.V("response_id", resp.ID))
			return nil, llmErr
		}

		items, err := decodeOutputItems(resp.Output)
		if err != nil {
			llmErr = err
			return nil, err
		}
		turn, err := outputItemsToTurn(items, s.issuer)
		if err != nil {
			llmErr = err
			return nil, err
		}
		if len(turn.message.Contents) > 0 {
			s.messages = append(s.messages, turn.message)
		}

		usage := usageFromResponse(resp.Usage)
		traceData = buildResponsesTraceData(resp.Model, req.SystemPrompt, newMessages, turn, usage)

		return &gollem.ContentResponse{
			Texts:                   turn.texts,
			Thoughts:                turn.thoughts,
			FunctionCalls:           turn.functionCalls,
			InputToken:              usage.input,
			OutputToken:             usage.output,
			CacheCreationInputToken: usage.cacheWrite,
			CacheReadInputToken:     usage.cacheRead,
		}, nil
	}

	handler := gollem.BuildContentBlockChain(s.cfg.ContentBlockMiddlewares(), baseHandler)
	contentResp, err := handler(ctx, contentReq)
	if err != nil {
		return nil, err
	}

	return &gollem.Response{
		Texts:                   contentResp.Texts,
		Thoughts:                contentResp.Thoughts,
		FunctionCalls:           contentResp.FunctionCalls,
		InputToken:              contentResp.InputToken,
		OutputToken:             contentResp.OutputToken,
		CacheCreationInputToken: contentResp.CacheCreationInputToken,
		CacheReadInputToken:     contentResp.CacheReadInputToken,
	}, nil
}

// Stream sends the conversation and the inputs to the Responses API and
// delivers text and reasoning summary deltas as they arrive. Function calls and
// the token usage are delivered after the response completes.
func (s *responsesSession) Stream(ctx context.Context, input []gollem.Input, opts ...gollem.GenerateOption) (<-chan *gollem.Response, error) {
	contentReq, err := s.contentRequest(input)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create history copy for middleware")
	}

	baseHandler := func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
		newMessages, err := s.startTurn(req)
		if err != nil {
			return nil, err
		}

		apiReq, err := s.buildRequest(req.SystemPrompt, opts...)
		if err != nil {
			return nil, err
		}

		traceHandler := trace.HandlerFrom(ctx)
		if traceHandler != nil {
			ctx = traceHandler.StartLLMCall(ctx)
		}

		stream, err := s.apiClient.CreateResponseStream(ctx, apiReq)
		if err != nil {
			if traceHandler != nil {
				traceHandler.EndLLMCall(ctx, nil, err)
			}
			return nil, goerr.Wrap(err, "failed to create response stream", tokenLimitErrorOptions(err)...)
		}

		responseChan := make(chan *gollem.ContentResponse)
		go func() {
			defer close(responseChan)

			var traceData *trace.LLMCallData
			var streamErr error
			if traceHandler != nil {
				defer func() { traceHandler.EndLLMCall(ctx, traceData, streamErr) }()
			}
			// The body has been read to the end or abandoned at this point, so a
			// failure to close it cannot change the result already delivered.
			defer func() { _ = stream.Close() }()

			send := func(resp *gollem.ContentResponse) bool {
				return sendOrDone(ctx, responseChan, resp)
			}
			fail := func(err error) {
				streamErr = err
				send(&gollem.ContentResponse{Error: err})
			}

			completed, items, err := s.readStream(ctx, stream, send)
			if err != nil {
				fail(err)
				return
			}

			turn, err := outputItemsToTurn(items, s.issuer)
			if err != nil {
				fail(err)
				return
			}
			if len(turn.message.Contents) > 0 {
				s.messages = append(s.messages, turn.message)
			}

			usage := usageFromResponse(completed.Usage)
			traceData = buildResponsesTraceData(completed.Model, req.SystemPrompt, newMessages, turn, usage)

			if len(turn.functionCalls) > 0 {
				if !send(&gollem.ContentResponse{
					FunctionCalls:           turn.functionCalls,
					InputToken:              usage.input,
					OutputToken:             usage.output,
					CacheCreationInputToken: usage.cacheWrite,
					CacheReadInputToken:     usage.cacheRead,
				}) {
					return
				}
			}
			send(&gollem.ContentResponse{
				InputToken:              usage.input,
				OutputToken:             usage.output,
				CacheCreationInputToken: usage.cacheWrite,
				CacheReadInputToken:     usage.cacheRead,
			})
		}()

		return responseChan, nil
	}

	handler := gollem.BuildContentStreamChain(s.cfg.ContentStreamMiddlewares(), baseHandler)
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
			resp := &gollem.Response{Error: streamResp.Error}
			if streamResp.Error == nil {
				resp = &gollem.Response{
					Texts:                   streamResp.Texts,
					Thoughts:                streamResp.Thoughts,
					FunctionCalls:           streamResp.FunctionCalls,
					InputToken:              streamResp.InputToken,
					OutputToken:             streamResp.OutputToken,
					CacheCreationInputToken: streamResp.CacheCreationInputToken,
					CacheReadInputToken:     streamResp.CacheReadInputToken,
				}
			}
			if !sendOrDone(ctx, responseChan, resp) {
				// The caller stopped reading. Drain the middleware channel so
				// that the goroutines writing to it can finish.
				for range streamChan {
				}
				return
			}
		}
	}()

	return responseChan, nil
}

// sendOrDone sends v on ch unless ctx is done first. It returns false when v
// was not sent. A receiver that is already waiting gets v even if ctx is done,
// so a caller that cancels while still reading receives the cancellation error.
func sendOrDone[T any](ctx context.Context, ch chan<- T, v T) bool {
	select {
	case ch <- v:
		return true
	default:
	}
	select {
	case ch <- v:
		return true
	case <-ctx.Done():
		return false
	}
}

// readStream passes text, refusal and reasoning summary deltas to send and
// collects the finished output items until the response completes. It returns
// the final response object, which carries the model and the usage.
func (s *responsesSession) readStream(ctx context.Context, stream *openai.ResponseStream, send func(*gollem.ContentResponse) bool) (*openai.CreateResponseResponse, []responseOutputItem, error) {
	itemsByIndex := make(map[int]responseOutputItem)
	errCancelled := func() error {
		return goerr.Wrap(ctx.Err(), "context cancelled during streaming")
	}

	for {
		if ctx.Err() != nil {
			return nil, nil, errCancelled()
		}

		event, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			return nil, nil, goerr.New("response stream ended before the response completed")
		}
		if err != nil {
			if ctx.Err() != nil {
				return nil, nil, errCancelled()
			}
			return nil, nil, goerr.Wrap(err, "failed to receive response stream", tokenLimitErrorOptions(err)...)
		}

		switch event.Type {
		case openai.ResponseStreamEventOutputTextDelta, openai.ResponseStreamEventRefusalDelta:
			if event.Delta != "" && !send(&gollem.ContentResponse{Texts: []string{event.Delta}}) {
				return nil, nil, errCancelled()
			}

		case openai.ResponseStreamEventReasoningSummaryTextDelta:
			if event.Delta != "" && !send(&gollem.ContentResponse{Thoughts: []string{event.Delta}}) {
				return nil, nil, errCancelled()
			}

		case openai.ResponseStreamEventOutputItemDone:
			var done struct {
				Item responseOutputItem `json:"item"`
			}
			if err := json.Unmarshal(event.Raw, &done); err != nil {
				return nil, nil, goerr.Wrap(err, "failed to decode output item",
					goerr.V("output_index", event.OutputIndex))
			}
			itemsByIndex[event.OutputIndex] = done.Item

		case openai.ResponseStreamEventCompleted, openai.ResponseStreamEventIncomplete:
			if event.Response == nil {
				return nil, nil, goerr.New("response stream event has no response", goerr.V("type", event.Type))
			}
			indexes := make([]int, 0, len(itemsByIndex))
			for i := range itemsByIndex {
				indexes = append(indexes, i)
			}
			sort.Ints(indexes)
			items := make([]responseOutputItem, 0, len(indexes))
			for _, i := range indexes {
				items = append(items, itemsByIndex[i])
			}
			return event.Response, items, nil

		case openai.ResponseStreamEventFailed:
			opts := []goerr.Option{goerr.V("type", event.Type)}
			if event.Response != nil {
				opts = append(opts, goerr.V("response_id", event.Response.ID))
				if event.Response.Error != nil {
					opts = append(opts,
						goerr.V("code", event.Response.Error.Code),
						goerr.V("message", event.Response.Error.Message))
				}
			}
			return nil, nil, goerr.New("response failed", opts...)

		case openai.ResponseStreamEventError:
			return nil, nil, goerr.New("response stream error",
				goerr.V("code", event.Code),
				goerr.V("message", event.Message))
		}
	}
}

// CountToken estimates the input tokens locally with tiktoken, counting the
// conversation as the equivalent Chat Completions messages.
func (s *responsesSession) CountToken(ctx context.Context, input ...gollem.Input) (int, error) {
	newMessages, err := inputsToMessages(input)
	if err != nil {
		return 0, goerr.Wrap(err, "failed to convert inputs for token counting")
	}
	messages := make([]gollem.Message, 0, len(s.messages)+len(newMessages))
	messages = append(messages, s.messages...)
	messages = append(messages, newMessages...)

	chatMessages, err := convertMessagesToOpenAI(messages)
	if err != nil {
		return 0, goerr.Wrap(err, "failed to convert messages for token counting")
	}
	return countChatTokens(s.model, s.systemPrompt, chatMessages, s.tools)
}

// inputsToMessages converts the inputs of one call to gollem messages.
// Consecutive text, image and PDF inputs form one user message; every function
// response is a tool message of its own.
func inputsToMessages(inputs []gollem.Input) ([]gollem.Message, error) {
	var messages []gollem.Message
	var userContents []gollem.MessageContent

	flushUser := func() {
		if len(userContents) > 0 {
			messages = append(messages, gollem.Message{Role: gollem.RoleUser, Contents: userContents})
			userContents = nil
		}
	}

	for _, in := range inputs {
		var content gollem.MessageContent
		var err error
		switch v := in.(type) {
		case gollem.Text:
			content, err = gollem.NewTextContent(string(v))
		case gollem.Image:
			content, err = gollem.NewImageContent(v.MimeType(), v.Data(), "", "")
		case gollem.PDF:
			content, err = gollem.NewPDFContent(v.Data(), "")
		case gollem.FunctionResponse:
			flushUser()
			response := v.Data
			if response == nil {
				response = map[string]any{}
			}
			if v.Error != nil {
				response = map[string]any{"error": v.Error.Error()}
			}
			mc, err := gollem.NewToolResponseContent(v.ID, v.Name, response, v.Error != nil)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to convert function response", goerr.V("call_id", v.ID))
			}
			messages = append(messages, gollem.Message{Role: gollem.RoleTool, Contents: []gollem.MessageContent{mc}})
			continue
		default:
			return nil, goerr.Wrap(gollem.ErrInvalidParameter, "invalid input")
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to convert input")
		}
		userContents = append(userContents, content)
	}
	flushUser()

	return messages, nil
}

// toResponseInput converts gollem messages to Responses API input items. The
// messages must have been filtered by gollem.FilterProviderData for the
// session's issuer.
//
// A thinking content without reasoningMeta (reasoning text that Chat
// Completions produced) is not sent: the Responses API accepts reasoning only
// as a reasoning item with the ID it issued.
func toResponseInput(messages []gollem.Message) ([]any, error) {
	items := make([]any, 0, len(messages))

	for _, msg := range messages {
		role := "user"
		if msg.Role == gollem.RoleSystem {
			role = "system"
		}

		// Text, image and PDF contents of a non-assistant message are collected
		// into one message item, which is emitted before the next item of
		// another type so that the order of the history is kept.
		var parts []any
		flushParts := func() {
			if len(parts) > 0 {
				items = append(items, openai.ResponseInputMessage{
					Type:    responseItemMessage,
					Role:    role,
					Content: parts,
				})
				parts = nil
			}
		}

		for i := range msg.Contents {
			content := &msg.Contents[i]
			switch content.Type {
			case gollem.MessageContentTypeText:
				text, err := content.GetTextContent()
				if err != nil {
					return nil, goerr.Wrap(err, "failed to get text content")
				}
				if msg.Role == gollem.RoleAssistant {
					meta, err := unmarshalMessageMeta(content.Provider)
					if err != nil {
						return nil, err
					}
					items = append(items, openai.ResponseInputMessage{
						Type:    responseItemMessage,
						Role:    "assistant",
						Content: text.Text,
						Phase:   meta.Phase,
					})
					continue
				}
				parts = append(parts, openai.ResponseInputText{Type: responseContentInputText, Text: text.Text})

			case gollem.MessageContentTypeImage, gollem.MessageContentTypePDF:
				if msg.Role == gollem.RoleAssistant {
					return nil, goerr.Wrap(gollem.ErrInvalidHistoryData,
						"the Responses API does not accept images or PDFs in assistant messages",
						goerr.V("content_type", content.Type))
				}
				part, err := mediaInputPart(content)
				if err != nil {
					return nil, err
				}
				parts = append(parts, part)

			case gollem.MessageContentTypeThinking:
				flushParts()
				meta, err := unmarshalReasoningMeta(content.Provider)
				if err != nil {
					return nil, err
				}
				if meta.ReasoningID == "" {
					continue
				}
				summary := make([]openai.ResponseSummaryPart, 0, len(meta.Summary))
				for _, text := range meta.Summary {
					summary = append(summary, openai.ResponseSummaryPart{Type: responseSummaryText, Text: text})
				}
				items = append(items, responseReasoningItem{
					Type:             responseItemReasoning,
					ID:               meta.ReasoningID,
					Summary:          summary,
					EncryptedContent: meta.EncryptedContent,
				})

			case gollem.MessageContentTypeToolCall:
				flushParts()
				call, err := content.GetToolCallContent()
				if err != nil {
					return nil, goerr.Wrap(err, "failed to get tool call content")
				}
				args, err := convert.StringifyJSONArguments(call.Arguments)
				if err != nil {
					return nil, goerr.Wrap(err, "failed to encode tool call arguments", goerr.V("call_id", call.ID))
				}
				items = append(items, responseFunctionCallItem{
					Type:      responseItemFunctionCall,
					CallID:    call.ID,
					Name:      call.Name,
					Arguments: args,
				})

			case gollem.MessageContentTypeToolResponse:
				flushParts()
				resp, err := content.GetToolResponseContent()
				if err != nil {
					return nil, goerr.Wrap(err, "failed to get tool response content")
				}
				output, err := convert.StringifyJSONArguments(resp.Response)
				if err != nil {
					return nil, goerr.Wrap(err, "failed to encode tool response", goerr.V("call_id", resp.ToolCallID))
				}
				items = append(items, openai.ResponseFunctionCallOutput{
					Type:   responseItemFunctionCallOutput,
					CallID: resp.ToolCallID,
					Output: output,
				})

			default:
				return nil, goerr.Wrap(gollem.ErrInvalidHistoryData, "unsupported content type",
					goerr.V("content_type", content.Type))
			}
		}
		flushParts()
	}

	return items, nil
}

// mediaInputPart converts an image or PDF content to an input content part.
func mediaInputPart(content *gollem.MessageContent) (any, error) {
	if content.Type == gollem.MessageContentTypeImage {
		img, err := content.GetImageContent()
		if err != nil {
			return nil, goerr.Wrap(err, "failed to get image content")
		}
		url := img.URL
		if len(img.Data) > 0 {
			url = "data:" + img.MediaType + ";base64," + base64.StdEncoding.EncodeToString(img.Data)
		}
		detail := img.Detail
		if detail == "" {
			detail = "auto"
		}
		return openai.ResponseInputImage{Type: responseContentInputImage, ImageURL: url, Detail: detail}, nil
	}

	pdf, err := content.GetPDFContent()
	if err != nil {
		return nil, goerr.Wrap(err, "failed to get PDF content")
	}
	if len(pdf.Data) > 0 {
		return openai.ResponseInputFile{
			Type:     responseContentInputFile,
			Filename: "document.pdf",
			FileData: "data:application/pdf;base64," + base64.StdEncoding.EncodeToString(pdf.Data),
		}, nil
	}
	return openai.ResponseInputFile{Type: responseContentInputFile, FileURL: pdf.URL}, nil
}

func unmarshalMessageMeta(provider *gollem.ProviderData) (messageMeta, error) {
	if provider == nil || len(provider.Data) == 0 {
		return messageMeta{}, nil
	}
	var m messageMeta
	if err := json.Unmarshal(provider.Data, &m); err != nil {
		return messageMeta{}, goerr.Wrap(err, "failed to unmarshal message meta")
	}
	return m, nil
}

func unmarshalReasoningMeta(provider *gollem.ProviderData) (reasoningMeta, error) {
	if provider == nil || len(provider.Data) == 0 {
		return reasoningMeta{}, nil
	}
	var m reasoningMeta
	if err := json.Unmarshal(provider.Data, &m); err != nil {
		return reasoningMeta{}, goerr.Wrap(err, "failed to unmarshal reasoning meta")
	}
	return m, nil
}

// decodeOutputItems decodes the output items of a response. The SDK leaves them
// as []any so that it can carry item types it does not model.
func decodeOutputItems(output []any) ([]responseOutputItem, error) {
	if len(output) == 0 {
		return nil, nil
	}
	data, err := json.Marshal(output)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to encode response output")
	}
	var items []responseOutputItem
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, goerr.Wrap(err, "failed to decode response output")
	}
	return items, nil
}

// outputItemsToTurn converts the output items of a response to the values
// returned to the caller and to the assistant message stored in the history.
// Reasoning items and message phases are recorded as issued by issuer.
//
// Only function tools are sent, so the items are reasoning, message and
// function_call. Other item types (built-in tool calls) are not expected and
// are neither returned nor stored.
func outputItemsToTurn(items []responseOutputItem, issuer gollem.Issuer) (*responseTurn, error) {
	turn := &responseTurn{
		texts:         []string{},
		thoughts:      []string{},
		functionCalls: []*gollem.FunctionCall{},
		message:       gollem.Message{Role: gollem.RoleAssistant},
	}

	for _, item := range items {
		switch item.Type {
		case responseItemReasoning:
			summary := make([]string, 0, len(item.Summary))
			for _, part := range item.Summary {
				summary = append(summary, part.Text)
				if part.Text != "" {
					turn.thoughts = append(turn.thoughts, part.Text)
				}
			}
			mc, err := gollem.NewThinkingContent(strings.Join(summary, "\n\n"))
			if err != nil {
				return nil, goerr.Wrap(err, "failed to create thinking content")
			}
			meta, err := json.Marshal(reasoningMeta{
				ReasoningID:      item.ID,
				EncryptedContent: item.EncryptedContent,
				Summary:          summary,
			})
			if err != nil {
				return nil, goerr.Wrap(err, "failed to marshal reasoning meta", goerr.V("reasoning_id", item.ID))
			}
			mc.Provider = &gollem.ProviderData{Issuer: issuer, Data: meta}
			turn.message.Contents = append(turn.message.Contents, mc)

		case responseItemMessage:
			for _, part := range item.Content {
				var text string
				switch part.Type {
				case responseContentOutputText:
					text = part.Text
				case responseContentRefusal:
					text = part.Refusal
				}
				if text == "" {
					continue
				}
				turn.texts = append(turn.texts, text)
				mc, err := gollem.NewTextContent(text)
				if err != nil {
					return nil, goerr.Wrap(err, "failed to create text content")
				}
				if item.Phase != "" {
					meta, err := json.Marshal(messageMeta{Phase: item.Phase})
					if err != nil {
						return nil, goerr.Wrap(err, "failed to marshal message meta", goerr.V("phase", item.Phase))
					}
					mc.Provider = &gollem.ProviderData{Issuer: issuer, Data: meta}
				}
				turn.message.Contents = append(turn.message.Contents, mc)
			}

		case responseItemFunctionCall:
			args := map[string]any{}
			if item.Arguments != "" {
				decoded, err := jsonutil.DecodeObject([]byte(item.Arguments))
				if err != nil {
					return nil, goerr.Wrap(err, "failed to unmarshal function call arguments",
						goerr.V("call_id", item.CallID), goerr.V("name", item.Name))
				}
				args = decoded
			}
			turn.functionCalls = append(turn.functionCalls, &gollem.FunctionCall{
				ID:        item.CallID,
				Name:      item.Name,
				Arguments: args,
			})
			mc, err := gollem.NewToolCallContent(item.CallID, item.Name, args)
			if err != nil {
				return nil, goerr.Wrap(err, "failed to create tool call content", goerr.V("call_id", item.CallID))
			}
			turn.message.Contents = append(turn.message.Contents, mc)
		}
	}

	return turn, nil
}

// usageFromResponse maps the Responses API usage to gollem's token fields.
// input_tokens is the total input including cache reads and writes.
func usageFromResponse(u *openai.ResponseUsage) responseUsage {
	if u == nil {
		return responseUsage{}
	}
	usage := responseUsage{
		input:  u.InputTokens,
		output: u.OutputTokens,
	}
	if u.InputTokensDetails != nil {
		usage.cacheRead = u.InputTokensDetails.CachedTokens
		usage.cacheWrite = u.InputTokensDetails.CacheWriteTokens
	}
	return usage
}

// buildResponsesTraceData records the messages added in this turn, not the
// whole conversation sent to the API; earlier turns are in earlier spans.
func buildResponsesTraceData(model, systemPrompt string, newMessages []gollem.Message, turn *responseTurn, usage responseUsage) *trace.LLMCallData {
	data := &trace.LLMCallData{
		InputTokens:              usage.input,
		OutputTokens:             usage.output,
		Model:                    model,
		CacheCreationInputTokens: usage.cacheWrite,
		CacheReadInputTokens:     usage.cacheRead,
		Request: &trace.LLMRequest{
			SystemPrompt: systemPrompt,
			Messages:     messagesToTraceMessages(newMessages),
		},
		Response: &trace.LLMResponse{},
	}
	if len(turn.texts) > 0 {
		data.Response.Texts = append(data.Response.Texts, turn.texts...)
	}
	for _, fc := range turn.functionCalls {
		data.Response.FunctionCalls = append(data.Response.FunctionCalls, &trace.FunctionCall{
			ID:        fc.ID,
			Name:      fc.Name,
			Arguments: fc.Arguments,
		})
	}
	return data
}

// messagesToTraceMessages converts gollem messages to trace messages. Trace data
// is diagnostic, so a content that cannot be decoded is left out instead of
// failing the request.
func messagesToTraceMessages(messages []gollem.Message) []trace.Message {
	var result []trace.Message
	for _, msg := range messages {
		var blocks []trace.MessageContent
		for i := range msg.Contents {
			content := &msg.Contents[i]
			switch content.Type {
			case gollem.MessageContentTypeText:
				if text, err := content.GetTextContent(); err == nil {
					blocks = append(blocks, trace.NewTextContent(text.Text))
				}
			case gollem.MessageContentTypeThinking:
				if thinking, err := content.GetThinkingContent(); err == nil {
					blocks = append(blocks, trace.NewThinkingContent(thinking.Text))
				}
			case gollem.MessageContentTypeImage:
				if img, err := content.GetImageContent(); err == nil {
					mc := trace.NewMediaContent("image", img.MediaType)
					mc.URL = img.URL
					blocks = append(blocks, mc)
				}
			case gollem.MessageContentTypePDF:
				if pdf, err := content.GetPDFContent(); err == nil {
					mc := trace.NewMediaContent("document", "application/pdf")
					mc.URL = pdf.URL
					blocks = append(blocks, mc)
				}
			case gollem.MessageContentTypeToolCall:
				if call, err := content.GetToolCallContent(); err == nil {
					blocks = append(blocks, trace.NewToolCallContent(call.ID, call.Name, call.Arguments))
				}
			case gollem.MessageContentTypeToolResponse:
				if resp, err := content.GetToolResponseContent(); err == nil {
					blocks = append(blocks, trace.NewToolResponseContent(resp.ToolCallID, resp.Name, resp.Response))
				}
			}
		}
		if len(blocks) > 0 {
			result = append(result, trace.Message{Role: string(msg.Role), Contents: blocks})
		}
	}
	return result
}
