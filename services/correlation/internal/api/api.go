package api

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/correlation/internal/cache"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/correlation/internal/graphdb"
)

// Server serves the REST API for the correlation graph.
type Server struct {
	reader *graphdb.Reader
	cache  *cache.Client
	log    *logging.Logger
}

// NewServer creates a new correlation API server.
func NewServer(reader *graphdb.Reader, cache *cache.Client, log *logging.Logger) *Server {
	return &Server{reader: reader, cache: cache, log: log}
}

// Handler returns an HTTP handler with all correlation routes.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/findings", s.ListFindings)
	mux.HandleFunc("GET /api/v1/dashboard/stats", s.DashboardStats)
	mux.HandleFunc("GET /api/v1/dashboard/severity-distribution", s.SeverityDistribution)
	mux.HandleFunc("GET /api/v1/graph", s.GetGraph)
	mux.HandleFunc("GET /api/v1/attack-paths", s.GetAttackPaths)
	return mux
}

// ListFindings handles GET /api/v1/findings
func (s *Server) ListFindings(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	severity := r.URL.Query().Get("severity")
	source := r.URL.Query().Get("source")
	ftype := r.URL.Query().Get("type")
	status := r.URL.Query().Get("status")
	search := r.URL.Query().Get("search")
	page := queryInt(r, "page", 1)
	pageSize := queryInt(r, "page_size", 25)

	result, err := s.reader.ListFindings(r.Context(), severity, source, ftype, status, search, page, pageSize)
	if err != nil {
		s.log.Error("listing findings", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list findings"})
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"findings":  result.Findings,
		"total":     result.Total,
		"page":      page,
		"page_size": pageSize,
	})

	s.log.Info("list findings",
		"total", result.Total,
		"returned", len(result.Findings),
		"duration", time.Since(start))
}

// DashboardStats handles GET /api/v1/dashboard/stats
func (s *Server) DashboardStats(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	// Try cache first
	var stats *graphdb.DashboardStats
	if err := s.cache.Get(r.Context(), cache.KeyDashboardStats, &stats); err == nil {
		writeJSON(w, http.StatusOK, stats)
		return
	}

	var err error
	stats, err = s.reader.GetDashboardStats(r.Context())
	if err != nil {
		s.log.Error("getting dashboard stats", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get stats"})
		return
	}

	// Cache for next request
	if err := s.cache.Set(r.Context(), cache.KeyDashboardStats, stats, cache.TTLDashboardStats); err != nil {
		s.log.Warn("failed to cache dashboard stats", "error", err)
	}

	writeJSON(w, http.StatusOK, stats)
	s.log.Info("dashboard stats", "stats", stats, "duration", time.Since(start))
}

// SeverityDistribution handles GET /api/v1/dashboard/severity-distribution
func (s *Server) SeverityDistribution(w http.ResponseWriter, r *http.Request) {
	dist, err := s.reader.GetSeverityDistribution(r.Context())
	if err != nil {
		s.log.Error("getting severity distribution", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get distribution"})
		return
	}
	writeJSON(w, http.StatusOK, dist)
}

// GetGraph handles GET /api/v1/graph
func (s *Server) GetGraph(w http.ResponseWriter, r *http.Request) {
	start := time.Now()
	limit := queryInt(r, "limit", 100)

	// Try cache
	var data *graphdb.GraphData
	if err := s.cache.Get(r.Context(), cache.KeyGraphData, &data); err == nil {
		writeJSON(w, http.StatusOK, data)
		return
	}

	var err error
	data, err = s.reader.GetGraphData(r.Context(), limit)
	if err != nil {
		s.log.Error("getting graph data", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get graph data"})
		return
	}

	if err := s.cache.Set(r.Context(), cache.KeyGraphData, data, cache.TTLGraphData); err != nil {
		s.log.Warn("failed to cache graph data", "error", err)
	}

	writeJSON(w, http.StatusOK, data)
	s.log.Info("graph data", "nodes", len(data.Nodes), "edges", len(data.Edges), "duration", time.Since(start))
}

// GetAttackPaths handles GET /api/v1/attack-paths
func (s *Server) GetAttackPaths(w http.ResponseWriter, r *http.Request) {
	limit := queryInt(r, "limit", 20)
	paths, err := s.reader.GetAttackPaths(r.Context(), limit)
	if err != nil {
		s.log.Error("getting attack paths", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to get attack paths"})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"paths": paths})
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
