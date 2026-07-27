package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/runtime/internal/engine"
)

// Server serves the runtime protection API.
type Server struct {
	engine *engine.Engine
	log    *logging.Logger
}

// NewServer creates a new runtime protection API server.
func NewServer(eng *engine.Engine, log *logging.Logger) *Server {
	return &Server{engine: eng, log: log}
}

// Handler returns an HTTP handler with runtime protection routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/runtime/event", s.IngestEvent)
	mux.HandleFunc("POST /api/v1/runtime/events", s.IngestEvents)
	mux.HandleFunc("POST /api/v1/runtime/falco-webhook", s.FalcoWebhook)
	return mux
}

// IngestEvent handles POST /api/v1/runtime/event
func (s *Server) IngestEvent(w http.ResponseWriter, r *http.Request) {
	var event engine.FalcoEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event payload"})
		return
	}

	if err := s.engine.ProcessEvent(r.Context(), event); err != nil {
		s.log.Error("processing runtime event", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to process event"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
}

// IngestEvents handles POST /api/v1/runtime/events
func (s *Server) IngestEvents(w http.ResponseWriter, r *http.Request) {
	var events []engine.FalcoEvent
	if err := json.NewDecoder(r.Body).Decode(&events); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid events payload"})
		return
	}

	count, err := s.engine.ProcessEvents(r.Context(), events)
	if err != nil {
		s.log.Warn("partial events processing", "processed", count, "total", len(events), "error", err)
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"status":           "accepted",
		"events_received":  len(events),
		"events_processed": count,
	})
}

// FalcoWebhook handles POST /api/v1/runtime/falco-webhook
// This is the endpoint Falco's webhook output plugin posts to.
func (s *Server) FalcoWebhook(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "failed to read body"})
		return
	}
	defer r.Body.Close()

	// Falco can send events as a JSON array or a single event
	var singleEvent engine.FalcoEvent
	if err := json.Unmarshal(body, &singleEvent); err == nil && singleEvent.Rule != "" {
		if err := s.engine.ProcessEvent(r.Context(), singleEvent); err != nil {
			s.log.Error("processing falco webhook event", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to process event"})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "accepted"})
		return
	}

	var events []engine.FalcoEvent
	if err := json.Unmarshal(body, &events); err == nil {
		count, err := s.engine.ProcessEvents(r.Context(), events)
		if err != nil {
			s.log.Warn("partial falco webhook processing", "processed", count, "total", len(events), "error", err)
		}
		writeJSON(w, http.StatusOK, map[string]any{
			"status":           "accepted",
			"events_received":  len(events),
			"events_processed": count,
		})
		return
	}

	writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid falco event format"})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
