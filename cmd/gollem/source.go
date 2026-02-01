package main

import (
	"context"
	"time"

	"github.com/m-mizutani/gollem/trace"
)

// traceSummary is a lightweight representation of a trace,
// derived from object metadata without reading the file contents.
type traceSummary struct {
	TraceID   string    `json:"trace_id"`
	Size      int64     `json:"size"`
	UpdatedAt time.Time `json:"updated_at"`
}

type listRequest struct {
	pageSize  int
	pageToken string
}

type listResponse struct {
	traces        []traceSummary
	nextPageToken string
}

// traceSource provides access to trace data from various backends.
type traceSource interface {
	List(ctx context.Context, req listRequest) (*listResponse, error)
	Get(ctx context.Context, traceID string) (*trace.Trace, error)
}
