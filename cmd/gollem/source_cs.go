package main

import (
	"context"
	"encoding/json"
	"io"
	"strings"

	"cloud.google.com/go/storage"
	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem/trace"
	"google.golang.org/api/iterator"
)

type csSource struct {
	bucket string
	prefix string
	client *storage.Client
}

func newCSSource(ctx context.Context, bucket, prefix string) (traceSource, error) {
	client, err := storage.NewClient(ctx)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to create Cloud Storage client")
	}
	return &csSource{
		bucket: bucket,
		prefix: prefix,
		client: client,
	}, nil
}

func (s *csSource) List(ctx context.Context, req listRequest) (*listResponse, error) {
	pageSize := req.pageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	query := &storage.Query{
		Prefix: s.prefix,
	}

	bkt := s.client.Bucket(s.bucket)
	it := bkt.Objects(ctx, query)

	// Set page size via pager
	pager := iterator.NewPager(it, pageSize, req.pageToken)
	var attrs []*storage.ObjectAttrs
	nextToken, err := pager.NextPage(&attrs)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to list objects",
			goerr.Value("bucket", s.bucket),
			goerr.Value("prefix", s.prefix),
		)
	}

	resp := &listResponse{
		nextPageToken: nextToken,
	}

	for _, attr := range attrs {
		if !strings.HasSuffix(attr.Name, ".json") {
			continue
		}
		name := attr.Name
		if s.prefix != "" {
			name = strings.TrimPrefix(name, s.prefix)
		}
		traceID := strings.TrimSuffix(name, ".json")
		// Skip directory-like entries
		if traceID == "" || strings.Contains(traceID, "/") {
			continue
		}

		resp.traces = append(resp.traces, traceSummary{
			TraceID:   traceID,
			Size:      attr.Size,
			UpdatedAt: attr.Updated,
		})
	}

	return resp, nil
}

func (s *csSource) Get(ctx context.Context, traceID string) (*trace.Trace, error) {
	objectName := s.prefix + traceID + ".json"
	reader, err := s.client.Bucket(s.bucket).Object(objectName).NewReader(ctx)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read trace object",
			goerr.Value("bucket", s.bucket),
			goerr.Value("object", objectName),
		)
	}
	defer func() { _ = reader.Close() }()

	data, err := io.ReadAll(reader)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read trace data",
			goerr.Value("bucket", s.bucket),
			goerr.Value("object", objectName),
		)
	}

	var t trace.Trace
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, goerr.Wrap(err, "failed to parse trace data",
			goerr.Value("bucket", s.bucket),
			goerr.Value("object", objectName),
		)
	}

	return &t, nil
}
