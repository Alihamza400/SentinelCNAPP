package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/logging"
	"github.com/sentinel-cnapp/sentinel-cnapp/services/asset-inventory/internal/store"
)

// HTTPServer serves REST API endpoints for assets.
type HTTPServer struct {
	repo *store.Repository
	log  *logging.Logger
}

// NewHTTPServer creates a new HTTP API server.
func NewHTTPServer(repo *store.Repository, log *logging.Logger) *HTTPServer {
	return &HTTPServer{repo: repo, log: log}
}

// Handler returns an HTTP handler with all asset routes.
func (s *HTTPServer) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/v1/assets", s.ListAssets)
	mux.HandleFunc("GET /api/v1/assets/{id}", s.GetAsset)
	return mux
}

// ListAssets handles GET /api/v1/assets
func (s *HTTPServer) ListAssets(w http.ResponseWriter, r *http.Request) {
	start := time.Now()

	filter := store.AssetFilter{
		Provider:    r.URL.Query().Get("provider"),
		AssetType:   r.URL.Query().Get("type"),
		Region:      r.URL.Query().Get("region"),
		Environment: r.URL.Query().Get("environment"),
		Search:      r.URL.Query().Get("search"),
		Page:        queryInt(r, "page", 1),
		PageSize:    queryInt(r, "page_size", store.DefaultPageSize),
	}

	activeStr := r.URL.Query().Get("active")
	if activeStr != "" {
		active := activeStr == "true"
		filter.Active = &active
	}

	assets, total, err := s.repo.ListAssets(r.Context(), filter)
	if err != nil {
		s.log.Error("listing assets", err)
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "failed to list assets"})
		return
	}

	if assets == nil {
		assets = []store.Asset{}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"assets":    assets,
		"total":     total,
		"page":      filter.Page,
		"page_size": filter.PageSize,
	})

	s.log.Info("listed assets",
		"count", len(assets),
		"total", total,
		"duration", time.Since(start))
}

// GetAsset handles GET /api/v1/assets/{id}
func (s *HTTPServer) GetAsset(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "asset id required"})
		return
	}

	asset, err := s.repo.GetAsset(r.Context(), id)
	if err != nil {
		s.log.Error("getting asset", err, "id", id)
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "asset not found"})
		return
	}

	writeJSON(w, http.StatusOK, asset)
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

