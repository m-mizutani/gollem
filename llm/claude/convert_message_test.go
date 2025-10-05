package claude_test

import (
	"testing"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gt"
)

func TestClaudeMessageRoundTrip(t *testing.T) {
	type testCase struct {
		name     string
		messages []anthropic.MessageParam
	}

	runTest := func(tc testCase) func(t *testing.T) {
		return func(t *testing.T) {
			// Convert Claude messages to gollem.History
			history, err := claude.NewHistory(tc.messages)
			gt.NoError(t, err)

			// Convert back to Claude messages
			restored, err := claude.ToMessages(history)
			gt.NoError(t, err)

			// Compare messages
			gt.Equal(t, tc.messages, restored)
		}
	}

	t.Run("text messages", runTest(testCase{
		name: "text messages",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Hello")),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("Hi, how can I help you?")),
		},
	}))

	t.Run("tool use and results", runTest(testCase{
		name: "tool use and results",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("What's the weather?")),
			anthropic.NewAssistantMessage(
				anthropic.NewToolUseBlock(
					"toolu_abc123",
					map[string]interface{}{"location": "Tokyo"},
					"get_weather",
				),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(
					"toolu_abc123",
					`{"temperature":25,"condition":"sunny"}`,
					false,
				),
			),
			anthropic.NewAssistantMessage(anthropic.NewTextBlock("The weather in Tokyo is sunny with a temperature of 25°C.")),
		},
	}))

	t.Run("multiple content blocks", runTest(testCase{
		name: "multiple content blocks",
		messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock("Tell me a joke and the weather")),
			anthropic.NewAssistantMessage(
				anthropic.NewTextBlock("Here's a joke: Why did the chicken cross the road? Let me check the weather..."),
				anthropic.NewToolUseBlock(
					"toolu_xyz789",
					map[string]interface{}{"location": "London"},
					"get_weather",
				),
			),
			anthropic.NewUserMessage(
				anthropic.NewToolResultBlock(
					"toolu_xyz789",
					`{"temperature":15,"condition":"rainy"}`,
					false,
				),
			),
		},
	}))
}
