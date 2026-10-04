package gollem_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/llm/claude"
	"github.com/gollem-dev/gollem/llm/gemini"
	"github.com/gollem-dev/gollem/llm/ollama"
	"github.com/gollem-dev/gollem/llm/openai"
	"github.com/gollem-dev/gollem/mock"
	"github.com/gollem-dev/gollem/trace"
	"github.com/m-mizutani/gt"
)

const (
	testTimeout   = 30 * time.Second
	maxTestTokens = 2048
)

// TestToolExecution tests tool execution with real LLM clients
func TestToolExecution(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		client, err := newClient(t)
		gt.NoError(t, err)

		rec := trace.New()
		agent := gollem.New(client,
			gollem.WithTools(&RandomNumberTool{}),
			gollem.WithLoopLimit(5),
			gollem.WithTrace(rec),
		)

		fmt.Printf("[TEST] Agent created: agent=%p\n", agent)

		_, err = agent.Execute(ctx, gollem.Text("Generate a random number between 1 and 100."))
		gt.NoError(t, err)

		// Verify Agent.Session().History() works correctly
		session := agent.Session()
		fmt.Printf("[TEST] Session after Execute: session=%p, type=%T\n", session, session)
		gt.NotNil(t, session)

		history, err := session.History()
		gt.NoError(t, err)
		gt.NotNil(t, history)
		fmt.Printf("[TEST] History after Execute: messages=%d\n", len(history.Messages))
		gt.True(t, len(history.Messages) >= 2)

		// Verify LLM call traces are recorded
		traceData := rec.Trace()
		gt.NotNil(t, traceData)
		gt.NotNil(t, traceData.RootSpan)

		llmCallCount := countSpansByKind(traceData.RootSpan, trace.SpanKindLLMCall)
		gt.N(t, llmCallCount).Greater(0)
		fmt.Printf("[TEST] LLM call spans recorded: %d\n", llmCallCount)
	}

	t.Run("Ollama", func(t *testing.T) {
		t.Parallel()
		testFn(t, newOllamaClient)
	})

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		var opts []gemini.Option
		if model := os.Getenv("TEST_GCP_MODEL"); model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location, opts...)
		})
	})
}

// TestContentMiddleware tests content middleware functionality with real LLM clients
func TestContentMiddleware(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		userName := "Quetzalcoatl"

		// Content middleware that injects fake history before the first call
		contentMiddleware := func(next gollem.ContentBlockHandler) gollem.ContentBlockHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (*gollem.ContentResponse, error) {
				if req.History == nil || len(req.History.Messages) == 0 {
					userData, _ := json.Marshal(map[string]string{
						"text": fmt.Sprintf("My name is %s.", userName),
					})
					assistantData, _ := json.Marshal(map[string]string{
						"text": "Got it.",
					})

					fakeHistory := []gollem.Message{
						{Role: gollem.RoleUser, Contents: []gollem.MessageContent{{Type: gollem.MessageContentTypeText, Data: userData}}},
						{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{{Type: gollem.MessageContentTypeText, Data: assistantData}}},
					}

					if req.History == nil {
						req.History = &gollem.History{Version: gollem.HistoryVersion, Messages: fakeHistory}
					} else {
						req.History.Messages = append(fakeHistory, req.History.Messages...)
					}
				}
				return next(ctx, req)
			}
		}

		client, err := newClient(t)
		gt.NoError(t, err)

		session, err := client.NewSession(ctx,
			gollem.WithSessionContentBlockMiddleware(contentMiddleware),
		)
		gt.NoError(t, err)

		// Single call: ask about the injected name (1 API call)
		resp, err := session.Generate(ctx, []gollem.Input{gollem.Text("What is my name?")}, gollem.WithMaxTokens(maxTestTokens))
		gt.NoError(t, err)
		gt.True(t, len(resp.Texts) > 0)

		combined := strings.Join(resp.Texts, " ")
		if !strings.Contains(combined, userName) {
			t.Fatalf("response should mention %s, got: %s", userName, combined)
		}
	}

	t.Run("Ollama", func(t *testing.T) {
		t.Parallel()
		testFn(t, newOllamaClient)
	})

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		var opts []gemini.Option
		if model := os.Getenv("TEST_GCP_MODEL"); model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location, opts...)
		})
	})
}

