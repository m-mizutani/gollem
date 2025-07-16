package claude_test

import (
	"context"
	"os"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/claude"
	"github.com/m-mizutani/gt"
)

func TestTokenCounts(t *testing.T) {
	apiKey, ok := os.LookupEnv("TEST_ANTHROPIC_API_KEY")
	if !ok {
		t.Skip("TEST_ANTHROPIC_API_KEY not set, skipping integration test")
	}

	ctx := context.Background()
	client, err := claude.New(ctx, apiKey)
	gt.NoError(t, err)

	session, err := client.NewSession(ctx)
	gt.NoError(t, err)

	t.Run("GenerateContent should return non-negative token counts", func(t *testing.T) {
		resp, err := session.GenerateContent(ctx, gollem.Text("Hello, how are you?"))
		gt.NoError(t, err)
		gt.V(t, resp).NotNil()

		// For a simple text interaction, we should have positive token counts
		gt.N(t, resp.InputToken).Greater(0)
		gt.N(t, resp.OutputToken).Greater(0)

		t.Logf("InputToken: %d, OutputToken: %d", resp.InputToken, resp.OutputToken)
	})

	t.Run("GenerateStream should return non-negative token counts", func(t *testing.T) {
		respChan, err := session.GenerateStream(ctx, gollem.Text("Count to 3"))
		gt.NoError(t, err)

		var lastResp *gollem.Response
		var maxInputTokens, maxOutputTokens int

		for resp := range respChan {
			gt.NoError(t, resp.Error)

			// Token counts should always be >= 0
			gt.N(t, resp.InputToken).GreaterOrEqual(0)
			gt.N(t, resp.OutputToken).GreaterOrEqual(0)

			// Keep track of maximum token counts seen
			if resp.InputToken > maxInputTokens {
				maxInputTokens = resp.InputToken
			}
			if resp.OutputToken > maxOutputTokens {
				maxOutputTokens = resp.OutputToken
			}

			lastResp = resp
		}

		gt.V(t, lastResp).NotNil()

		// Either the final response or some response during streaming should have positive token counts
		hasPositiveTokens := maxInputTokens > 0 && maxOutputTokens > 0
		gt.V(t, hasPositiveTokens).Equal(true)

		t.Logf("Max streaming tokens - InputToken: %d, OutputToken: %d", maxInputTokens, maxOutputTokens)
	})
}
