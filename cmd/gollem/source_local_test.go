package main_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	main "github.com/m-mizutani/gollem/cmd/gollem"
	"github.com/m-mizutani/gt"
)

func TestLocalSourceList(t *testing.T) {
	ctx := context.Background()

	t.Run("list all traces", func(t *testing.T) {
		src := main.NewLocalSource("testdata")
		resp := gt.R1(src.List(ctx, 10, "")).NoError(t)
		gt.Equal(t, 3, len(resp.Traces))
		gt.Equal(t, "trace-001", resp.Traces[0].TraceID)
		gt.Equal(t, "trace-002", resp.Traces[1].TraceID)
		gt.Equal(t, "trace-003", resp.Traces[2].TraceID)
		gt.Equal(t, "", resp.NextPageToken)
	})

	t.Run("pagination first page", func(t *testing.T) {
		src := main.NewLocalSource("testdata")
		resp := gt.R1(src.List(ctx, 2, "")).NoError(t)
		gt.Equal(t, 2, len(resp.Traces))
		gt.Equal(t, "trace-001", resp.Traces[0].TraceID)
		gt.Equal(t, "trace-002", resp.Traces[1].TraceID)
		gt.True(t, resp.NextPageToken != "")
	})

	t.Run("pagination second page", func(t *testing.T) {
		src := main.NewLocalSource("testdata")
		resp1 := gt.R1(src.List(ctx, 2, "")).NoError(t)

		resp2 := gt.R1(src.List(ctx, 2, resp1.NextPageToken)).NoError(t)
		gt.Equal(t, 1, len(resp2.Traces))
		gt.Equal(t, "trace-003", resp2.Traces[0].TraceID)
		gt.Equal(t, "", resp2.NextPageToken)
	})

	t.Run("empty directory", func(t *testing.T) {
		dir := t.TempDir()
		src := main.NewLocalSource(dir)
		resp := gt.R1(src.List(ctx, 10, "")).NoError(t)
		gt.Equal(t, 0, len(resp.Traces))
	})

	t.Run("non-existent directory", func(t *testing.T) {
		src := main.NewLocalSource("/nonexistent")
		_, err := src.List(ctx, 10, "")
		gt.Error(t, err)
	})

	t.Run("default page size", func(t *testing.T) {
		src := main.NewLocalSource("testdata")
		resp := gt.R1(src.List(ctx, 0, "")).NoError(t)
		gt.Equal(t, 3, len(resp.Traces))
	})
}

func TestLocalSourceGet(t *testing.T) {
	ctx := context.Background()

	t.Run("get existing trace", func(t *testing.T) {
		src := main.NewLocalSource("testdata")
		tr := gt.R1(src.Get(ctx, "trace-001")).NoError(t)
		gt.Equal(t, "trace-001", tr.TraceID)
		gt.Equal(t, "gpt-4", tr.Metadata.Model)
		gt.NotEqual(t, nil, tr.RootSpan)
		gt.Equal(t, 2, len(tr.RootSpan.Children))
	})

	t.Run("get non-existent trace", func(t *testing.T) {
		src := main.NewLocalSource("testdata")
		_, err := src.Get(ctx, "non-existent")
		gt.Error(t, err)
	})

	t.Run("get trace with error status", func(t *testing.T) {
		src := main.NewLocalSource("testdata")
		tr := gt.R1(src.Get(ctx, "trace-002")).NoError(t)
		gt.Equal(t, "trace-002", tr.TraceID)
		gt.Equal(t, "error", string(tr.RootSpan.Status))
		gt.Equal(t, "tool execution failed", tr.RootSpan.Error)
	})

	t.Run("invalid json file", func(t *testing.T) {
		dir := t.TempDir()
		err := os.WriteFile(filepath.Join(dir, "bad.json"), []byte("not json"), 0644)
		gt.NoError(t, err)

		src := main.NewLocalSource(dir)
		_, err = src.Get(ctx, "bad")
		gt.Error(t, err)
	})
}