// TestStreamMiddleware tests streaming middleware functionality with real LLM clients
func TestStreamMiddleware(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		modifiedPrompt := "Modified: Please respond with exactly: MIDDLEWARE_WORKS"

		// Streaming middleware that modifies the input prompt
		streamMiddleware := func(next gollem.ContentStreamHandler) gollem.ContentStreamHandler {
			return func(ctx context.Context, req *gollem.ContentRequest) (<-chan *gollem.ContentResponse, error) {
				// Modify the input - replace user's prompt
				for i, input := range req.Inputs {
					if _, ok := input.(gollem.Text); ok {
						req.Inputs[i] = gollem.Text(modifiedPrompt)
					}
				}

				// Call the next handler with modified request
				return next(ctx, req)
			}
		}

		client, err := newClient(t)
		gt.NoError(t, err)

		session, err := client.NewSession(ctx,
			gollem.WithSessionContentStreamMiddleware(streamMiddleware),
		)
		gt.NoError(t, err)

		// Generate stream with original prompt (will be modified by middleware)
		streamChan, err := session.Stream(ctx, []gollem.Input{gollem.Text("Say ORIGINAL_PROMPT")}, gollem.WithMaxTokens(maxTestTokens))
		gt.NoError(t, err)
		if err != nil {
			return // Early return if stream creation failed
		}

		// Collect all streaming responses
		var collectedTexts []string
		for resp := range streamChan {
			if resp.Error != nil {
				t.Fatalf("Stream error: %v", resp.Error)
			}
			collectedTexts = append(collectedTexts, resp.Texts...)
		}

		// Verify the response contains MIDDLEWARE_WORKS (from modified prompt)
		fullResponse := strings.Join(collectedTexts, "")
		if !strings.Contains(fullResponse, "MIDDLEWARE_WORKS") {
			t.Logf("Response should contain MIDDLEWARE_WORKS from modified prompt, got: %s", fullResponse)
		}
		gt.True(t, strings.Contains(fullResponse, "MIDDLEWARE_WORKS"))

		// Verify history contains the modified input
		history, err := session.History()
		gt.NoError(t, err)
		gt.True(t, len(history.Messages) >= 2) // User + Assistant

		// Check the first message (user) contains modified prompt
		if len(history.Messages) > 0 && len(history.Messages[0].Contents) > 0 {
			var content map[string]string
			err := json.Unmarshal(history.Messages[0].Contents[0].Data, &content)
			gt.NoError(t, err)
			if !strings.Contains(content["text"], "Modified:") {
				t.Logf("First message should contain modified prompt, got: %s", content["text"])
			}
			gt.True(t, strings.Contains(content["text"], "Modified:"))
		}
	}

	t.Run("Ollama", func(t *testing.T) {
		t.Parallel()
		testFn(t, newOllamaClient)
	})

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			// Use gpt-5-nano for streaming tests as it supports streaming without organization verification
			return openai.New(context.Background(), apiKey, openai.WithModel("gpt-5-nano"))
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		var opts []gemini.Option
		if model := os.Getenv("TEST_GCP_MODEL"); model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location, opts...)
		})
	})
}

// TestCountToken tests token counting functionality with real LLM clients
func TestCountToken(t *testing.T) {
	t.Parallel()

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		client, err := newClient(t)
		gt.NoError(t, err)

		session, err := client.NewSession(ctx,
			gollem.WithSessionSystemPrompt("You are a helpful assistant."),
			gollem.WithSessionTools(&RandomNumberTool{}),
		)
		gt.NoError(t, err)

		// Get initial history (should be empty)
		initialHistory, err := session.History()
		gt.NoError(t, err)

		// Basic token count - verify it doesn't modify history
		count, err := session.CountToken(ctx, gollem.Text("Hello, world!"))
		gt.NoError(t, err)
		gt.N(t, count).Greater(0)

		// Verify history is unchanged after CountToken
		historyAfterCount, err := session.History()
		gt.NoError(t, err)
		gt.V(t, historyAfterCount).Equal(initialHistory)

		// Generate content to add history
		_, err = session.Generate(ctx, []gollem.Input{gollem.Text("Hi!")}, gollem.WithMaxTokens(maxTestTokens))
		gt.NoError(t, err)

		// Get history after GenerateContent
		historyAfterGenerate, err := session.History()
		gt.NoError(t, err)

		// Count tokens with history - verify it doesn't modify history
		count, err = session.CountToken(ctx, gollem.Text("What is 2+2?"))
		gt.NoError(t, err)
		gt.N(t, count).Greater(0)

		// Verify history is unchanged after CountToken (should still match historyAfterGenerate)
		historyAfterSecondCount, err := session.History()
		gt.NoError(t, err)
		gt.V(t, historyAfterSecondCount).Equal(historyAfterGenerate)
	}

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		var opts []gemini.Option
		if model := os.Getenv("TEST_GCP_MODEL"); model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location, opts...)
		})
	})
}

