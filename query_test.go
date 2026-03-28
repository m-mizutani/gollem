package gollem_test

import (
	"context"
	"errors"
	"testing"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/mock"
	"github.com/m-mizutani/gt"
)

type testQueryResult struct {
	Name  string `json:"name" description:"name of the item"`
	Count int    `json:"count" description:"number of items"`
}

func setupQueryMock(t *testing.T, genFunc func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error)) *mock.LLMClientMock {
	t.Helper()
	sessionMock := &mock.SessionMock{
		GenerateContentFunc: genFunc,
	}
	return &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			return sessionMock, nil
		},
	}
}

func TestQuerySuccess(t *testing.T) {
	client := setupQueryMock(t, func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
		return &gollem.Response{
			Texts:       []string{`{"name":"test","count":42}`},
			InputToken:  10,
			OutputToken: 5,
		}, nil
	})

	resp, err := gollem.Query[testQueryResult](context.Background(), client, "test prompt")
	gt.NoError(t, err)
	gt.Value(t, resp.Data.Name).Equal("test")
	gt.Value(t, resp.Data.Count).Equal(42)
	gt.Value(t, resp.InputToken).Equal(10)
	gt.Value(t, resp.OutputToken).Equal(5)
}

func buildSessionConfig(opts []gollem.SessionOption) gollem.SessionConfig {
	return gollem.NewSessionConfig(opts...)
}

func TestQueryWithSystemPrompt(t *testing.T) {
	var capturedOpts []gollem.SessionOption
	sessionMock := &mock.SessionMock{
		GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			return &gollem.Response{
				Texts: []string{`{"name":"x","count":1}`},
			}, nil
		},
	}
	client := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			capturedOpts = options
			return sessionMock, nil
		},
	}

	_, err := gollem.Query[testQueryResult](context.Background(), client, "test",
		gollem.WithQuerySystemPrompt("You are a planner"),
	)
	gt.NoError(t, err)

	cfg := buildSessionConfig(capturedOpts)
	gt.Value(t, cfg.ContentType()).Equal(gollem.ContentTypeJSON)
	gt.Value(t, cfg.ResponseSchema()).NotEqual((*gollem.Parameter)(nil))
	gt.Value(t, cfg.SystemPrompt()).Equal("You are a planner")
	gt.Value(t, cfg.History()).Equal((*gollem.History)(nil))
}

func TestQueryWithHistory(t *testing.T) {
	var capturedOpts []gollem.SessionOption
	sessionMock := &mock.SessionMock{
		GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			return &gollem.Response{
				Texts: []string{`{"name":"x","count":1}`},
			}, nil
		},
	}
	client := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			capturedOpts = options
			return sessionMock, nil
		},
	}

	history := &gollem.History{}
	_, err := gollem.Query[testQueryResult](context.Background(), client, "test",
		gollem.WithQueryHistory(history),
	)
	gt.NoError(t, err)

	cfg := buildSessionConfig(capturedOpts)
	gt.Value(t, cfg.ContentType()).Equal(gollem.ContentTypeJSON)
	gt.Value(t, cfg.ResponseSchema()).NotEqual((*gollem.Parameter)(nil))
	gt.Value(t, cfg.SystemPrompt()).Equal("")
	gt.Value(t, cfg.History()).Equal(history)
}

func TestQueryRetrySuccess(t *testing.T) {
	callCount := 0
	client := setupQueryMock(t, func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
		callCount++
		if callCount == 1 {
			// First call returns invalid JSON
			return &gollem.Response{
				Texts:       []string{`not valid json`},
				InputToken:  10,
				OutputToken: 5,
			}, nil
		}
		// Second call returns valid JSON
		return &gollem.Response{
			Texts:       []string{`{"name":"retry","count":99}`},
			InputToken:  12,
			OutputToken: 6,
		}, nil
	})

	resp, err := gollem.Query[testQueryResult](context.Background(), client, "test")
	gt.NoError(t, err)
	gt.Value(t, resp.Data.Name).Equal("retry")
	gt.Value(t, resp.Data.Count).Equal(99)
	gt.Value(t, callCount).Equal(2)
	// Token counts should be accumulated
	gt.Value(t, resp.InputToken).Equal(22)
	gt.Value(t, resp.OutputToken).Equal(11)
}

func TestQueryRetryExhausted(t *testing.T) {
	callCount := 0
	client := setupQueryMock(t, func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
		callCount++
		return &gollem.Response{
			Texts: []string{`not json`},
		}, nil
	})

	_, err := gollem.Query[testQueryResult](context.Background(), client, "test",
		gollem.WithQueryMaxRetry(2),
	)
	gt.Error(t, err)
	// 1 initial + 2 retries = 3 calls
	gt.Value(t, callCount).Equal(3)
}

func TestQueryEmptyResponse(t *testing.T) {
	client := setupQueryMock(t, func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
		return &gollem.Response{
			Texts: []string{},
		}, nil
	})

	_, err := gollem.Query[testQueryResult](context.Background(), client, "test")
	gt.Error(t, err)
}

func TestQueryGenerateContentError(t *testing.T) {
	callCount := 0
	sessionMock := &mock.SessionMock{
		GenerateContentFunc: func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
			callCount++
			return nil, errors.New("network error")
		},
	}
	clientWithCounter := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			return sessionMock, nil
		},
	}

	_, err := gollem.Query[testQueryResult](context.Background(), clientWithCounter, "test")
	gt.Error(t, err)
	// Should not retry on GenerateContent error
	gt.Value(t, callCount).Equal(1)
}

func TestQueryNewSessionError(t *testing.T) {
	client := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			return nil, errors.New("session creation failed")
		},
	}

	_, err := gollem.Query[testQueryResult](context.Background(), client, "test")
	gt.Error(t, err)
}

func TestQueryNilSession(t *testing.T) {
	client := &mock.LLMClientMock{
		NewSessionFunc: func(ctx context.Context, options ...gollem.SessionOption) (gollem.Session, error) {
			return nil, nil
		},
	}

	_, err := gollem.Query[testQueryResult](context.Background(), client, "test")
	gt.Error(t, err)
}

func TestQueryNegativeMaxRetry(t *testing.T) {
	callCount := 0
	client := setupQueryMock(t, func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
		callCount++
		return &gollem.Response{
			Texts: []string{`{"name":"ok","count":1}`},
		}, nil
	})

	// Negative retry should still make the initial request
	resp, err := gollem.Query[testQueryResult](context.Background(), client, "test",
		gollem.WithQueryMaxRetry(-5),
	)
	gt.NoError(t, err)
	gt.Value(t, resp.Data.Name).Equal("ok")
	gt.Value(t, callCount).Equal(1)
}

func TestQueryRetryFeedbackMessage(t *testing.T) {
	var secondCallInputs []gollem.Input
	callCount := 0
	client := setupQueryMock(t, func(ctx context.Context, input ...gollem.Input) (*gollem.Response, error) {
		callCount++
		if callCount == 1 {
			return &gollem.Response{
				Texts: []string{`{broken`},
			}, nil
		}
		secondCallInputs = input
		return &gollem.Response{
			Texts: []string{`{"name":"ok","count":1}`},
		}, nil
	})

	_, err := gollem.Query[testQueryResult](context.Background(), client, "test")
	gt.NoError(t, err)
	// Verify that the retry input contains error feedback
	gt.Value(t, len(secondCallInputs)).Equal(1)
	feedbackText := secondCallInputs[0].String()
	gt.Value(t, feedbackText != "").Equal(true)
}
