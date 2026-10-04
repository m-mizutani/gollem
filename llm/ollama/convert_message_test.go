package ollama_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/gollem-dev/gollem/llm/ollama"
	"github.com/gollem-dev/gollem/llm/openai"
	"github.com/m-mizutani/gt"
	goopenai "github.com/sashabaranov/go-openai"
)

// mustContent returns a function that unwraps the result of a
// gollem.New*Content constructor, failing the test on error.
func mustContent(t *testing.T) func(gollem.MessageContent, error) gollem.MessageContent {
	return func(c gollem.MessageContent, err error) gollem.MessageContent {
		t.Helper()
		gt.NoError(t, err).Required()
		return c
	}
}

func wireMessages(t *testing.T, data json.RawMessage) []map[string]any {
	t.Helper()
	var messages []map[string]any
	gt.NoError(t, json.Unmarshal(data, &messages)).Required()
	return messages
}

func TestHistoryConversion(t *testing.T) {
	t.Run("round trip of every supported content", func(t *testing.T) {
		thinking := mustContent(t)(gollem.NewThinkingContent("let me think"))
		thinking.Provider = &gollem.ProviderData{Issuer: ollama.TestIssuer}
		text := mustContent(t)(gollem.NewTextContent("look at this"))
		image := mustContent(t)(gollem.NewImageContent("image/png", pngData, "", ""))
		call := mustContent(t)(gollem.NewToolCallContent("c1", "get_weather", map[string]any{"city": "Tokyo"}))
		resp := mustContent(t)(gollem.NewToolResponseContent("c1", "get_weather", map[string]any{"weather": "sunny"}, false))

		history := &gollem.History{
			LLType:  gollem.LLMTypeOllama,
			Version: gollem.HistoryVersion,
			Messages: []gollem.Message{
				{Role: gollem.RoleSystem, Contents: []gollem.MessageContent{mustContent(t)(gollem.NewTextContent("sys"))}},
				{Role: gollem.RoleUser, Contents: []gollem.MessageContent{text, image}},
				{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{thinking, call}},
				{Role: gollem.RoleTool, Contents: []gollem.MessageContent{resp}},
			},
		}

		wire, back, err := ollama.HistoryRoundTrip(history)
		gt.NoError(t, err).Required()

		msgs := wireMessages(t, wire)
		gt.A(t, msgs).Length(4).Required()
		gt.Equal(t, msgs[0]["role"], any("system"))
		gt.Equal(t, msgs[1]["images"], any([]any{"iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR4nGMAAQAABQABDQottAAAAABJRU5ErkJggg=="}))
		gt.Equal(t, msgs[2]["thinking"], any("let me think"))
		gt.Equal(t, msgs[3]["tool_call_id"], any("c1"))
		gt.Equal(t, msgs[3]["tool_name"], any("get_weather"))

		gt.Equal(t, back.LLType, gollem.LLMTypeOllama)
		gt.Equal(t, back.Version, gollem.HistoryVersion)
		gt.Equal(t, back.Messages, history.Messages)

		img, err := back.Messages[1].Contents[1].GetImageContent()
		gt.NoError(t, err).Required()
		gt.Equal(t, img.MediaType, "image/png")
	})

	t.Run("several tool responses in one message", func(t *testing.T) {
		history := &gollem.History{Messages: []gollem.Message{{
			Role: gollem.RoleTool,
			Contents: []gollem.MessageContent{
				mustContent(t)(gollem.NewToolResponseContent("c1", "a", map[string]any{"v": 1}, false)),
				mustContent(t)(gollem.NewToolResponseContent("c2", "b", map[string]any{"v": 2}, false)),
			},
		}}}
		wire, _, err := ollama.HistoryRoundTrip(history)
		gt.NoError(t, err).Required()
		msgs := wireMessages(t, wire)
		gt.A(t, msgs).Length(2).Required()
		gt.Equal(t, msgs[0]["tool_call_id"], any("c1"))
		gt.Equal(t, msgs[1]["tool_call_id"], any("c2"))
	})

	t.Run("tool response inside a user message", func(t *testing.T) {
		history := &gollem.History{Messages: []gollem.Message{{
			Role: gollem.RoleUser,
			Contents: []gollem.MessageContent{
				mustContent(t)(gollem.NewToolResponseContent("c1", "a", map[string]any{"v": 1}, false)),
			},
		}}}
		wire, _, err := ollama.HistoryRoundTrip(history)
		gt.NoError(t, err).Required()
		msgs := wireMessages(t, wire)
		gt.A(t, msgs).Length(1).Required()
		gt.Equal(t, msgs[0]["role"], any("tool"))
	})

	t.Run("tool responses precede the user text of the same message", func(t *testing.T) {
		// Claude histories put the tool results and the next user text in one
		// user message, in this order.
		history := &gollem.History{Messages: []gollem.Message{
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{
				mustContent(t)(gollem.NewToolCallContent("c1", "a", map[string]any{})),
			}},
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{
				mustContent(t)(gollem.NewToolResponseContent("c1", "a", map[string]any{"v": 1}, false)),
				mustContent(t)(gollem.NewTextContent("and next?")),
			}},
		}}
		wire, _, err := ollama.HistoryRoundTrip(history)
		gt.NoError(t, err).Required()
		msgs := wireMessages(t, wire)
		gt.A(t, msgs).Length(3).Required()
		gt.Equal(t, msgs[0]["role"], any("assistant"))
		gt.Equal(t, msgs[1]["role"], any("tool"))
		gt.Equal(t, msgs[1]["tool_call_id"], any("c1"))
		gt.Equal(t, msgs[2]["role"], any("user"))
		gt.Equal(t, msgs[2]["content"], any("and next?"))
	})

	runRejected := func(content gollem.MessageContent) func(t *testing.T) {
		return func(t *testing.T) {
			history := &gollem.History{Messages: []gollem.Message{{
				Role:     gollem.RoleUser,
				Contents: []gollem.MessageContent{content},
			}}}
			_, _, err := ollama.HistoryRoundTrip(history)
			gt.Error(t, err).Is(gollem.ErrInvalidParameter)
		}
	}
	t.Run("image given only by URL", runRejected(mustContent(t)(gollem.NewImageContent("", nil, "https://example.com/a.png", ""))))
	t.Run("PDF", runRejected(mustContent(t)(gollem.NewPDFContent([]byte("%PDF-1.4"), ""))))

	t.Run("tool message whose content is not JSON", func(t *testing.T) {
		history, err := ollama.HistoryFromWire([]byte(`[{"role":"tool","content":"Error message: timeout","tool_name":"a","tool_call_id":"c1"}]`))
		gt.NoError(t, err).Required()
		resp, err := history.Messages[0].Contents[0].GetToolResponseContent()
		gt.NoError(t, err).Required()
		gt.Equal(t, resp.Response, map[string]any{"content": "Error message: timeout"})
		gt.Equal(t, resp.ToolCallID, "c1")
	})

	t.Run("history from another provider", func(t *testing.T) {
		openaiHistory, err := openai.NewHistory([]goopenai.ChatCompletionMessage{
			{Role: goopenai.ChatMessageRoleUser, Content: "from openai"},
			{Role: goopenai.ChatMessageRoleAssistant, Content: "openai reply"},
		}, gollem.Issuer{Provider: gollem.LLMTypeOpenAI, Model: "gpt-test"})
		gt.NoError(t, err).Required()

		fs := newFakeServer(t, replyText("ok", ""))
		session := newTestSession(t, newTestClient(t, fs), gollem.WithSessionHistory(openaiHistory))
		_, err = session.Generate(context.Background(), []gollem.Input{gollem.Text("next")})
		gt.NoError(t, err).Required()

		msgs := messagesOf(t, fs.Last(t))
		gt.A(t, msgs).Length(3).Required()
		gt.Equal(t, msgs[0]["content"], any("from openai"))
		gt.Equal(t, msgs[1]["role"], any("assistant"))
		gt.Equal(t, msgs[1]["content"], any("openai reply"))
	})

	t.Run("thinking is sent only to its issuer", func(t *testing.T) {
		ownThinking := mustContent(t)(gollem.NewThinkingContent("own reasoning"))
		ownThinking.Provider = &gollem.ProviderData{Issuer: ollama.TestIssuer}
		otherThinking := mustContent(t)(gollem.NewThinkingContent("other reasoning"))
		otherThinking.Provider = &gollem.ProviderData{
			Issuer: gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "claude-test"},
			Data:   json.RawMessage(`{"signature":"sig"}`),
		}
		history := &gollem.History{Messages: []gollem.Message{
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{mustContent(t)(gollem.NewTextContent("q"))}},
			{Role: gollem.RoleAssistant, Contents: []gollem.MessageContent{
				otherThinking, ownThinking, mustContent(t)(gollem.NewTextContent("answer")),
			}},
		}}

		wire, back, err := ollama.HistoryRoundTrip(history)
		gt.NoError(t, err).Required()
		msgs := wireMessages(t, wire)
		gt.A(t, msgs).Length(2).Required()
		gt.Equal(t, msgs[1]["thinking"], any("own reasoning"))

		// The thinking read back records the session's issuer and no data.
		thinking := back.Messages[1].Contents[0]
		gt.Equal(t, gollem.MessageContentTypeThinking, thinking.Type)
		gt.NotNil(t, thinking.Provider)
		gt.Equal(t, ollama.TestIssuer, thinking.Provider.Issuer)
		gt.A(t, thinking.Provider.Data).Length(0)

		otherModel := gollem.Issuer{Provider: gollem.LLMTypeOllama, Model: "other-model"}
		wire, _, err = ollama.HistoryRoundTripWith(history, otherModel, otherModel)
		gt.NoError(t, err).Required()
		msgs = wireMessages(t, wire)
		gt.A(t, msgs).Length(2).Required()
		_, hasThinking := msgs[1]["thinking"]
		gt.False(t, hasThinking)
		gt.Equal(t, msgs[1]["content"], any("answer"))
	})

	t.Run("session history with unsupported content", func(t *testing.T) {
		history := &gollem.History{Messages: []gollem.Message{{
			Role:     gollem.RoleUser,
			Contents: []gollem.MessageContent{mustContent(t)(gollem.NewPDFContent([]byte("%PDF-1.4"), ""))},
		}}}
		fs := newFakeServer(t, replyText("ok", ""))
		_, err := newTestClient(t, fs).NewSession(context.Background(), gollem.WithSessionHistory(history))
		gt.Error(t, err).Is(gollem.ErrInvalidParameter)
	})
}