// TestNewPDF tests PDF type creation and validation
func TestNewPDF(t *testing.T) {
	validPDF := []byte("%PDF-1.4 test content here")

	t.Run("valid PDF", func(t *testing.T) {
		pdf, err := gollem.NewPDF(validPDF)
		gt.NoError(t, err)
		gt.V(t, pdf.MimeType()).Equal("application/pdf")
		gt.V(t, pdf.Data()).Equal(validPDF)
		gt.V(t, pdf.String()).Equal(fmt.Sprintf("pdf (%d bytes)", len(validPDF)))
		gt.V(t, len(pdf.Base64()) > 0).Equal(true)
	})

	t.Run("empty data", func(t *testing.T) {
		_, err := gollem.NewPDF([]byte{})
		gt.Error(t, err)
	})

	t.Run("invalid magic bytes", func(t *testing.T) {
		_, err := gollem.NewPDF([]byte("not a pdf file"))
		gt.Error(t, err)
	})

	t.Run("too short data", func(t *testing.T) {
		_, err := gollem.NewPDF([]byte("%PD"))
		gt.Error(t, err)
	})

	t.Run("from reader", func(t *testing.T) {
		reader := bytes.NewReader(validPDF)
		pdf, err := gollem.NewPDFFromReader(reader)
		gt.NoError(t, err)
		gt.V(t, pdf.Data()).Equal(validPDF)
	})
}

// TestPDFContent tests PDFContent serialization/deserialization
func TestPDFContent(t *testing.T) {
	pdfData := []byte("%PDF-1.4 test content")

	t.Run("round trip with data", func(t *testing.T) {
		mc, err := gollem.NewPDFContent(pdfData, "")
		gt.NoError(t, err)
		gt.V(t, mc.Type).Equal(gollem.MessageContentTypePDF)

		content, err := mc.GetPDFContent()
		gt.NoError(t, err)
		gt.V(t, content.Data).Equal(pdfData)
		gt.V(t, content.URL).Equal("")
	})

	t.Run("round trip with URL", func(t *testing.T) {
		mc, err := gollem.NewPDFContent(nil, "https://example.com/doc.pdf")
		gt.NoError(t, err)

		content, err := mc.GetPDFContent()
		gt.NoError(t, err)
		gt.V(t, len(content.Data)).Equal(0)
		gt.V(t, content.URL).Equal("https://example.com/doc.pdf")
	})

	t.Run("wrong type returns error", func(t *testing.T) {
		mc, err := gollem.NewTextContent("hello")
		gt.NoError(t, err)

		_, err = mc.GetPDFContent()
		gt.Error(t, err)
	})
}

// TestPDFInput tests PDF input with real LLM providers.
// The test PDF contains a unique secret code "GOLLEM-PDF-7X9K2" embedded in a PDF stream.
// The LLM must actually process the document as a PDF to extract this code;
// simply reading the raw bytes as text would not reliably yield the correct answer.
func TestPDFInput(t *testing.T) {
	t.Parallel()

	pdfData, err := os.ReadFile("testdata/test_document.pdf")
	gt.NoError(t, err)
	pdf, err := gollem.NewPDF(pdfData)
	gt.NoError(t, err)

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		client, err := newClient(t)
		gt.NoError(t, err).Required()

		session, err := client.NewSession(ctx)
		gt.NoError(t, err).Required()

		result, err := session.Generate(ctx, []gollem.Input{
			pdf,
			gollem.Text("This PDF document contains a secret code. What is the secret code? Reply with only the code, nothing else."),
		}, gollem.WithMaxTokens(maxTestTokens))
		gt.NoError(t, err).Required()
		gt.V(t, len(result.Texts) > 0).Equal(true)
		// The PDF contains "The secret code is: GOLLEM-PDF-7X9K2"
		combined := strings.Join(result.Texts, " ")
		gt.V(t, strings.Contains(combined, "GOLLEM-PDF-7X9K2")).Equal(true)
	}

	// Note: OpenAI API does not support PDF via image_url field.
	// The SDK workaround (data URL in image_url) is used for History conversion only.
	// Direct PDF input to OpenAI is not supported at this time.

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		var opts []gemini.Option
		if model := os.Getenv("TEST_GCP_MODEL"); model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location, opts...)
		})
	})
}

