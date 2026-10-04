package gollem_test

import (
	"encoding/json"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/m-mizutani/gt"
)

func TestMessageContentTypeThinking(t *testing.T) {
	ct := gollem.MessageContentTypeThinking
	if ct != "thinking" {
		t.Errorf("Expected \"thinking\", got '%s'", ct)
	}
}

func TestThinkingContent(t *testing.T) {
	content := gollem.ThinkingContent{Text: "Let me think..."}
	if content.Text != "Let me think..." {
		t.Errorf("Expected 'Let me think...', got '%s'", content.Text)
	}
}

func TestNewThinkingContent(t *testing.T) {
	mc, err := gollem.NewThinkingContent("test thinking")
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if mc.Type != gollem.MessageContentTypeThinking {
		t.Errorf("Expected type 'reasoning', got '%s'", mc.Type)
	}
}

func TestGetThinkingContent(t *testing.T) {
	mc, _ := gollem.NewThinkingContent("test thinking")
	tc, err := mc.GetThinkingContent()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if tc.Text != "test thinking" {
		t.Errorf("Expected 'test thinking', got '%s'", tc.Text)
	}
}

func TestIssuerEqual(t *testing.T) {
	base := gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "m", Scope: "s"}

	type testCase struct {
		other    gollem.Issuer
		expected bool
	}
	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			gt.Equal(t, tc.expected, base.Equal(tc.other))
			gt.Equal(t, tc.expected, tc.other.Equal(base))
		}
	}

	t.Run("all fields equal", runTest(testCase{
		other:    gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "m", Scope: "s"},
		expected: true,
	}))
	t.Run("provider differs", runTest(testCase{
		other:    gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "m", Scope: "s"},
		expected: false,
	}))
	t.Run("model differs", runTest(testCase{
		other:    gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "m2", Scope: "s"},
		expected: false,
	}))
	t.Run("scope differs", runTest(testCase{
		other:    gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "m", Scope: "s2"},
		expected: false,
	}))
}

func mustContent(t *testing.T, mc gollem.MessageContent, err error) gollem.MessageContent {
	t.Helper()
	gt.NoError(t, err)
	return mc
}

func withProvider(mc gollem.MessageContent, issuer gollem.Issuer, data string) gollem.MessageContent {
	mc.Provider = &gollem.ProviderData{Issuer: issuer}
	if data != "" {
		mc.Provider.Data = json.RawMessage(data)
	}
	return mc
}

