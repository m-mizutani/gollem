package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
)

type apiError struct {
	Error string `json:"error"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("failed to encode JSON response", slog.Any("error", err))
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiError{Error: msg})
}

func (s *server) handleHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

type listEntriesResponse struct {
	Path          string         `json:"path"`
	Entries       []entrySummary `json:"entries"`
	NextPageToken string         `json:"next_page_token,omitempty"`
}

func (s *server) handleListTraces(w http.ResponseWriter, r *http.Request) {
	pageSizeStr := r.URL.Query().Get("page_size")
	pageToken := r.URL.Query().Get("page_token")
	path := r.URL.Query().Get("path")

	cleaned, err := cleanRelativePath(path)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	pageSize := 20
	if pageSizeStr != "" {
		n, err := strconv.Atoi(pageSizeStr)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid page_size parameter")
			return
		}
		const maxPageSize = 1000
		if n > maxPageSize {
			n = maxPageSize
		}
		pageSize = n
	}

	resp, err := s.source.List(r.Context(), listRequest{
		path:      cleaned,
		pageSize:  pageSize,
		pageToken: pageToken,
	})
	if err != nil {
		slog.Error("failed to list traces", slog.Any("error", err), slog.String("path", cleaned))
		writeError(w, http.StatusInternalServerError, "failed to list traces")
		return
	}

	entries := resp.entries
	if entries == nil {
		entries = []entrySummary{}
	}

	writeJSON(w, http.StatusOK, listEntriesResponse{
		Path:          cleaned,
		Entries:       entries,
		NextPageToken: resp.nextPageToken,
	})
}

func (s *server) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	// The "{path...}" wildcard path value preserves slashes in the matched portion.
	tracePath := r.PathValue("path")
	tracePath = strings.TrimPrefix(tracePath, "/")
	if tracePath == "" {
		writeError(w, http.StatusBadRequest, "trace path is required")
		return
	}

	cleaned, err := cleanRelativePath(tracePath)
	if err != nil || cleaned == "" {
		writeError(w, http.StatusBadRequest, "invalid trace path")
		return
	}

	t, err := s.source.Get(r.Context(), cleaned)
	if err != nil {
		slog.Error("failed to get trace", slog.Any("error", err), slog.String("path", cleaned))
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}

	writeJSON(w, http.StatusOK, t)
}
