package openai_test

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/llm/openai"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/gt"
)

func newCaptureTestSession(t *testing.T, handler http.HandlerFunc) gollem.Session {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)

	client, err := openai.New(context.Background(), "test-key", openai.WithBaseURL(srv.URL))
	gt.NoError(t, err).Required()
	session, err := client.NewSession(context.Background())
	gt.NoError(t, err).Required()
	return session
}

func writeBody(t *testing.T, w io.Writer, s string) {
	t.Helper()
	_, err := io.WriteString(w, s)
	gt.NoError(t, err)
}

func chatCompletionBody(usage string) string {
	return `{"id":"chatcmpl-1","object":"chat.completion","model":"gpt-test",` +
		`"choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}],` +
		`"usage":` + usage + `}`
}

func findLLMCallSpan(t *testing.T, root *trace.Span) *trace.Span {
	t.Helper()
	for _, child := range root.Children {
		if child.Kind == trace.SpanKindLLMCall {
			return child
		}
	}
	t.Fatal("llm_call span not found")
	return nil
}

func TestGenerateReportsCacheWriteTokens(t *testing.T) {
	type testCase struct {
		usage           string
		wantInput       int
		wantCacheRead   int
		wantCacheCreate int
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			session := newCaptureTestSession(t, func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				writeBody(t, w, chatCompletionBody(tc.usage))
			})

			rec := trace.New()
			ctx := rec.StartAgentExecute(context.Background())
			ctx = trace.WithHandler(ctx, rec)

			resp, err := session.Generate(ctx, []gollem.Input{gollem.Text("hi")})
			gt.NoError(t, err).Required()
			rec.EndAgentExecute(ctx, nil)

			gt.Equal(t, []string{"ok"}, resp.Texts)
			gt.Equal(t, tc.wantInput, resp.InputToken)
			gt.Equal(t, tc.wantCacheRead, resp.CacheReadInputToken)
			gt.Equal(t, tc.wantCacheCreate, resp.CacheCreationInputToken)

			span := findLLMCallSpan(t, rec.Trace().RootSpan)
			gt.Equal(t, tc.wantInput, span.LLMCall.InputTokens)
			gt.Equal(t, tc.wantCacheRead, span.LLMCall.CacheReadInputTokens)
			gt.Equal(t, tc.wantCacheCreate, span.LLMCall.CacheCreationInputTokens)
		}
	}

	t.Run("cache reads and writes", runTest(testCase{
		usage: `{"prompt_tokens":1000,"completion_tokens":10,"total_tokens":1010,` +
			`"prompt_tokens_details":{"cached_tokens":200,"cache_write_tokens":300}}`,
		wantInput:       1000,
		wantCacheRead:   200,
		wantCacheCreate: 300,
	}))

	t.Run("no cache_write_tokens field", runTest(testCase{
		usage: `{"prompt_tokens":1000,"completion_tokens":10,"total_tokens":1010,` +
			`"prompt_tokens_details":{"cached_tokens":200}}`,
		wantInput:     1000,
		wantCacheRead: 200,
	}))

	t.Run("null cache_write_tokens", runTest(testCase{
		usage: `{"prompt_tokens":1000,"completion_tokens":10,"total_tokens":1010,` +
			`"prompt_tokens_details":{"cached_tokens":200,"cache_write_tokens":null}}`,
		wantInput:     1000,
		wantCacheRead: 200,
	}))

	t.Run("no prompt_tokens_details", runTest(testCase{
		usage:     `{"prompt_tokens":1000,"completion_tokens":10,"total_tokens":1010}`,
		wantInput: 1000,
	}))
}

func TestGenerateCacheWriteTokensErrors(t *testing.T) {
	type testCase struct {
		handler     http.HandlerFunc
		errContains string
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			session := newCaptureTestSession(t, tc.handler)
			resp, err := session.Generate(context.Background(), []gollem.Input{gollem.Text("hi")})
			gt.Error(t, err).Required()
			gt.V(t, resp).Nil()
			gt.S(t, err.Error()).Contains(tc.errContains)
		}
	}

	t.Run("API error body is decoded by go-openai", runTest(testCase{
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusBadRequest)
			writeBody(t, w, `{"error":{"message":"bad request from test","type":"invalid_request_error"}}`)
		},
		errContains: "bad request from test",
	}))

	t.Run("cache_write_tokens is not an integer", runTest(testCase{
		handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			writeBody(t, w, chatCompletionBody(`{"prompt_tokens":1000,"completion_tokens":10,"total_tokens":1010,`+
				`"prompt_tokens_details":{"cached_tokens":200,"cache_write_tokens":"300"}}`))
		},
		errContains: "cache_write_tokens",
	}))

	t.Run("body ends before Content-Length after a complete JSON value", runTest(testCase{
		handler: func(w http.ResponseWriter, r *http.Request) {
			body := chatCompletionBody(`{"prompt_tokens":1000,"completion_tokens":10,"total_tokens":1010,` +
				`"prompt_tokens_details":{"cached_tokens":200,"cache_write_tokens":300}}`)
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Content-Length", strconv.Itoa(len(body)+100))
			writeBody(t, w, body)
		},
		errContains: "failed to read chat completion response body",
	}))
}

