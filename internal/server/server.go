package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/Digni/ding-ding/internal/config"
	"github.com/Digni/ding-ding/internal/notifier"
)

// Start launches the HTTP server that agents can POST to.
func Start(cfg config.Config) error {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /notify", func(w http.ResponseWriter, r *http.Request) {
		var msg notifier.Message
		if err := json.NewDecoder(r.Body).Decode(&msg); err != nil {
			http.Error(w, fmt.Sprintf("invalid JSON: %v", err), http.StatusBadRequest)
			return
		}

		if msg.Body == "" && msg.Title == "" {
			http.Error(w, "title or body required", http.StatusBadRequest)
			return
		}

		log.Printf("received notification: title=%q body=%q agent=%q", msg.Title, msg.Body, msg.Agent)

		if err := notifier.Notify(cfg, msg); err != nil {
			log.Printf("notification error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
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

		if err := notifier.Notify(cfg, msg); err != nil {
			log.Printf("notification error: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	// Health check
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintln(w, `{"status":"ok"}`)
	})

	log.Printf("ding-ding server listening on %s", cfg.Server.Address)
	return http.ListenAndServe(cfg.Server.Address, mux)
}
