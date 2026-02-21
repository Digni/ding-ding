package server

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/logging"
	"github.com/Digni/ding-ding/internal/notifier"
)

type errorResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		slog.Error("server.response.encode_failed", "error", err)
	}
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, errorResponse{Code: code, Message: message})
}

// NewMux builds the HTTP handler for ding-ding's server endpoints.
func NewMux(cfg config.Config, logger *slog.Logger) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /notify", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := logging.EnsureRequestID(r.Header.Get(logging.RequestIDHeader))
		operationID := logging.NewOperationID()
		logger := logger.With("request_id", requestID, "operation_id", operationID, "method", r.Method, "path", r.URL.Path)
		logger.Info("server.notify.request.started")

		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		rawBody, err := io.ReadAll(r.Body)
		if err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				meta := logging.PayloadMetadataFromBody(nil, r.ContentLength, "json")
				logger.Warn("server.notify.request.rejected", append(meta.Fields(), "status", "error", "error_code", "request_too_large", "duration_ms", time.Since(start).Milliseconds())...)
				writeJSONError(w, http.StatusRequestEntityTooLarge, "request_too_large", "request body too large")
				return
			}
			logger.Error("server.notify.request.read_failed", "status", "error", "duration_ms", time.Since(start).Milliseconds(), "error", err)
			writeJSONError(w, http.StatusBadRequest, "invalid_request_body", "invalid request body")
			return
		}

		payloadMeta := logging.PayloadMetadataFromBody(rawBody, r.ContentLength, "json")
		logger.Info("server.notify.request.payload", payloadMeta.Fields()...)

		var msg notifier.Message
		if err := json.Unmarshal(rawBody, &msg); err != nil {
			logger.Warn("server.notify.request.rejected", append(payloadMeta.Fields(), "status", "error", "error_code", "invalid_request_body", "duration_ms", time.Since(start).Milliseconds(), "error", err)...)
			writeJSONError(w, http.StatusBadRequest, "invalid_request_body", "invalid request body")
			return
		}

		if msg.Body == "" && msg.Title == "" {
			logger.Warn("server.notify.request.rejected", append(payloadMeta.Fields(), "status", "error", "error_code", "missing_content", "duration_ms", time.Since(start).Milliseconds())...)
			writeJSONError(w, http.StatusBadRequest, "missing_content", "title or body required")
			return
		}

		msg.RequestID = requestID
		msg.OperationID = operationID

		if err := notifier.NotifyRemote(cfg, msg); err != nil {
			logger.Error("server.notify.request.completed", append(payloadMeta.Fields(), "status", "error", "status_code", http.StatusInternalServerError, "duration_ms", time.Since(start).Milliseconds(), "error", err)...)
			writeJSONError(w, http.StatusInternalServerError, "notification_delivery_failed", "notification delivery failed")
			return
		}

		logger.Info("server.notify.request.completed", append(payloadMeta.Fields(), "status", "ok", "status_code", http.StatusOK, "duration_ms", time.Since(start).Milliseconds())...)

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Simple GET endpoint for quick curl usage
	mux.HandleFunc("GET /notify", func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		requestID := logging.EnsureRequestID(r.Header.Get(logging.RequestIDHeader))
		operationID := logging.NewOperationID()
		logger := logger.With("request_id", requestID, "operation_id", operationID, "method", r.Method, "path", r.URL.Path)
		logger.Info("server.notify.request.started")

		msg := notifier.Message{
			Title: r.URL.Query().Get("title"),
			Body:  r.URL.Query().Get("message"),
			Agent: r.URL.Query().Get("agent"),
		}
		payloadMeta := logging.PayloadMetadataFromQuery(queryFieldNames(r), int64(len(r.URL.RawQuery)))
		logger.Info("server.notify.request.payload", payloadMeta.Fields()...)

		if msg.Body == "" && msg.Title == "" {
			msg.Title = "ding ding!"
			msg.Body = "Agent task completed"
		}

		msg.RequestID = requestID
		msg.OperationID = operationID

		if err := notifier.NotifyRemote(cfg, msg); err != nil {
			logger.Error("server.notify.request.completed", append(payloadMeta.Fields(), "status", "error", "status_code", http.StatusInternalServerError, "duration_ms", time.Since(start).Milliseconds(), "error", err)...)
			writeJSONError(w, http.StatusInternalServerError, "notification_delivery_failed", "notification delivery failed")
			return
		}

		logger.Info("server.notify.request.completed", append(payloadMeta.Fields(), "status", "ok", "status_code", http.StatusOK, "duration_ms", time.Since(start).Milliseconds())...)

		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})

	return mux
}

// Start launches the HTTP server that agents can POST to.
func Start(cfg config.Config) error {
	mux := NewMux(cfg, slog.Default())
	slog.Info("server.started", "address", cfg.Server.Address)
	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.ListenAndServe()
}

func queryFieldNames(r *http.Request) []string {
	query := r.URL.Query()
	names := make([]string, 0, len(query))
	for field := range query {
		names = append(names, field)
	}
	return names
}