// TestSessionQueryWithRealLLM tests SessionQuery with real LLM clients.
// It verifies two critical behaviors in a single test per provider (minimal API calls):
//  1. SessionQuery uses per-call GenerateOption (ResponseSchema) to get JSON from a plain-text session
//  2. SessionQuery preserves conversation history from prior Generate calls
func TestSessionQueryWithRealLLM(t *testing.T) {
	t.Parallel()

	type nameAnswer struct {
		Name string `json:"name" description:"the name that was mentioned"`
	}

	testFn := func(t *testing.T, newClient func(t *testing.T) (gollem.LLMClient, error)) {
		ctx, cancel := context.WithTimeout(context.Background(), testTimeout)
		defer cancel()

		client, err := newClient(t)
		gt.NoError(t, err).Required()

		// Create a plain-text session (no ContentTypeJSON, no ResponseSchema)
		session, err := client.NewSession(ctx)
		gt.NoError(t, err).Required()

		// Step 1: Tell the LLM a name via normal Generate (builds history)
		_, err = session.Generate(ctx, []gollem.Input{
			gollem.Text("My name is Quetzalcoatl."),
		}, gollem.WithMaxTokens(maxTestTokens))
		gt.NoError(t, err)

		// Step 2: SessionQuery on the same session.
		// Verifies: history preserved + per-call ResponseSchema produces JSON.
		// Per-call override is not yet fully implemented in providers,
		// so the prompt explicitly requests JSON to maximize reliability.
		resp, err := gollem.SessionQuery[nameAnswer](
			ctx, session,
			`What is my name? Respond as {"name":"..."}`,
			gollem.WithSessionQueryMaxRetry(3),
		)
		gt.NoError(t, err).Required()
		gt.NotNil(t, resp.Data)

		// The LLM should recall the unusual name from the conversation history
		gt.V(t, strings.Contains(resp.Data.Name, "Quetzalcoatl")).Equal(true)
	}

	t.Run("Ollama", func(t *testing.T) {
		t.Parallel()
		testFn(t, newOllamaClient)
	})

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return openai.New(context.Background(), apiKey)
		})
	})

	t.Run("Claude", func(t *testing.T) {
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return claude.New(context.Background(), apiKey)
		})
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		var opts []gemini.Option
		if model := os.Getenv("TEST_GCP_MODEL"); model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		testFn(t, func(t *testing.T) (gollem.LLMClient, error) {
			return gemini.New(context.Background(), projectID, location, opts...)
		})
	})
}

// newOllamaClient creates an Ollama client for the model named by
// TEST_OLLAMA_MODEL, skipping the test when it is not set. TEST_OLLAMA_BASE_URL
// overrides the server address.
func newOllamaClient(t *testing.T) (gollem.LLMClient, error) {
	t.Helper()
	model, ok := os.LookupEnv("TEST_OLLAMA_MODEL")
	if !ok {
		t.Skip("TEST_OLLAMA_MODEL is not set")
	}
	var opts []ollama.Option
	if baseURL, ok := os.LookupEnv("TEST_OLLAMA_BASE_URL"); ok {
		opts = append(opts, ollama.WithBaseURL(baseURL))
	}
	return ollama.New(context.Background(), model, opts...)
}

// countSpansByKind recursively counts spans of a given kind in the trace tree.
func countSpansByKind(span *trace.Span, kind trace.SpanKind) int {
	if span == nil {
		return 0
	}
	count := 0
	if span.Kind == kind {
		count++
	}
	for _, child := range span.Children {
		count += countSpansByKind(child, kind)
	}
	return count
}

