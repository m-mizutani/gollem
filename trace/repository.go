package trace

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/m-mizutani/goerr/v2"
)

// Repository is the interface for persisting trace data.
type Repository interface {
	Save(ctx context.Context, trace *Trace) error
}

// FileRepository persists trace data as JSON files.
type FileRepository struct {
	dir string
}

// NewFileRepository creates a new FileRepository that writes to the given directory.
func NewFileRepository(dir string) *FileRepository {
	return &FileRepository{dir: dir}
}

// Save writes the trace as JSON to {dir}/{trace_id}.json.
func (r *FileRepository) Save(_ context.Context, trace *Trace) error {
	if err := os.MkdirAll(r.dir, 0750); err != nil {
		return goerr.Wrap(err, "failed to create trace directory", goerr.V("dir", r.dir))
	}

	data, err := json.MarshalIndent(trace, "", "  ")
	if err != nil {
		return goerr.Wrap(err, "failed to marshal trace")
	}

	filePath := filepath.Join(r.dir, trace.TraceID+".json")
	if err := os.WriteFile(filePath, data, 0600); err != nil {
		return goerr.Wrap(err, "failed to write trace file", goerr.V("path", filePath))
	}

	return nil
}
