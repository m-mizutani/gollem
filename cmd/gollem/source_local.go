package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/m-mizutani/goerr/v2"
	"github.com/m-mizutani/gollem/trace"
)

type localSource struct {
	dir string
}

func newLocalSource(dir string) traceSource {
	return &localSource{dir: dir}
}

func (s *localSource) List(ctx context.Context, req listRequest) (*listResponse, error) {
	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, goerr.Wrap(err, "failed to read directory", goerr.Value("dir", s.dir))
	}

	// Filter and collect JSON files
	type fileEntry struct {
		name string
		info os.FileInfo
	}
	var files []fileEntry
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, fileEntry{name: e.Name(), info: info})
	}

	// Sort by name for deterministic ordering
	sort.Slice(files, func(i, j int) bool {
		return files[i].name < files[j].name
	})

	// Find the start position based on pageToken
	startIdx := 0
	if req.pageToken != "" {
		lastFile, err := decodePageToken(req.pageToken)
		if err != nil {
			return nil, goerr.Wrap(err, "invalid page token")
		}
		for i, f := range files {
			if f.name > lastFile {
				startIdx = i
				break
			}
		}
		// If no file is after the token, we're past the end
		if startIdx == 0 && len(files) > 0 && files[len(files)-1].name <= lastFile {
			return &listResponse{}, nil
		}
	}

	pageSize := req.pageSize
	if pageSize <= 0 {
		pageSize = 20
	}

	endIdx := startIdx + pageSize
	if endIdx > len(files) {
		endIdx = len(files)
	}

	resp := &listResponse{}
	for _, f := range files[startIdx:endIdx] {
		traceID := strings.TrimSuffix(f.name, ".json")
		resp.traces = append(resp.traces, traceSummary{
			TraceID:   traceID,
			Size:      f.info.Size(),
			UpdatedAt: f.info.ModTime(),
		})
	}

	if endIdx < len(files) {
		resp.nextPageToken = encodePageToken(files[endIdx-1].name)
	}

	return resp, nil
}

func (s *localSource) Get(ctx context.Context, traceID string) (*trace.Trace, error) {
	filePath := filepath.Clean(filepath.Join(s.dir, traceID+".json"))

	data, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, goerr.Wrap(err, "trace not found", goerr.Value("traceID", traceID))
		}
		return nil, goerr.Wrap(err, "failed to read trace file", goerr.Value("traceID", traceID))
	}

	var t trace.Trace
	if err := json.Unmarshal(data, &t); err != nil {
		return nil, goerr.Wrap(err, "failed to parse trace file", goerr.Value("traceID", traceID))
	}

	return &t, nil
}

func encodePageToken(fileName string) string {
	return base64.URLEncoding.EncodeToString([]byte(fileName))
}

func decodePageToken(token string) (string, error) {
	b, err := base64.URLEncoding.DecodeString(token)
	if err != nil {
		return "", goerr.Wrap(err, "failed to decode page token")
	}
	return string(b), nil
}