// TestModelNamer pins that reporting the model name is optional: a client that
// implements it answers through the LLMClient a caller already holds, and one
// that does not is reported as unable rather than failing the caller.
func TestModelNamer(t *testing.T) {
	t.Run("implementing client reports its configured model", func(t *testing.T) {
		client, err := openai.New(context.Background(), "test-key", openai.WithModel("gpt-5-mini"))
		gt.NoError(t, err).Required()

		var llm gollem.LLMClient = client
		namer, ok := llm.(gollem.ModelNamer)
		gt.True(t, ok).Required()
		gt.Equal(t, "gpt-5-mini", namer.Model())
	})

	t.Run("Ollama client reports its configured model", func(t *testing.T) {
		client, err := ollama.New(context.Background(), "qwen3:8b")
		gt.NoError(t, err).Required()

		var llm gollem.LLMClient = client
		namer, ok := llm.(gollem.ModelNamer)
		gt.True(t, ok).Required()
		gt.Equal(t, "qwen3:8b", namer.Model())
	})

	t.Run("non-implementing client reports nothing", func(t *testing.T) {
		var llm gollem.LLMClient = &mock.LLMClientMock{}
		_, ok := llm.(gollem.ModelNamer)
		gt.False(t, ok)
	})
}

// weatherLookupTool returns a fixed observation so that the final JSON can be
// checked against known values.
type weatherLookupTool struct{}

func (t *weatherLookupTool) Spec() gollem.ToolSpec {
	return gollem.ToolSpec{
		Name:        "get_weather",
		Description: "Returns the current weather observation for a city.",
		Parameters: map[string]*gollem.Parameter{
			"city": {Type: gollem.TypeString, Description: "City name", Required: true},
		},
	}
}

func (t *weatherLookupTool) Run(ctx context.Context, args map[string]any) (map[string]any, error) {
	return map[string]any{"city": args["city"], "temperature_celsius": 21.5, "condition": "sunny"}, nil
}

