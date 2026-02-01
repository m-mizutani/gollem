package main

import (
	"context"
	"net/http"

	"github.com/m-mizutani/gollem/trace"
)

// ListTracesResponse is exported for testing.
type ListTracesResponse = listTracesResponse

// Exported constructors for testing
var NewServer = newServer

// Exported server options for testing
var WithSource = withSource
var WithAddr = withAddr
var WithNoBrowser = withNoBrowser

// Handler returns the server's HTTP handler for testing.
func (s *server) Handler() http.Handler {
	return s.handler()
}

// TraceSummaryExported is an exported version of traceSummary for testing.
type TraceSummaryExported = traceSummary

// ListResult holds the exported result of a List call.
type ListResult struct {
	Traces        []TraceSummaryExported
	NextPageToken string
}

// TestableSource wraps a traceSource for external test access.
type TestableSource struct {
	src traceSource
}

// NewLocalSource creates a TestableSource backed by localSource.
func NewLocalSource(dir string) *TestableSource {
	return &TestableSource{src: newLocalSource(dir)}
}

// List calls the underlying source's List with exported types.
func (ts *TestableSource) List(ctx context.Context, pageSize int, pageToken string) (*ListResult, error) {
	resp, err := ts.src.List(ctx, listRequest{
		pageSize:  pageSize,
		pageToken: pageToken,
	})
	if err != nil {
		return nil, err
	}
	return &ListResult{
		Traces:        resp.traces,
		NextPageToken: resp.nextPageToken,
	}, nil
}

// Get calls the underlying source's Get.
func (ts *TestableSource) Get(ctx context.Context, traceID string) (*trace.Trace, error) {
	return ts.src.Get(ctx, traceID)
}

// AsTraceSource returns the underlying traceSource for use with server options.
func (ts *TestableSource) AsTraceSource() traceSource {
	return ts.src
}

// WithTestSource creates a server option from a TestableSource.
func WithTestSource(ts *TestableSource) serverOption {
	return withSource(ts.src)
}
