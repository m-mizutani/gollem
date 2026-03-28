package gollem_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gt"
)

// mockSession implements gollem.Session for testing WithDelegatedHistory.
type mockSession struct {
	history    *gollem.History
	historyErr error
}

func (s *mockSession) GenerateContent(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
	return nil, nil
}

func (s *mockSession) GenerateStream(ctx context.Context, input ...gollem.Input) (<-chan *gollem.Response, error) {
	return nil, nil
}

func (s *mockSession) History() (*gollem.History, error) {
	return s.history, s.historyErr
}

func (s *mockSession) AppendHistory(h *gollem.History) error {
	return nil
}

func (s *mockSession) CountToken(ctx context.Context, input ...gollem.Input) (int, error) {
	return 0, nil
}

func TestWithDelegatedHistory(t *testing.T) {
	testHistory := func() *gollem.History {
		return &gollem.History{
			LLType:  gollem.LLMTypeClaude,
			Version: gollem.HistoryVersion,
			Messages: []gollem.Message{
				{
					Role: gollem.RoleUser,
					Contents: []gollem.MessageContent{
						mustTextContent(t, "hello"),
					},
				},
			},
		}
	}

	t.Run("inherits history from source session", func(t *testing.T) {
		src := &mockSession{history: testHistory()}
		cfg := gollem.NewSessionConfig(gollem.WithDelegatedHistory(src))

		gt.NoError(t, cfg.Err())
		gt.Value(t, cfg.History()).NotNil()
		gt.Equal(t, len(cfg.History().Messages), 1)
		gt.Equal(t, cfg.History().LLType, gollem.LLMTypeClaude)
	})

	t.Run("deep copies history so source is not affected", func(t *testing.T) {
		h := testHistory()
		src := &mockSession{history: h}
		cfg := gollem.NewSessionConfig(gollem.WithDelegatedHistory(src))

		gt.NoError(t, cfg.Err())
		// Mutate the delegated history
		cfg.History().Messages = append(cfg.History().Messages, gollem.Message{
			Role: gollem.RoleAssistant,
		})
		// Source history should remain unchanged
		gt.Equal(t, len(h.Messages), 1)
	})

	t.Run("nil history from source session", func(t *testing.T) {
		src := &mockSession{history: nil}
		cfg := gollem.NewSessionConfig(gollem.WithDelegatedHistory(src))

		gt.NoError(t, cfg.Err())
		gt.Value(t, cfg.History()).Nil()
	})

	t.Run("error from source session is deferred to Err()", func(t *testing.T) {
		src := &mockSession{historyErr: errors.New("history unavailable")}
		cfg := gollem.NewSessionConfig(gollem.WithDelegatedHistory(src))

		gt.Error(t, cfg.Err())
		gt.Value(t, cfg.History()).Nil()
	})

	t.Run("nil source session returns error instead of panic", func(t *testing.T) {
		cfg := gollem.NewSessionConfig(gollem.WithDelegatedHistory(nil))

		gt.Error(t, cfg.Err())
		gt.Value(t, cfg.History()).Nil()
	})
}

func mustTextContent(t *testing.T, text string) gollem.MessageContent {
	t.Helper()
	c, err := gollem.NewTextContent(text)
	gt.NoError(t, err)
	return c
}
