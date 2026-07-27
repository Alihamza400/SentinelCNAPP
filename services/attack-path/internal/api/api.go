package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/attack-path/internal/engine"
)

// Server serves the attack path API.
type Server struct {
	engine *engine.Engine
	log    *logging.Logger
}

// NewServer creates a new attack path API server.
func NewServer(eng *engine.Engine, log *logging.Logger) *Server {
	return &Server{engine: eng, log: log}
}

// Handler returns an HTTP handler with attack path routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/attack-paths", s.ListAttackPaths)
	mux.HandleFunc("GET /api/v1/attack-paths/summary", s.GetSummary)
	mux.HandleFunc("GET /api/v1/attack-paths/{finding_id}", s.GetPathsForFinding)
	return mux
}

// ListAttackPaths handles GET /api/v1/attack-paths
func (s *Server) ListAttackPaths(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 50)

	paths, err := s.engine.FindAll(r.Context(), limit)
	if err != nil {
		s.log.Error("finding attack paths", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to find attack paths"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"paths": paths,
		"total": len(paths),
	})
}

// GetSummary handles GET /api/v1/attack-paths/summary
func (s *Server) GetSummary(w http.ResponseWriter, r *http.Request) {
	summary, err := s.engine.GetSummary(r.Context())
	if err != nil {
		s.log.Error("getting attack path summary", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get summary"})
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// GetPathsForFinding handles GET /api/v1/attack-paths/{finding_id}
func (s *Server) GetPathsForFinding(w http.ResponseWriter, r *http.Request) {
	findingID := r.PathValue("finding_id")
	if findingID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "finding_id required"})
		return
	}

	paths, err := s.engine.FindFromFinding(r.Context(), findingID)
	if err != nil {
		s.log.Error("finding paths for finding", err, "finding_id", findingID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"finding_id": findingID,
		"paths":      paths,
		"total":      len(paths),
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func queryInt(r *http.Request, key string, defaultVal int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultVal
	}
	i, err := strconv.Atoi(val)
	if err != nil {
		return defaultVal
	}
	return i
}
