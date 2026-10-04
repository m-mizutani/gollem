package gollem_test

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/m-mizutani/gt"
)

// Cross-provider conversion tests (for example TestOpenAIToClaudeConversion) live in
// the convert_message_test.go of each provider package under llm/, since the
// conversion functions are unexported. See internal/historytest.

func TestHistoryUnmarshalVersionValidation(t *testing.T) {
	type testCase struct {
		version   int
		expectErr bool
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			data := fmt.Sprintf(`{"type":"OpenAI","version":%d,"messages":[]}`, tc.version)
			var h gollem.History
			err := json.Unmarshal([]byte(data), &h)

			if tc.expectErr {
				gt.Error(t, err)
				gt.True(t, errors.Is(err, gollem.ErrHistoryVersionMismatch))
			} else {
				gt.NoError(t, err)
				gt.Equal(t, tc.version, h.Version)
			}
		}
	}

	t.Run("current version", runTest(testCase{
		version:   gollem.HistoryVersion,
		expectErr: false,
	}))

	t.Run("old version 1", runTest(testCase{
		version:   1,
		expectErr: true,
	}))

	t.Run("old version 2", runTest(testCase{
		version:   2,
		expectErr: true,
	}))

	t.Run("old version 3", runTest(testCase{
		version:   3,
		expectErr: true,
	}))

	t.Run("future version", runTest(testCase{
		version:   99,
		expectErr: true,
	}))

	t.Run("zero version", runTest(testCase{
		version:   0,
		expectErr: true,
	}))
}

func TestHistoryCloneWithCurrentVersion(t *testing.T) {
	original := &gollem.History{
		LLType:  gollem.LLMTypeOpenAI,
		Version: gollem.HistoryVersion,
	}
	cloned := original.Clone()
	gt.Equal(t, original.LLType, cloned.LLType)
	gt.Equal(t, original.Version, cloned.Version)
}

// History.Clone must deep-copy ProviderData, which carries signatures that are
// sent back to the provider unchanged.
func TestCloneCopiesProviderData(t *testing.T) {
	issuer := gollem.Issuer{Provider: gollem.LLMTypeGemini, Model: "gemini-test", Scope: "s"}
	original := &gollem.History{
		LLType:  gollem.LLMTypeGemini,
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{
				Role: gollem.RoleAssistant,
				Contents: []gollem.MessageContent{
					{
						Type: gollem.MessageContentTypeThinking,
						Data: json.RawMessage(`{"text":"reasoning"}`),
						Provider: &gollem.ProviderData{
							Issuer: issuer,
							Data:   json.RawMessage(`{"thought_signature":"YWJjZA=="}`),
						},
					},
					{
						Type: gollem.MessageContentTypeText,
						Data: json.RawMessage(`{"text":"answer"}`),
					},
				},
			},
		},
	}

	cloned := original.Clone()

	gotProvider := cloned.Messages[0].Contents[0].Provider
	gt.NotNil(t, gotProvider)
	gt.Equal(t, issuer, gotProvider.Issuer)
	gt.Equal(t, string(original.Messages[0].Contents[0].Provider.Data), string(gotProvider.Data))
	gt.Nil(t, cloned.Messages[0].Contents[1].Provider)

	// Rewriting the clone's bytes must not reach the original.
	gotProvider.Data[2] = 'X'
	gotProvider.Issuer.Scope = "changed"
	gt.Equal(t, `{"thought_signature":"YWJjZA=="}`, string(original.Messages[0].Contents[0].Provider.Data))
	gt.Equal(t, "s", original.Messages[0].Contents[0].Provider.Issuer.Scope)
}

