package main

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"
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

type listTracesResponse struct {
	Traces        []traceSummary `json:"traces"`
	NextPageToken string         `json:"next_page_token,omitempty"`
}

func (s *server) handleListTraces(w http.ResponseWriter, r *http.Request) {
	pageSizeStr := r.URL.Query().Get("page_size")
	pageToken := r.URL.Query().Get("page_token")

	pageSize := 20
	if pageSizeStr != "" {
		n, err := strconv.Atoi(pageSizeStr)
		if err != nil || n <= 0 {
			writeError(w, http.StatusBadRequest, "invalid page_size parameter")
			return
		}
		pageSize = n
	}

	resp, err := s.source.List(r.Context(), listRequest{
		pageSize:  pageSize,
		pageToken: pageToken,
	})
	if err != nil {
		slog.Error("failed to list traces", slog.Any("error", err))
		writeError(w, http.StatusInternalServerError, "failed to list traces")
		return
	}

	traces := resp.traces
	if traces == nil {
		traces = []traceSummary{}
	}

	writeJSON(w, http.StatusOK, listTracesResponse{
		Traces:        traces,
		NextPageToken: resp.nextPageToken,
	})
}

func (s *server) handleGetTrace(w http.ResponseWriter, r *http.Request) {
	traceID := r.PathValue("id")
	if traceID == "" {
		writeError(w, http.StatusBadRequest, "trace ID is required")
		return
	}

	t, err := s.source.Get(r.Context(), traceID)
	if err != nil {
		slog.Error("failed to get trace", slog.Any("error", err), slog.String("traceID", traceID))
		writeError(w, http.StatusNotFound, "trace not found")
		return
	}

	writeJSON(w, http.StatusOK, t)
}
