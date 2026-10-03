package ollama

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/m-mizutani/goerr/v2"
)

const (
	chatPath  = "/api/chat"
	embedPath = "/api/embed"

	// maxErrorBodySize bounds how much of an error response body is read, so a
	// misbehaving server cannot make the client buffer an unbounded body.
	maxErrorBodySize = 1 << 20

	contentTypeJSON   = "application/json"
	contentTypeNDJSON = "application/x-ndjson"
)

// chatRequest is the body of POST /api/chat.
type chatRequest struct {
	Model     string          `json:"model"`
	Messages  []message       `json:"messages"`
	Tools     []tool          `json:"tools,omitempty"`
	Format    json.RawMessage `json:"format,omitempty"`
	Options   map[string]any  `json:"options,omitempty"`
	Stream    bool            `json:"stream"`
	Think     *thinkValue     `json:"think,omitempty"`
	KeepAlive json.RawMessage `json:"keep_alive,omitempty"`
	// Truncate and Shift are always false. The server defaults both to true,
	// which drops or shifts the oldest messages without reporting it; sending
	// false makes an overflow fail so it can be tagged as a token limit error.
	Truncate bool `json:"truncate"`
	Shift    bool `json:"shift"`
}

// message is a chat message in the Ollama wire format.
type message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`
	Thinking   string     `json:"thinking,omitempty"`
	Images     []string   `json:"images,omitempty"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolName   string     `json:"tool_name,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string           `json:"id,omitempty"`
	Function toolCallFunction `json:"function"`
}

type toolCallFunction struct {
	Index     int             `json:"index"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string         `json:"name"`
	Description string         `json:"description,omitempty"`
	Parameters  map[string]any `json:"parameters"`
}

// chatResponse is a whole response, or one chunk of a streamed response.
type chatResponse struct {
	Model      string  `json:"model"`
	Message    message `json:"message"`
	Done       bool    `json:"done"`
	DoneReason string  `json:"done_reason,omitempty"`
	// PromptEvalCount includes the prompt tokens served from the server's KV
	// cache, which PromptEvalCachedCount reports separately.
	PromptEvalCount       int `json:"prompt_eval_count,omitempty"`
	PromptEvalCachedCount int `json:"prompt_eval_cached_count,omitempty"`
	EvalCount             int `json:"eval_count,omitempty"`
	// Error and Status are set on an error line inside a stream.
	Error  string `json:"error,omitempty"`
	Status int    `json:"status,omitempty"`
}

type embedRequest struct {
	Model      string   `json:"model"`
	Input      []string `json:"input"`
	Dimensions int      `json:"dimensions,omitempty"`
}

type embedResponse struct {
	Model           string      `json:"model"`
	Embeddings      [][]float64 `json:"embeddings"`
	PromptEvalCount int         `json:"prompt_eval_count,omitempty"`
}

// thinkValue is the "think" request field. It marshals to a JSON string when
// level is set, otherwise to a JSON boolean.
type thinkValue struct {
	enabled bool
	level   string
}

func (t thinkValue) MarshalJSON() ([]byte, error) {
	if t.level != "" {
		return json.Marshal(t.level)
	}
	return json.Marshal(t.enabled)
}

// keepAliveJSON encodes d the way the server's Duration type decodes it: a
// negative number keeps the model loaded indefinitely, a string is parsed by
// time.ParseDuration.
func keepAliveJSON(d time.Duration) json.RawMessage {
	if d < 0 {
		return json.RawMessage("-1")
	}
	return json.RawMessage(`"` + d.String() + `"`)
}

// apiError is an error returned by the Ollama server, either as a non-2xx
// response or as an error line inside a stream.
type apiError struct {
	// StatusCode is the HTTP status. It is 0 for an error line inside a stream
	// that carries no status.
	StatusCode int
	// Message is the "error" field of the body, or the raw body when it is not
	// JSON.
	Message string
}

func (e *apiError) Error() string {
	return fmt.Sprintf("ollama API error: status=%d, message=%s", e.StatusCode, e.Message)
}

// apiClient sends requests to an Ollama server.
type apiClient struct {
	baseURL    *url.URL
	apiKey     string
	httpClient *http.Client
}

