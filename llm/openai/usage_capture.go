package openai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"

	"github.com/m-mizutani/goerr/v2"
	"github.com/sashabaranov/go-openai"
)

// usageCapture receives the usage fields that go-openai drops while decoding a
// chat completion response. go-openai's PromptTokensDetails has no
// cache_write_tokens field, so the count is read from the raw response body by
// usageCaptureDoer. One capture belongs to exactly one API call.
type usageCapture struct {
	stream bool

	mu               sync.Mutex
	cacheWriteTokens int
	err              error
}

func (c *usageCapture) setCacheWriteTokens(n int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cacheWriteTokens = n
}

func (c *usageCapture) setErr(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.err == nil {
		c.err = err
	}
}

// result returns the last cache_write_tokens seen and the first error met
// while reading it.
func (c *usageCapture) result() (int, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.cacheWriteTokens, c.err
}

type ctxUsageCaptureKey struct{}

// withUsageCapture returns a context that makes usageCaptureDoer record the
// usage of the request sent with it into a new capture.
func withUsageCapture(ctx context.Context, stream bool) (context.Context, *usageCapture) {
	capture := &usageCapture{stream: stream}
	return context.WithValue(ctx, ctxUsageCaptureKey{}, capture), capture
}

func usageCaptureFrom(ctx context.Context) *usageCapture {
	capture, _ := ctx.Value(ctxUsageCaptureKey{}).(*usageCapture)
	return capture
}

// parseCacheWriteTokens extracts usage.prompt_tokens_details.cache_write_tokens
// from a chat completion response or stream chunk. ok is false when the
// payload has no such field.
//
// Only a present cache_write_tokens that is not an integer is an error.
// Payloads that are not JSON, or whose enclosing usage objects are malformed,
// are reported as absent: go-openai decodes the same payload into its own
// structs and returns those errors to the caller, while it never looks at
// cache_write_tokens and so cannot detect a malformed value there.
func parseCacheWriteTokens(payload []byte) (n int, ok bool, err error) {
	var body struct {
		Usage *struct {
			PromptTokensDetails *struct {
				CacheWriteTokens json.RawMessage `json:"cache_write_tokens"`
			} `json:"prompt_tokens_details"`
		} `json:"usage"`
	}
	if json.Unmarshal(payload, &body) != nil {
		return 0, false, nil
	}
	if body.Usage == nil || body.Usage.PromptTokensDetails == nil {
		return 0, false, nil
	}
	raw := body.Usage.PromptTokensDetails.CacheWriteTokens
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, false, nil
	}
	if err := json.Unmarshal(raw, &n); err != nil {
		return 0, false, goerr.Wrap(err, "failed to decode usage.prompt_tokens_details.cache_write_tokens",
			goerr.V("cache_write_tokens", string(raw)))
	}
	return n, true, nil
}

// usageCaptureDoer delegates to the configured HTTP client and, for requests
// whose context carries a usageCapture, reads cache_write_tokens from the
// response body. The body handed to go-openai has the same bytes as the body
// received, so go-openai decodes the response as it would without this doer.
type usageCaptureDoer struct {
	next openai.HTTPDoer
}

func (d *usageCaptureDoer) Do(req *http.Request) (*http.Response, error) {
	resp, err := d.next.Do(req)
	if err != nil || resp == nil || resp.Body == nil {
		return resp, err
	}

	capture := usageCaptureFrom(req.Context())
	if capture == nil {
		return resp, nil
	}

	if capture.stream {
		resp.Body = &streamUsageReader{body: resp.Body, capture: capture}
		return resp, nil
	}

	// go-openai decodes a single JSON value and stops reading, so it would not
	// see a read error that follows a complete value. Report it here instead
	// of returning a response whose usage may be incomplete.
	body, err := io.ReadAll(resp.Body)
	if closeErr := resp.Body.Close(); err == nil && closeErr != nil {
		err = goerr.Wrap(closeErr, "failed to close chat completion response body")
	}
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read chat completion response body",
			goerr.V("status", resp.StatusCode), goerr.V("read_bytes", len(body)))
	}

	n, ok, err := parseCacheWriteTokens(body)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read cache write tokens", goerr.V("status", resp.StatusCode))
	}
	if ok {
		capture.setCacheWriteTokens(n)
	}

	resp.Body = io.NopCloser(bytes.NewReader(body))
	return resp, nil
}

// streamUsageReader passes a server-sent events body through unchanged and
// records cache_write_tokens from every "data:" event that carries usage. The
// last value seen wins, matching how the stream reports usage in its final
// chunk. A malformed value is stored in the capture for the session to report
// once the stream ends.
type streamUsageReader struct {
	body    io.ReadCloser
	capture *usageCapture
	line    []byte
}

func (r *streamUsageReader) Read(p []byte) (int, error) {
	n, err := r.body.Read(p)
	r.scan(p[:n])
	if err == io.EOF {
		// A final event without a trailing newline is still a complete event.
		r.processLine(r.line)
		r.line = nil
	}
	return n, err
}

func (r *streamUsageReader) Close() error {
	return r.body.Close()
}

func (r *streamUsageReader) scan(chunk []byte) {
	for len(chunk) > 0 {
		i := bytes.IndexByte(chunk, '\n')
		if i < 0 {
			r.line = append(r.line, chunk...)
			return
		}
		if len(r.line) == 0 {
			r.processLine(chunk[:i])
		} else {
			r.line = append(r.line, chunk[:i]...)
			r.processLine(r.line)
			r.line = r.line[:0]
		}
		chunk = chunk[i+1:]
	}
}

var (
	sseDataPrefix = []byte("data:")
	usageKey      = []byte(`"usage"`)
)

func (r *streamUsageReader) processLine(line []byte) {
	payload, ok := bytes.CutPrefix(bytes.TrimSpace(line), sseDataPrefix)
	if !ok || !bytes.Contains(payload, usageKey) {
		return
	}
	n, ok, err := parseCacheWriteTokens(bytes.TrimSpace(payload))
	if err != nil {
		r.capture.setErr(err)
		return
	}
	if ok {
		r.capture.setCacheWriteTokens(n)
	}
}
