// Package main demonstrates how to implement HistoryRepository and use it with gollem
// to persist conversation history across sessions.
//
// This example shows a filesystem-based implementation of gollem.HistoryRepository.
// For production use, you can implement the same interface using any storage backend
// (S3, GCS, a database, etc.).
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/m-mizutani/gollem"
	"github.com/m-mizutani/gollem/llm/openai"
)

// FileRepository is a sample HistoryRepository that stores History as JSON files.
// It demonstrates how to implement gollem.HistoryRepository for any storage backend.
type FileRepository struct {
	dir string
}

// NewFileRepository creates a FileRepository that stores files in dir.
func NewFileRepository(dir string) *FileRepository {
	return &FileRepository{dir: dir}
}

// Load retrieves History by session ID. Returns nil if the session does not exist yet.
func (r *FileRepository) Load(ctx context.Context, sessionID string) (*gollem.History, error) {
	path := filepath.Join(r.dir, sessionID+".json")
	data, err := os.ReadFile(path) // #nosec G304 -- sessionID should be validated by callers
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil // new session
		}
		return nil, fmt.Errorf("read history: %w", err)
	}

	var h gollem.History
	if err := json.Unmarshal(data, &h); err != nil {
		return nil, fmt.Errorf("unmarshal history: %w", err)
	}
	return &h, nil
}

// Save persists History for the given session ID, overwriting any previous value.
func (r *FileRepository) Save(ctx context.Context, sessionID string, history *gollem.History) error {
	if err := os.MkdirAll(r.dir, 0750); err != nil {
		return fmt.Errorf("mkdir: %w", err)
	}
	data, err := json.Marshal(history)
	if err != nil {
		return fmt.Errorf("marshal history: %w", err)
	}
	path := filepath.Join(r.dir, sessionID+".json")
	if err := os.WriteFile(path, data, 0600); err != nil { // #nosec G304
		return fmt.Errorf("write history: %w", err)
	}
	return nil
}

func main() {
	ctx := context.Background()

	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "OPENAI_API_KEY is not set")
		os.Exit(1)
	}

	client, err := openai.New(ctx, apiKey)
	if err != nil {
		fmt.Fprintln(os.Stderr, "failed to create openai client:", err)
		os.Exit(1)
	}

	// Use /tmp/gollem-history as the storage directory.
	repo := NewFileRepository("/tmp/gollem-history")

	// The session ID identifies this conversation across runs.
	// Reusing the same ID resumes the previous conversation.
	sessionID := "demo-session"

	agent := gollem.New(client, gollem.WithHistoryRepository(repo, sessionID))

	resp, err := agent.Execute(ctx, gollem.Text("Hello! What's 1+1?"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(resp)

	// Run again with the same agent — history is already in memory.
	resp, err = agent.Execute(ctx, gollem.Text("What did I just ask you?"))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	fmt.Println(resp)
}
