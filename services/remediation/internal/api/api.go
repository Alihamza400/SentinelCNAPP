package api

import (
	"encoding/json"
	"net/http"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/remediation/internal/engine"
)

// Server serves the remediation API.
type Server struct {
	engine *engine.Engine
	log    *logging.Logger
}

// NewServer creates a new remediation API server.
func NewServer(eng *engine.Engine, log *logging.Logger) *Server {
	return &Server{engine: eng, log: log}
}

// Handler returns an HTTP handler with remediation routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/remediation/suggest/{finding_id}", s.Suggest)
	mux.HandleFunc("POST /api/v1/remediation/approve/{remediation_id}", s.Approve)
	mux.HandleFunc("GET /api/v1/remediation/pending", s.Pending)
	mux.HandleFunc("GET /api/v1/remediation/finding/{finding_id}", s.ListByFinding)
	mux.HandleFunc("POST /api/v1/remediation/auto-execute", s.AutoExecute)
	return mux
}

// Suggest handles GET /api/v1/remediation/suggest/{finding_id}
func (s *Server) Suggest(w http.ResponseWriter, r *http.Request) {
	findingID := r.PathValue("finding_id")
	if findingID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "finding_id required"})
		return
	}

	remediations, err := s.engine.SuggestRemediation(r.Context(), findingID)
	if err != nil {
		s.log.Error("suggesting remediation", err, "finding_id", findingID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"finding_id":   findingID,
		"remediations": remediations,
		"total":        len(remediations),
	})
}

// Approve handles POST /api/v1/remediation/approve/{remediation_id}
func (s *Server) Approve(w http.ResponseWriter, r *http.Request) {
	remediationID := r.PathValue("remediation_id")
	if remediationID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "remediation_id required"})
		return
	}

	var req struct {
		ApprovedBy string `json:"approved_by"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		req.ApprovedBy = "unknown"
	}
	if req.ApprovedBy == "" {
		req.ApprovedBy = "unknown"
	}

	rem, err := s.engine.ApproveRemediation(r.Context(), remediationID, req.ApprovedBy)
	if err != nil {
		s.log.Error("approving remediation", err, "id", remediationID)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, rem)
}

// Pending handles GET /api/v1/remediation/pending
func (s *Server) Pending(w http.ResponseWriter, r *http.Request) {
	pending := s.engine.ListPending(r.Context())
	writeJSON(w, http.StatusOK, map[string]any{
		"remediations": pending,
		"total":        len(pending),
	})
}

// ListByFinding handles GET /api/v1/remediation/finding/{finding_id}
func (s *Server) ListByFinding(w http.ResponseWriter, r *http.Request) {
	findingID := r.PathValue("finding_id")
	if findingID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "finding_id required"})
		return
	}

	remediations := s.engine.ListByFinding(r.Context(), findingID)
	writeJSON(w, http.StatusOK, map[string]any{
		"finding_id":   findingID,
		"remediations": remediations,
		"total":        len(remediations),
	})
}

// AutoExecute handles POST /api/v1/remediation/auto-execute
func (s *Server) AutoExecute(w http.ResponseWriter, r *http.Request) {
	count, err := s.engine.ExecuteAllAutoRemediations(r.Context())
	if err != nil {
		s.log.Error("auto-executing remediations", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"executed": count,
		"status":   "completed",
	})
}

func writeJSON(w http.ResponseWriter, status int, data any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
