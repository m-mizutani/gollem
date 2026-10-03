package ollama_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/gollem-dev/gollem"
	"github.com/m-mizutani/gt"
)

func TestAPIErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("JSON error body", func(t *testing.T) {
		fs := newFakeServer(t, func(w http.ResponseWriter, req recordedRequest) {
			writeJSON(w, http.StatusNotFound, map[string]any{"error": "model 'test-model' not found"})
		})
		session := newTestSession(t, newTestClient(t, fs))
		_, err := session.Generate(ctx, []gollem.Input{gollem.Text("hi")})
		gt.Error(t, err).Contains("status=404")
		gt.Error(t, err).Contains("model 'test-model' not found")

		// The input stays in the history and no assistant message is added.
		history, err := session.History()
		gt.NoError(t, err).Required()
		gt.A(t, history.Messages).Length(1).Required()
		gt.Equal(t, history.Messages[0].Role, gollem.RoleUser)
	})

	t.Run("body that is not JSON", func(t *testing.T) {
		fs := newFakeServer(t, func(w http.ResponseWriter, req recordedRequest) {
			w.WriteHeader(http.StatusBadGateway)
			_, _ = io.WriteString(w, "  upstream unavailable\n")
		})
		_, err := newTestSession(t, newTestClient(t, fs)).Generate(ctx, []gollem.Input{gollem.Text("hi")})
		gt.Error(t, err).Contains("status=502, message=upstream unavailable")
	})

	t.Run("error body larger than the read limit", func(t *testing.T) {
		fs := newFakeServer(t, func(w http.ResponseWriter, req recordedRequest) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, strings.Repeat("a", 2<<20))
		})
		_, err := newTestSession(t, newTestClient(t, fs)).Generate(ctx, []gollem.Input{gollem.Text("hi")})
		gt.Error(t, err).Required()
		gt.True(t, strings.Contains(err.Error(), strings.Repeat("a", 1<<20)))
		gt.False(t, strings.Contains(err.Error(), strings.Repeat("a", 1<<20+1)))
	})

	t.Run("invalid JSON response", func(t *testing.T) {
		fs := newFakeServer(t, func(w http.ResponseWriter, req recordedRequest) {
			w.WriteHeader(http.StatusOK)
			_, _ = io.WriteString(w, "{not json")
		})
		_, err := newTestSession(t, newTestClient(t, fs)).Generate(ctx, []gollem.Input{gollem.Text("hi")})
		gt.Error(t, err).Contains("failed to decode chat response")
	})

	t.Run("connection failure", func(t *testing.T) {
		fs := newFakeServer(t, replyText("ok", ""))
		client := newTestClient(t, fs)
		session := newTestSession(t, client)
		cctx, cancel := context.WithCancel(ctx)
		cancel()
		_, err := session.Generate(cctx, []gollem.Input{gollem.Text("hi")})
		gt.Error(t, err).Is(context.Canceled)
	})
}
