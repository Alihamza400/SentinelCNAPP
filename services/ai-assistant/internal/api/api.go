package api

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/ai-assistant/internal/engine"
)

// Server serves the AI assistant API.
type Server struct {
	engine *engine.Engine
	log    *logging.Logger
	history []QueryEntry
}

// QueryEntry represents a past query and its result.
type QueryEntry struct {
	Question  string `json:"question"`
	Cypher    string `json:"cypher"`
	Rows      int    `json:"rows"`
	Timestamp string `json:"timestamp"`
}

// NewServer creates a new AI assistant API server.
func NewServer(eng *engine.Engine, log *logging.Logger) *Server {
	return &Server{
		engine:  eng,
		log:     log,
		history: make([]QueryEntry, 0),
	}
}

// Handler returns an HTTP handler with AI assistant routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/ai/query", s.Query)
	mux.HandleFunc("GET /api/v1/ai/templates", s.ListTemplates)
	mux.HandleFunc("GET /api/v1/ai/templates/{id}", s.GetTemplate)
	mux.HandleFunc("GET /api/v1/ai/history", s.GetHistory)
	return mux
}

// Query handles POST /api/v1/ai/query
func (s *Server) Query(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Question string `json:"question"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.Question == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "question required"})
		return
	}

	start := time.Now()
	result, err := s.engine.Query(r.Context(), req.Question)
	duration := time.Since(start)

	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{
			"question":     req.Question,
			"error":        err.Error(),
			"suggestions":  s.engine.ListTemplates(),
			"duration_ms":  duration.Milliseconds(),
		})
		return
	}

	// Record history
	s.history = append(s.history, QueryEntry{
		Question:  req.Question,
		Cypher:    result.Cypher,
		Rows:      result.Rows,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
	if len(s.history) > 100 {
		s.history = s.history[1:]
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"question":    req.Question,
		"cypher":      result.Cypher,
		"results":     result.Results,
		"rows":        result.Rows,
		"duration_ms": duration.Milliseconds(),
	})
}

// ListTemplates handles GET /api/v1/ai/templates
func (s *Server) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := s.engine.ListTemplates()
	writeJSON(w, http.StatusOK, map[string]any{
		"templates": templates,
		"total":     len(templates),
	})
}

// GetTemplate handles GET /api/v1/ai/templates/{id}
func (s *Server) GetTemplate(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "template id required"})
		return
	}

	t := s.engine.GetTemplate(id)
	if t == nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "template not found"})
		return
	}

	writeJSON(w, http.StatusOK, t)
}

// GetHistory handles GET /api/v1/ai/history
func (s *Server) GetHistory(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"history": s.history,
		"total":   len(s.history),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
