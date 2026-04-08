// Copyright (C) 2026 Yota Hamada
// SPDX-License-Identifier: GPL-3.0-or-later

package sse

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// TopicMutationRequest updates the topic set for a multiplexed SSE session.
type TopicMutationRequest struct {
	SessionID string   `json:"sessionID"`
	Add       []string `json:"add"`
	Remove    []string `json:"remove"`
}

// MultiplexHandler serves the multiplexed SSE stream and topic mutation API.
type MultiplexHandler struct {
	mux *Multiplexer
}

// NewMultiplexHandler creates a handler for multiplexed SSE endpoints.
func NewMultiplexHandler(mux *Multiplexer) *MultiplexHandler {
	return &MultiplexHandler{
		mux: mux,
	}
}

// HandleStream opens the multiplexed SSE stream.
func (h *MultiplexHandler) HandleStream(w http.ResponseWriter, r *http.Request) {
	SetSSEHeaders(w)
	rc := http.NewResponseController(w)
	_ = rc.SetWriteDeadline(time.Time{})

	lastEventID, err := parseLastEventID(r)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_last_event_id", err.Error())
		return
	}

	result, err := h.mux.createSession(r.Context(), w, parseInitialTopics(r.URL.Query()), lastEventID)
	if err != nil {
		http.Error(w, "unable to open SSE stream", http.StatusServiceUnavailable)
		return
	}
	defer h.mux.removeSession(result.session)

	if err := result.session.writeControl(result.control); err != nil {
		return
	}
	result.session.bootstrapTopics(r.Context(), lastEventID, result.topics)
	_ = result.session.Serve(r.Context())
}

// HandleTopicMutation adds and removes topics for an existing stream.
func (h *MultiplexHandler) HandleTopicMutation(w http.ResponseWriter, r *http.Request) {
	var req TopicMutationRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&req); err != nil {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "invalid JSON request body")
		return
	}
	if req.SessionID == "" {
		writeJSONError(w, http.StatusBadRequest, "invalid_request", "sessionID is required")
		return
	}

	result, err := h.mux.mutateSession(r.Context(), req.SessionID, req.Add, req.Remove)
	if err != nil {
		switch {
		case errors.Is(err, ErrUnknownSession):
			writeJSONError(w, http.StatusNotFound, "unknown_session", "unknown_session")
		case errors.Is(err, ErrTooManyTopics):
			writeJSONError(w, http.StatusBadRequest, "too_many_topics", err.Error())
		case errors.Is(err, ErrConflictingTopicMutation):
			writeJSONError(w, http.StatusBadRequest, "invalid_request", err.Error())
		default:
			writeJSONError(w, http.StatusBadRequest, "invalid_topic", err.Error())
		}
		return
	}

	if len(result.added) > 0 {
		if session, sessionErr := h.mux.getSession(req.SessionID); sessionErr == nil {
			session.bootstrapTopics(r.Context(), 0, result.added)
		}
	}

	writeJSON(w, result.statusCode, result.response)
}

func parseLastEventID(r *http.Request) (uint64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get("lastEventId"))
	if raw == "" {
		raw = strings.TrimSpace(r.Header.Get("Last-Event-ID"))
	}
	if raw == "" {
		return 0, nil
	}

	var lastEventID uint64
	if _, err := fmt.Sscanf(raw, "%d", &lastEventID); err != nil {
		return 0, err
	}
	return lastEventID, nil
}

func writeJSON(w http.ResponseWriter, statusCode int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(value)
}

func writeJSONError(w http.ResponseWriter, statusCode int, code, message string) {
	writeJSON(w, statusCode, map[string]string{
		"error":   code,
		"message": message,
	})
}
