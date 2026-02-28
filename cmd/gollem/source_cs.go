package main

import (
	"context"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"time"

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
	query := &storage.Query{
		Prefix: s.prefix,
	}

	bkt := s.client.Bucket(s.bucket)
	it := bkt.Objects(ctx, query)

	// Collect all matching objects from GCS
	type objectEntry struct {
		name      string
		size      int64
		updatedAt time.Time
	}
	var objects []objectEntry

	for {
		attr, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, goerr.Wrap(err, "failed to list objects",
				goerr.Value("bucket", s.bucket),
				goerr.Value("prefix", s.prefix),
			)
		}

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

		objects = append(objects, objectEntry{
			name:      traceID,
			size:      attr.Size,
			updatedAt: attr.Updated,
		})
	}

	// Sort by name for deterministic ordering
	sort.Slice(objects, func(i, j int) bool {
		return objects[i].name < objects[j].name
	})

	// Find the start position based on pageToken
	startIdx := 0
	if req.pageToken != "" {
		lastFile, err := decodePageToken(req.pageToken)
		if err != nil {
			return nil, goerr.Wrap(err, "invalid page token")
		}
		startIdx = sort.Search(len(objects), func(i int) bool {
			return objects[i].name > lastFile
		})
		if startIdx >= len(objects) {
			return &listResponse{}, nil
		}
	}

	pageSize := req.pageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	endIdx := startIdx + pageSize
	if endIdx > len(objects) {
		endIdx = len(objects)
	}

	resp := &listResponse{}
	for _, obj := range objects[startIdx:endIdx] {
		resp.traces = append(resp.traces, traceSummary{
			TraceID:   obj.name,
			Size:      obj.size,
			UpdatedAt: obj.updatedAt,
		})
	}

	if endIdx < len(objects) {
		resp.nextPageToken = encodePageToken(objects[endIdx-1].name)
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
