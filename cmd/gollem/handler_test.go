package main_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	main "github.com/m-mizutani/gollem/cmd/gollem"
	"github.com/m-mizutani/gt"
)

func TestHandleHealth(t *testing.T) {
	src := main.NewLocalSource("testdata")
	s := main.NewServer(main.WithTestSource(src))

	req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
	rec := httptest.NewRecorder()
	s.Handler().ServeHTTP(rec, req)

	gt.Equal(t, http.StatusOK, rec.Code)

	var resp map[string]string
	gt.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	gt.Equal(t, "ok", resp["status"])
}

func TestHandleListTraces(t *testing.T) {
	src := main.NewLocalSource("testdata")
	s := main.NewServer(main.WithTestSource(src))

	t.Run("list all traces", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/traces", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)

		gt.Equal(t, http.StatusOK, rec.Code)

		var resp main.ListTracesResponse
		gt.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		gt.Equal(t, 3, len(resp.Traces))
	})

	t.Run("with page size", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/traces?page_size=2", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)

		gt.Equal(t, http.StatusOK, rec.Code)

		var resp main.ListTracesResponse
		gt.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		gt.Equal(t, 2, len(resp.Traces))
		gt.True(t, resp.NextPageToken != "")
	})

	t.Run("invalid page size", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/traces?page_size=abc", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)

		gt.Equal(t, http.StatusBadRequest, rec.Code)
	})
}

func TestHandleGetTrace(t *testing.T) {
	src := main.NewLocalSource("testdata")
	s := main.NewServer(main.WithTestSource(src))

	t.Run("get existing trace", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/traces/trace-001", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)

		gt.Equal(t, http.StatusOK, rec.Code)

		var resp map[string]any
		gt.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
		gt.Equal(t, "trace-001", resp["trace_id"])
	})

	t.Run("get non-existent trace", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/traces/nonexistent", nil)
		rec := httptest.NewRecorder()
		s.Handler().ServeHTTP(rec, req)

		gt.Equal(t, http.StatusNotFound, rec.Code)
	})
}