func (c *apiClient) chat(ctx context.Context, req *chatRequest) (*chatResponse, error) {
	resp, err := c.post(ctx, chatPath, req, contentTypeJSON)
	if err != nil {
		return nil, err
	}
	// The result is decided by the decoded body or the decode error; a Close
	// failure only means the connection is not reused, so it is ignored.
	defer func() { _ = resp.Body.Close() }()

	var out chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, goerr.Wrap(err, "failed to decode chat response")
	}
	if out.Error != "" {
		return nil, &apiError{StatusCode: resp.StatusCode, Message: out.Error}
	}
	return &out, nil
}

func (c *apiClient) openChatStream(ctx context.Context, req *chatRequest) (*chatStream, error) {
	resp, err := c.post(ctx, chatPath, req, contentTypeNDJSON)
	if err != nil {
		return nil, err
	}
	return &chatStream{
		body:    resp.Body,
		decoder: json.NewDecoder(resp.Body),
	}, nil
}

func (c *apiClient) embed(ctx context.Context, req *embedRequest) (*embedResponse, error) {
	resp, err := c.post(ctx, embedPath, req, contentTypeJSON)
	if err != nil {
		return nil, err
	}
	// Same as chat: a Close failure does not change the decoded result.
	defer func() { _ = resp.Body.Close() }()

	var out embedResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, goerr.Wrap(err, "failed to decode embed response")
	}
	return &out, nil
}

// post sends body as JSON to path. On a 2xx status it returns the response,
// whose body the caller must close. Otherwise it closes the body and returns
// an *apiError.
func (c *apiClient) post(ctx context.Context, path string, body any, accept string) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to marshal request", goerr.V("path", path))
	}

	endpoint := c.baseURL.JoinPath(path).String()
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(data))
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create request", goerr.V("endpoint", endpoint))
	}
	httpReq.Header.Set("Content-Type", contentTypeJSON)
	httpReq.Header.Set("Accept", accept)
	if c.apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to send request", goerr.V("endpoint", endpoint))
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		// This path always returns the API error, which is what the caller
		// needs; a Close failure would only hide it.
		defer func() { _ = resp.Body.Close() }()
		apiErr, err := readAPIError(resp)
		if err != nil {
			return nil, goerr.Wrap(err, "failed to read error response",
				goerr.V("endpoint", endpoint), goerr.V("status", resp.StatusCode))
		}
		return nil, goerr.Wrap(apiErr, "ollama API returned an error status",
			goerr.V("endpoint", endpoint), goerr.V("status", resp.StatusCode))
	}

	return resp, nil
}

func readAPIError(resp *http.Response) (*apiError, error) {
	raw, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBodySize))
	if err != nil {
		return nil, err
	}

	var body struct {
		Error string `json:"error"`
	}
	if err := json.Unmarshal(raw, &body); err == nil && body.Error != "" {
		return &apiError{StatusCode: resp.StatusCode, Message: body.Error}, nil
	}
	// The body is not the documented {"error": "..."} shape (for example an
	// HTML page from a proxy), so its text is the only description available.
	return &apiError{StatusCode: resp.StatusCode, Message: strings.TrimSpace(string(raw))}, nil
}

// chatStream reads the newline-delimited JSON objects of a streamed chat
// response.
type chatStream struct {
	body    io.ReadCloser
	decoder *json.Decoder
	done    bool
}

// errStreamEndedEarly is returned when the body ends before the chunk that has
// done set to true.
var errStreamEndedEarly = errors.New("stream ended before the done chunk")

// recv returns the next chunk. After the done chunk it returns io.EOF. It
// returns an *apiError for an error line and errStreamEndedEarly when the body
// ends before the done chunk.
func (s *chatStream) recv() (*chatResponse, error) {
	if s.done {
		return nil, io.EOF
	}

	var chunk chatResponse
	if err := s.decoder.Decode(&chunk); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, goerr.Wrap(errStreamEndedEarly, "failed to read chat stream")
		}
		return nil, goerr.Wrap(err, "failed to decode chat stream chunk")
	}
	if chunk.Error != "" {
		return nil, &apiError{StatusCode: chunk.Status, Message: chunk.Error}
	}
	if chunk.Done {
		s.done = true
	}
	return &chunk, nil
}

func (s *chatStream) close() error {
	return s.body.Close()
}