func TestHistoryProviderDataJSONRoundTrip(t *testing.T) {
	issuer := gollem.Issuer{Provider: gollem.LLMTypeClaude, Model: "claude-test", Scope: "tenant-a"}
	original := &gollem.History{
		LLType:  gollem.LLMTypeClaude,
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{
				Role: gollem.RoleAssistant,
				Contents: []gollem.MessageContent{
					{
						Type: gollem.MessageContentTypeThinking,
						Data: json.RawMessage(`{"text":"plan"}`),
						Provider: &gollem.ProviderData{
							Issuer: issuer,
							Data:   json.RawMessage(`{"signature":"sig"}`),
						},
					},
					{
						Type: gollem.MessageContentTypeText,
						Data: json.RawMessage(`{"text":"answer"}`),
					},
				},
			},
		},
	}

	data, err := json.Marshal(original)
	gt.NoError(t, err)

	var raw struct {
		Messages []struct {
			Contents []map[string]json.RawMessage `json:"contents"`
		} `json:"messages"`
	}
	gt.NoError(t, json.Unmarshal(data, &raw))
	_, hasProvider := raw.Messages[0].Contents[1]["provider"]
	gt.False(t, hasProvider)
	_, hasMeta := raw.Messages[0].Contents[0]["meta"]
	gt.False(t, hasMeta)

	var restored gollem.History
	gt.NoError(t, json.Unmarshal(data, &restored))
	got := restored.Messages[0].Contents[0].Provider
	gt.NotNil(t, got)
	gt.Equal(t, issuer, got.Issuer)
	gt.Equal(t, `{"signature":"sig"}`, string(got.Data))
	gt.Nil(t, restored.Messages[0].Contents[1].Provider)
}

// Clone used to deep-copy Metadata through a JSON round-trip, which dropped it entirely on
// an encoding failure and rewrote every number as a float64.
func TestCloneMetadataIsIndependentAndKeepsValues(t *testing.T) {
	original := &gollem.History{
		LLType:  gollem.LLMTypeClaude,
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{
				Role: gollem.RoleAssistant,
				Metadata: map[string]any{
					"account": int64(9007199254740993),
					"nested":  map[string]any{"tags": []any{"a", "b"}},
				},
			},
		},
	}

	cloned := original.Clone()

	gt.Equal(t, int64(9007199254740993), gt.Cast[int64](t, cloned.Messages[0].Metadata["account"]))

	// Mutating the clone must not reach the original.
	clonedNested := gt.Cast[map[string]any](t, cloned.Messages[0].Metadata["nested"])
	clonedTags := gt.Cast[[]any](t, clonedNested["tags"])
	clonedTags[0] = "changed"
	cloned.Messages[0].Metadata["account"] = int64(1)

	originalNested := gt.Cast[map[string]any](t, original.Messages[0].Metadata["nested"])
	originalTags := gt.Cast[[]any](t, originalNested["tags"])
	gt.Equal(t, "a", gt.Cast[string](t, originalTags[0]))
	gt.Equal(t, int64(9007199254740993), gt.Cast[int64](t, original.Messages[0].Metadata["account"]))
}

// Metadata is an exported map that callers fill with arbitrary Go values, not only the
// map[string]any / []any shapes a JSON decode produces. Those values must be copied too,
// or the clone shares storage with the original.
func TestCloneMetadataCopiesNonJSONReferenceTypes(t *testing.T) {
	labels := map[string]string{"env": "prod"}
	tags := []string{"a", "b"}
	counter := 7

	original := &gollem.History{
		LLType:  gollem.LLMTypeClaude,
		Version: gollem.HistoryVersion,
		Messages: []gollem.Message{
			{
				Role: gollem.RoleAssistant,
				Metadata: map[string]any{
					"tags":    tags,
					"labels":  labels,
					"counter": &counter,
				},
			},
		},
	}

	cloned := original.Clone()

	gt.Cast[[]string](t, cloned.Messages[0].Metadata["tags"])[0] = "changed"
	gt.Cast[map[string]string](t, cloned.Messages[0].Metadata["labels"])["env"] = "dev"
	*gt.Cast[*int](t, cloned.Messages[0].Metadata["counter"]) = 99

	gt.Equal(t, "a", tags[0])
	gt.Equal(t, "prod", labels["env"])
	gt.Equal(t, 7, counter)
}