// sseChunk returns one chat.completion.chunk event with a single choice.
func sseChunk(delta, finish string) string {
	if finish == "" {
		finish = "null"
	} else {
		finish = `"` + finish + `"`
	}
	return `data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-test",` +
		`"choices":[{"index":0,"delta":` + delta + `,"finish_reason":` + finish + `}],"usage":null}` + "\n\n"
}

// sseUsageChunk returns the trailing usage event sent when
// stream_options.include_usage is set.
func sseUsageChunk(promptTokensDetails string) string {
	return `data: {"id":"c1","object":"chat.completion.chunk","model":"gpt-test","choices":[],` +
		`"usage":{"prompt_tokens":1000,"completion_tokens":10,"total_tokens":1010,` +
		`"prompt_tokens_details":` + promptTokensDetails + `}}` + "\n\n"
}

func TestStreamReportsCacheWriteTokens(t *testing.T) {
	var includeUsage atomic.Bool
	session := newCaptureTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			StreamOptions struct {
				IncludeUsage bool `json:"include_usage"`
			} `json:"stream_options"`
		}
		gt.NoError(t, json.NewDecoder(r.Body).Decode(&req))
		includeUsage.Store(req.StreamOptions.IncludeUsage)

		w.Header().Set("Content-Type", "text/event-stream")
		flusher := w.(http.Flusher)
		write := func(s string) {
			writeBody(t, w, s)
			flusher.Flush()
		}

		write(sseChunk(`{"role":"assistant","content":"Hel"}`, ""))
		write(sseChunk(`{"content":"lo"}`, ""))
		write(sseChunk(`{"tool_calls":[{"index":0,"id":"call_1","type":"function","function":{"name":"lookup","arguments":"{\"q\":"}}]}`, ""))
		write(sseChunk(`{"tool_calls":[{"index":0,"function":{"arguments":"\"x\"}"}}]}`, ""))
		write(sseChunk(`{}`, "tool_calls"))

		// Flush the usage event one byte at a time so the capturing reader
		// has to join an event that arrives across several reads.
		usage := sseUsageChunk(`{"cached_tokens":200,"cache_write_tokens":300}`)
		for i := range len(usage) {
			write(usage[i : i+1])
		}
		write("data: [DONE]\n\n")
	})

	rec := trace.New()
	ctx := rec.StartAgentExecute(context.Background())
	ctx = trace.WithHandler(ctx, rec)

	ch, err := session.Stream(ctx, []gollem.Input{gollem.Text("hi")})
	gt.NoError(t, err).Required()

	var texts []string
	var calls []*gollem.FunctionCall
	var last *gollem.Response
	for resp := range ch {
		gt.NoError(t, resp.Error).Required()
		texts = append(texts, resp.Texts...)
		calls = append(calls, resp.FunctionCalls...)
		last = resp
	}
	rec.EndAgentExecute(ctx, nil)

	gt.True(t, includeUsage.Load())
	gt.Equal(t, []string{"Hel", "lo"}, texts)
	gt.A(t, calls).Length(1).Required()
	gt.Equal(t, "call_1", calls[0].ID)
	gt.Equal(t, "lookup", calls[0].Name)
	gt.Equal(t, map[string]any{"q": "x"}, calls[0].Arguments)

	gt.V(t, last).NotNil().Required()
	gt.Equal(t, 1000, last.InputToken)
	gt.Equal(t, 200, last.CacheReadInputToken)
	gt.Equal(t, 300, last.CacheCreationInputToken)

	span := findLLMCallSpan(t, rec.Trace().RootSpan)
	gt.Equal(t, 300, span.LLMCall.CacheCreationInputTokens)
	gt.Equal(t, 200, span.LLMCall.CacheReadInputTokens)
}

