package server

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"ccp/internal/anthropic"
	"ccp/internal/model"
	"ccp/internal/providers"
	"ccp/internal/usage"
)

func (s *Server) handleMessages(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	requestID := fmt.Sprintf("req_%d", time.Now().UnixNano())
	status := http.StatusOK
	var route model.Route
	var usageRecord usage.Record
	var requestErr string
	defer func() {
		duration := time.Since(start).Milliseconds()
		s.logger.Info("request",
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"alias", route.Alias,
			"provider", route.Provider,
			"model", route.Model,
			"status", status,
			"duration_ms", duration,
		)
		if s.cfg.Usage.Enabled && route.Provider != "" {
			usageRecord.RequestID = requestID
			usageRecord.Timestamp = start.Format(time.RFC3339)
			usageRecord.Provider = route.Provider
			usageRecord.Model = route.Model
			usageRecord.Alias = route.Alias
			usageRecord.Stream = usageRecord.Stream || false
			usageRecord.Status = status
			usageRecord.DurationMS = duration
			usageRecord.Error = requestErr
			if err := usage.Append(s.cfg.Usage.File, usageRecord); err != nil {
				s.logger.Warn("usage write failed", "request_id", requestID, "error", err.Error())
			}
		}
	}()

	releaseGlobal, err := s.globalLimiter.acquire(r.Context())
	if err != nil {
		status = http.StatusServiceUnavailable
		requestErr = "server busy"
		writeError(w, status, requestErr)
		return
	}
	defer releaseGlobal()

	var req anthropic.MessageRequest
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&req); err != nil {
		status = http.StatusBadRequest
		requestErr = err.Error()
		writeError(w, status, err.Error())
		return
	}
	if req.Model == "" {
		status = http.StatusBadRequest
		requestErr = "missing model"
		writeError(w, status, "missing model")
		return
	}
	resolved, err := model.Resolve(req.Model, s.cfg.Aliases)
	if err != nil {
		status = http.StatusBadRequest
		requestErr = err.Error()
		writeError(w, status, err.Error())
		return
	}
	route = resolved
	provider, ok := s.providers[route.Provider]
	if !ok {
		status = http.StatusBadRequest
		requestErr = fmt.Sprintf("provider %q not configured", route.Provider)
		writeError(w, status, fmt.Sprintf("provider %q not configured", route.Provider))
		return
	}
	providerRoute := providers.Route(route)
	if limiter := s.providerLimiter[route.Provider]; limiter != nil {
		releaseProvider, err := limiter.acquire(r.Context())
		if err != nil {
			status = http.StatusServiceUnavailable
			requestErr = fmt.Sprintf("provider %q busy", route.Provider)
			writeError(w, status, requestErr)
			return
		}
		defer releaseProvider()
	}
	usageRecord.Stream = req.Stream
	if req.Stream {
		body, contentType, err := provider.StreamMessages(r.Context(), providerRoute, req)
		if err != nil {
			status = http.StatusBadGateway
			requestErr = err.Error()
			writeError(w, status, err.Error())
			return
		}
		defer body.Close()
		w.Header().Set("Content-Type", contentType)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(http.StatusOK)
		_, _ = io.Copy(w, body)
		return
	}

	resp, err := provider.Messages(r.Context(), providerRoute, req)
	if err != nil {
		status = http.StatusBadGateway
		requestErr = err.Error()
		writeError(w, status, err.Error())
		return
	}
	if resp.Usage != nil {
		usageRecord.InputTokens = resp.Usage.InputTokens
		usageRecord.OutputTokens = resp.Usage.OutputTokens
		usageRecord.TotalTokens = resp.Usage.InputTokens + resp.Usage.OutputTokens
	}
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		s.logger.Warn("write response failed", "request_id", requestID, "error", err.Error())
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]any{
		"type":  "error",
		"error": map[string]string{"message": message},
	})
}
