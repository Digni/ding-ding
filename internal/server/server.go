package server

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"time"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/notifier"
)

// NewMux builds the HTTP handler for ding-ding's server endpoints.
func NewMux(cfg config.Config) *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /notify", func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, 1<<16)
		var msg notifier.Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) {
				http.Error(w, "request body too large", http.StatusRequestEntityTooLarge)
				return
			}
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if msg.Body == "" && msg.Title == "" {
			http.Error(w, "title or body required", http.StatusBadRequest)
			return
		}

		log.Printf("received notification: title=%q body=%q agent=%q", msg.Title, msg.Body, msg.Agent)

		if err := notifier.NotifyRemote(cfg, msg); err != nil {
			log.Printf("notification error: %v", err)
			http.Error(w, "notification delivery failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}` + "\n"))
	})

	// Simple GET endpoint for quick curl usage
	mux.HandleFunc("GET /notify", func(w http.ResponseWriter, r *http.Request) {
		msg := notifier.Message{
			Title: r.URL.Query().Get("title"),
			Body:  r.URL.Query().Get("message"),
			Agent: r.URL.Query().Get("agent"),
		}

		if msg.Body == "" && msg.Title == "" {
			msg.Title = "ding ding!"
			msg.Body = "Agent task completed"
		}

		log.Printf("received notification (GET): title=%q body=%q agent=%q", msg.Title, msg.Body, msg.Agent)

		if err := notifier.NotifyRemote(cfg, msg); err != nil {
			log.Printf("notification error: %v", err)
			http.Error(w, "notification delivery failed", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}` + "\n"))
	})

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ok"}` + "\n"))
	})

	return mux
}

// Start launches the HTTP server that agents can POST to.
func Start(cfg config.Config) error {
	mux := NewMux(cfg)
	log.Printf("ding-ding server listening on %s", cfg.Server.Address)
	srv := &http.Server{
		Addr:         cfg.Server.Address,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	return srv.ListenAndServe()
}