func TestFilterProviderData(t *testing.T) {
	dest := gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "claude-test", Scope: "a"}

	thinking := func(t *testing.T, text string) gollem.MessageContent {
		mc, err := gollem.NewThinkingContent(text)
		return mustContent(t, mc, err)
	}
	text := func(t *testing.T, s string) gollem.MessageContent {
		mc, err := gollem.NewTextContent(s)
		return mustContent(t, mc, err)
	}
	toolCall := func(t *testing.T) gollem.MessageContent {
		mc, err := gollem.NewToolCallContent("call_1", "search", map[string]any{"q": "x"})
		return mustContent(t, mc, err)
	}
	assistant := func(contents ...gollem.MessageContent) gollem.Message {
		return gollem.Message{Role: gollem.RoleAssistant, Contents: contents}
	}

	t.Run("thinking issued by dest is kept with its data", func(t *testing.T) {
		in := []gollem.Message{assistant(withProvider(thinking(t, "plan"), dest, `{"signature":"sig"}`))}
		out := gollem.FilterProviderData(in, dest)
		gt.A(t, out).Length(1)
		gt.A(t, out[0].Contents).Length(1)
		gt.NotNil(t, out[0].Contents[0].Provider)
		gt.Equal(t, `{"signature":"sig"}`, string(out[0].Contents[0].Provider.Data))
	})

	runDropThinking := func(issuer gollem.Issuer) func(t *testing.T) {
		return func(t *testing.T) {
			in := []gollem.Message{assistant(
				withProvider(thinking(t, "plan"), issuer, `{"signature":"sig"}`),
				text(t, "answer"),
			)}
			out := gollem.FilterProviderData(in, dest)
			gt.A(t, out).Length(1)
			gt.A(t, out[0].Contents).Length(1)
			gt.Equal(t, gollem.MessageContentTypeText, out[0].Contents[0].Type)
		}
	}
	t.Run("thinking from another provider is removed", runDropThinking(
		gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "claude-test", Scope: "a"}))
	t.Run("thinking from another model is removed", runDropThinking(
		gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "claude-other", Scope: "a"}))
	t.Run("thinking from another scope is removed", runDropThinking(
		gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "claude-test", Scope: "b"}))

	t.Run("thinking without provider data is removed", func(t *testing.T) {
		in := []gollem.Message{assistant(thinking(t, "plan"), text(t, "answer"))}
		out := gollem.FilterProviderData(in, dest)
		gt.A(t, out).Length(1)
		gt.A(t, out[0].Contents).Length(1)
		gt.Equal(t, gollem.MessageContentTypeText, out[0].Contents[0].Type)
	})

	t.Run("text and tool call from another issuer lose only their provider data", func(t *testing.T) {
		other := gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "gemini-test"}
		txt := withProvider(text(t, "answer"), other, `{"thought_signature":"c2ln"}`)
		call := withProvider(toolCall(t), other, `{"thought_signature":"c2ln"}`)
		in := []gollem.Message{assistant(txt, call)}

		out := gollem.FilterProviderData(in, dest)
		gt.A(t, out).Length(1)
		gt.A(t, out[0].Contents).Length(2)
		gt.Nil(t, out[0].Contents[0].Provider)
		gt.Nil(t, out[0].Contents[1].Provider)
		gt.Equal(t, string(txt.Data), string(out[0].Contents[0].Data))
		gt.Equal(t, string(call.Data), string(out[0].Contents[1].Data))
	})

	t.Run("tool call issued by dest keeps its provider data", func(t *testing.T) {
		in := []gollem.Message{assistant(withProvider(toolCall(t), dest, `{"k":1}`))}
		out := gollem.FilterProviderData(in, dest)
		gt.NotNil(t, out[0].Contents[0].Provider)
		gt.Equal(t, `{"k":1}`, string(out[0].Contents[0].Provider.Data))
	})

	t.Run("message left without content is removed and order is kept", func(t *testing.T) {
		other := gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "gemini-test"}
		in := []gollem.Message{
			{Role: gollem.RoleUser, Contents: []gollem.MessageContent{text(t, "first")}},
			assistant(withProvider(thinking(t, ""), other, `{"thought_signature":"c2ln"}`)),
			assistant(text(t, "second")),
		}
		out := gollem.FilterProviderData(in, dest)
		gt.A(t, out).Length(2)
		gt.Equal(t, gollem.RoleUser, out[0].Role)
		gt.Equal(t, gollem.RoleAssistant, out[1].Role)
		tc, err := out[1].Contents[0].GetTextContent()
		gt.NoError(t, err)
		gt.Equal(t, "second", tc.Text)
	})

	t.Run("message that had no content is kept", func(t *testing.T) {
		in := []gollem.Message{{Role: gollem.RoleAssistant}}
		out := gollem.FilterProviderData(in, dest)
		gt.A(t, out).Length(1)
	})

	t.Run("input is not modified", func(t *testing.T) {
		other := gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "gemini-test"}
		in := []gollem.Message{assistant(
			withProvider(thinking(t, "plan"), other, `{"s":1}`),
			withProvider(text(t, "answer"), other, `{"s":2}`),
		)}
		_ = gollem.FilterProviderData(in, dest)

		gt.A(t, in[0].Contents).Length(2)
		gt.NotNil(t, in[0].Contents[0].Provider)
		gt.NotNil(t, in[0].Contents[1].Provider)
		gt.Equal(t, `{"s":2}`, string(in[0].Contents[1].Provider.Data))
	})

	t.Run("custom client issuer", func(t *testing.T) {
		custom := gollem.Issuer{Provider: gollem.LLMType("custom"), Model: "m"}
		in := []gollem.Message{assistant(
			withProvider(thinking(t, ""), custom, `{"block":"opaque"}`),
			text(t, "answer"),
		)}

		kept := gollem.FilterProviderData(in, custom)
		gt.A(t, kept[0].Contents).Length(2)
		gt.Equal(t, gollem.MessageContentTypeThinking, kept[0].Contents[0].Type)

		removed := gollem.FilterProviderData(in, gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "m"})
		gt.A(t, removed[0].Contents).Length(1)
		gt.Equal(t, gollem.MessageContentTypeText, removed[0].Contents[0].Type)
	})
}