func TestStreamCacheWriteTokensNotInteger(t *testing.T) {
	session := newCaptureTestSession(t, func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		writeBody(t, w, sseChunk(`{"role":"assistant","content":"ok"}`, "stop"))
		writeBody(t, w, sseUsageChunk(`{"cached_tokens":200,"cache_write_tokens":"300"}`))
		writeBody(t, w, "data: [DONE]\n\n")
	})

	ch, err := session.Stream(context.Background(), []gollem.Input{gollem.Text("hi")})
	gt.NoError(t, err).Required()

	var texts []string
	var streamErr error
	for resp := range ch {
		if resp.Error != nil {
			streamErr = resp.Error
			continue
		}
		gt.Equal(t, 0, resp.CacheCreationInputToken)
		texts = append(texts, resp.Texts...)
	}

	gt.Equal(t, []string{"ok"}, texts)
	gt.Error(t, streamErr).Required()
	gt.S(t, streamErr.Error()).Contains("cache_write_tokens")
}

var callNumberPattern = regexp.MustCompile(`n=(\d+)`)

func TestConcurrentSessionsReportOwnCacheWriteTokens(t *testing.T) {
	// The server derives cache_write_tokens from the user message so that each
	// request has a distinct, predictable count.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		gt.NoError(t, err)
		// Each session sends its whole history, so the last match is the
		// message of this request.
		matches := callNumberPattern.FindAllSubmatch(body, -1)
		gt.A(t, matches).Longer(0).Required()
		n, err := strconv.Atoi(string(matches[len(matches)-1][1]))
		gt.NoError(t, err)

		w.Header().Set("Content-Type", "application/json")
		writeBody(t, w, chatCompletionBody(fmt.Sprintf(
			`{"prompt_tokens":5000,"completion_tokens":1,"total_tokens":5001,"prompt_tokens_details":{"cached_tokens":0,"cache_write_tokens":%d}}`, n)))
	}))
	t.Cleanup(srv.Close)

	client, err := openai.New(context.Background(), "test-key", openai.WithBaseURL(srv.URL))
	gt.NoError(t, err).Required()

	const sessions = 2
	const callsPerSession = 20
	var wg sync.WaitGroup
	for s := range sessions {
		session, err := client.NewSession(context.Background())
		gt.NoError(t, err).Required()

		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range callsPerSession {
				want := (s+1)*1000 + i
				resp, err := session.Generate(context.Background(), []gollem.Input{gollem.Text(fmt.Sprintf("n=%d", want))})
				gt.NoError(t, err)
				if resp != nil {
					gt.Equal(t, want, resp.CacheCreationInputToken)
				}
			}
		}()
	}
	wg.Wait()
}

// TestCacheWriteTokensLive checks against the real API that a cache write is
// reported on the first call, a cache read on the second, and that
// prompt_tokens counts both.
func TestCacheWriteTokensLive(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
	if !ok {
		t.Skip("TEST_OPENAI_API_KEY is not set")
	}
	model := "gpt-5.6"
	if v, ok := os.LookupEnv("TEST_OPENAI_CACHE_WRITE_MODEL"); ok {
		model = v
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*testTimeout)
	defer cancel()

	client, err := openai.New(ctx, apiKey, openai.WithModel(model))
	gt.NoError(t, err).Required()

	// A random prefix keeps the prompt out of any cache left by earlier runs,
	// and the repeated text keeps it above the 1024-token caching threshold.
	// The text is sent as the user input because the OpenAI session does not
	// send the session system prompt in the request.
	nonce := make([]byte, 16)
	_, err = rand.Read(nonce)
	gt.NoError(t, err).Required()
	prompt := "Session " + hex.EncodeToString(nonce) + ".\n" +
		strings.Repeat("You are a careful assistant who answers in a single word. ", 300) +
		"\nSay hello."

	generate := func() *gollem.Response {
		session, err := client.NewSession(ctx)
		gt.NoError(t, err).Required()
		resp, err := session.Generate(ctx, []gollem.Input{gollem.Text(prompt)}, gollem.WithMaxTokens(maxTestTokens))
		gt.NoError(t, err).Required()
		return resp
	}

	first := generate()
	second := generate()
	t.Logf("first: input=%d write=%d read=%d", first.InputToken, first.CacheCreationInputToken, first.CacheReadInputToken)
	t.Logf("second: input=%d write=%d read=%d", second.InputToken, second.CacheCreationInputToken, second.CacheReadInputToken)

	gt.True(t, first.CacheCreationInputToken > 0)
	gt.True(t, second.CacheReadInputToken > 0)
	for _, resp := range []*gollem.Response{first, second} {
		gt.True(t, resp.InputToken >= resp.CacheReadInputToken+resp.CacheCreationInputToken)
	}
}