// TestSchemaCallAfterToolUseWithRealLLM reproduces an agent that first runs a
// tool loop and then asks for a structured answer over the same history, with
// the same system prompt and tool list, tool calls disabled and a per-call
// response schema. On Claude the history carries signed thinking blocks, which
// the API accepts only while the system prompt and tool list are unchanged.
func TestSchemaCallAfterToolUseWithRealLLM(t *testing.T) {
	t.Parallel()

	const systemPrompt = "You are a weather assistant. Always use the get_weather tool to answer questions about the weather."

	answerSchema := &gollem.Parameter{
		Type: gollem.TypeObject,
		Properties: map[string]*gollem.Parameter{
			"city":                {Type: gollem.TypeString, Required: true},
			"temperature_celsius": {Type: gollem.TypeNumber, Required: true},
			"summary":             {Type: gollem.TypeString, Required: true},
		},
	}

	testFn := func(t *testing.T, client gollem.LLMClient, expectThinking bool) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
		defer cancel()

		tool := &weatherLookupTool{}
		sessionOpts := []gollem.SessionOption{
			gollem.WithSessionSystemPrompt(systemPrompt),
			gollem.WithSessionTools(tool),
		}

		// 1. Call with tools and obtain a tool call.
		session, err := client.NewSession(ctx, sessionOpts...)
		gt.NoError(t, err).Required()
		// Claude uses adaptive thinking and skips it for simple requests; the
		// puzzle makes it reason before choosing the tool argument, so that the
		// history carries a signed thinking block.
		resp, err := session.Generate(ctx, []gollem.Input{gollem.Text(
			"I will visit exactly one city. Candidates: Tokyo, Osaka, Sapporo, Fukuoka. " +
				"Rule 1: the city's name must not contain the letter 'k' unless it has exactly five letters. " +
				"Rule 2: among the remaining cities, pick the one whose name has the most vowels; break ties alphabetically. " +
				"Work out the city carefully, then get its weather with the tool.")})
		gt.NoError(t, err).Required()
		gt.A(t, resp.FunctionCalls).Longer(0).Required()

		if expectThinking {
			history, err := session.History()
			gt.NoError(t, err).Required()
			var signedThinking int
			for _, msg := range history.Messages {
				for _, c := range msg.Contents {
					var providerData int
					if c.Provider != nil {
						providerData = len(c.Provider.Data)
					}
					t.Logf("history: role=%s type=%s provider_data=%dB", msg.Role, c.Type, providerData)
					if c.Type == gollem.MessageContentTypeThinking && providerData > 0 {
						signedThinking++
					}
				}
			}
			gt.N(t, signedThinking).Greater(0).Required()
		}

		// 2. Return the tool results.
		var results []gollem.Input
		for _, fc := range resp.FunctionCalls {
			out, err := tool.Run(ctx, fc.Arguments)
			gt.NoError(t, err).Required()
			results = append(results, gollem.FunctionResponse{ID: fc.ID, Name: fc.Name, Data: out})
		}
		_, err = session.Generate(ctx, results)
		gt.NoError(t, err).Required()

		// 3. Same system prompt and tool list over the recorded history, with
		// tool calls disabled and a response schema.
		history, err := session.History()
		gt.NoError(t, err).Required()
		next, err := client.NewSession(ctx, append(sessionOpts, gollem.WithSessionHistory(history))...)
		gt.NoError(t, err).Required()
		resp, err = next.Generate(ctx,
			[]gollem.Input{gollem.Text("Report the observation you obtained as JSON.")},
			gollem.WithToolCallsDisabled(),
			gollem.WithGenerateResponseSchema(answerSchema),
		)

		// 4. The call succeeds with schema-conforming JSON and no tool call.
		gt.NoError(t, err).Required()
		gt.A(t, resp.FunctionCalls).Length(0)
		gt.A(t, resp.Texts).Longer(0).Required()
		var answer struct {
			City               *string  `json:"city"`
			TemperatureCelsius *float64 `json:"temperature_celsius"`
			Summary            *string  `json:"summary"`
		}
		gt.NoError(t, json.Unmarshal([]byte(strings.Join(resp.Texts, "")), &answer)).Required()
		gt.NotNil(t, answer.City)
		gt.NotNil(t, answer.Summary)
		gt.NotNil(t, answer.TemperatureCelsius)
		if answer.TemperatureCelsius != nil {
			gt.Equal(t, 21.5, *answer.TemperatureCelsius)
		}
	}

	t.Run("Claude", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_CLAUDE_API_KEY")
		if !ok {
			t.Skip("TEST_CLAUDE_API_KEY is not set")
		}
		client, err := claude.New(context.Background(), apiKey,
			claude.WithModel("claude-opus-5-5"), claude.WithTimeout(5*time.Minute))
		gt.NoError(t, err).Required()
		testFn(t, client, true)
	})

	t.Run("ClaudeVertex", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_CLAUDE_VERTEX_AI_PROJECT_ID")
		if !ok {
			t.Skip("TEST_CLAUDE_VERTEX_AI_PROJECT_ID is not set")
		}
		location := os.Getenv("TEST_CLAUDE_VERTEX_AI_LOCATION")
		if location == "" {
			location = "us-east5"
		}
		client, err := claude.NewWithVertex(context.Background(), location, projectID,
			claude.WithVertexModel("claude-opus-5-5"))
		gt.NoError(t, err).Required()
		testFn(t, client, true)
	})

	t.Run("OpenAI", func(t *testing.T) {
		t.Parallel()
		apiKey, ok := os.LookupEnv("TEST_OPENAI_API_KEY")
		if !ok {
			t.Skip("TEST_OPENAI_API_KEY is not set")
		}
		client, err := openai.New(context.Background(), apiKey)
		gt.NoError(t, err).Required()
		testFn(t, client, false)
	})

	t.Run("Gemini", func(t *testing.T) {
		t.Parallel()
		projectID, ok := os.LookupEnv("TEST_GCP_PROJECT_ID")
		if !ok {
			t.Skip("TEST_GCP_PROJECT_ID is not set")
		}
		location, ok := os.LookupEnv("TEST_GCP_LOCATION")
		if !ok {
			t.Skip("TEST_GCP_LOCATION is not set")
		}
		var opts []gemini.Option
		if model := os.Getenv("TEST_GCP_MODEL"); model != "" {
			opts = append(opts, gemini.WithModel(model))
		}
		client, err := gemini.New(context.Background(), projectID, location, opts...)
		gt.NoError(t, err).Required()
		testFn(t, client, false)
	})

	t.Run("Ollama", func(t *testing.T) {
		t.Parallel()
		client, err := newOllamaClient(t)
		gt.NoError(t, err).Required()
		testFn(t, client, false)
	})
}
