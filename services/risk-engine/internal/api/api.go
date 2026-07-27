package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/risk-engine/internal/engine"
)

// Server serves the risk engine API.
type Server struct {
	engine *engine.Engine
	log    *logging.Logger
}

// NewServer creates a new risk engine API server.
func NewServer(eng *engine.Engine, log *logging.Logger) *Server {
	return &Server{engine: eng, log: log}
}

// Handler returns an HTTP handler with risk engine routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/risk/evaluate", s.EvaluateFinding)
	mux.HandleFunc("POST /api/v1/risk/evaluate-all", s.EvaluateAll)
	mux.HandleFunc("GET /api/v1/risk/{finding_id}", s.GetRisk)
	return mux
}

// EvaluateFinding handles POST /api/v1/risk/evaluate
func (s *Server) EvaluateFinding(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	var req struct {
		FindingID string `json:"finding_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if req.FindingID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "finding_id required"})
		return
	}

	eval, err := s.engine.Evaluate(r.Context(), req.FindingID)
	if err != nil {
		s.log.Error("risk evaluation failed", err, "finding_id", req.FindingID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, eval)
	s.log.Info("risk evaluated via API",
		"finding_id", req.FindingID,
		"score", eval.OverallScore,
		"duration", time.Since(start))
}

// EvaluateAll handles POST /api/v1/risk/evaluate-all
func (s *Server) EvaluateAll(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	evals, err := s.engine.EvaluateAll(r.Context())
	if err != nil {
		s.log.Error("batch risk evaluation failed", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"evaluations": evals,
		"total":       len(evals),
	})
	s.log.Info("batch risk evaluation complete",
		"evaluated", len(evals),
		"duration", time.Since(start))
}

// GetRisk handles GET /api/v1/risk/{finding_id}
func (s *Server) GetRisk(w http.ResponseWriter, r *http.Request) {
	findingID := r.PathValue("finding_id")
	if findingID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "finding_id required"})
		return
	}

	eval, err := s.engine.Evaluate(r.Context(), findingID)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "finding not found"})
		return
	}

	writeJSON(w, http.StatusOK, eval)
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
