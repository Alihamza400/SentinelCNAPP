// Command dev-gateway is a lightweight reverse proxy for local development.
// It stands in for Envoy and routes /api/v1/* requests to the correct
// SentinelCNAPP microservice based on path prefix (and X-Scanner for scans).
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"time"
)

type route struct {
	prefix string
	target string
}

var routes = []route{
	{"/api/v1/assets", "http://localhost:8080"},       // asset-inventory
	{"/api/v1/findings", "http://localhost:8082"},     // correlation
	{"/api/v1/dashboard", "http://localhost:8082"},    // correlation
	{"/api/v1/graph", "http://localhost:8082"},        // correlation
	{"/api/v1/attack-paths", "http://localhost:8088"}, // attack-path
	{"/api/v1/risk", "http://localhost:8087"},         // risk-engine
	{"/api/v1/remediation", "http://localhost:8090"},  // remediation
	{"/api/v1/ai", "http://localhost:8091"},           // ai-assistant
	{"/api/v1/runtime", "http://localhost:8089"},      // runtime
}

// scannerRoutes maps X-Scanner header values to scanner service ports.
var scannerRoutes = map[string]string{
	"iac":       "http://localhost:8083",
	"container": "http://localhost:8084",
	"k8s":       "http://localhost:8085",
	"secrets":   "http://localhost:8086",
	"":          "http://localhost:8083", // default: IaC scanner
}

type backendHealth struct {
	Name   string `json:"name"`
	URL    string `json:"url"`
	Status string `json:"status"`
}

func main() {
	port := os.Getenv("SENTINEL_GATEWAY_PORT")
	if port == "" {
		port = "8081"
	}

	proxies := map[string]*httputil.ReverseProxy{}

	buildProxy := func(target string) *httputil.ReverseProxy {
		if p, ok := proxies[target]; ok {
			return p
		}
		u, err := url.Parse(target)
		if err != nil {
			log.Fatalf("bad target %q: %v", target, err)
		}
		p := httputil.NewSingleHostReverseProxy(u)
		p.FlushInterval = -1
		// Preserve original host so services that check Host still work.
		p.Director = func(req *http.Request) {
			req.Host = u.Host
			req.URL.Scheme = u.Scheme
			req.URL.Host = u.Host
		}
		proxies[target] = p
		return p
	}

	mux := http.NewServeMux()

	// Aggregate health endpoint for the gateway itself + best-effort backend ping.
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		checks := []backendHealth{}
		for _, rt := range routes {
			checks = append(checks, ping(rt.prefix, rt.target))
		}
		for name, tgt := range scannerRoutes {
			if name == "" {
				continue
			}
			checks = append(checks, ping("scanner-"+name, tgt))
		}
		status := "ok"
		for _, c := range checks {
			if c.Status != "ok" {
				status = "degraded"
				break
			}
		}
		json.NewEncoder(w).Encode(map[string]any{
			"status":   status,
			"service":  "dev-gateway",
			"backends": checks,
		})
	})

	// CORS preflight for everything.
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Scanner, X-GitHub-Event, X-Gitlab-Event, X-Hub-Signature-256")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// Scanner fan-out: /api/v1/scan and /webhook select backend via X-Scanner.
		if strings.HasPrefix(r.URL.Path, "/api/v1/scan") || strings.HasPrefix(r.URL.Path, "/webhook") {
			scanner := r.Header.Get("X-Scanner")
			target, ok := scannerRoutes[scanner]
			if !ok {
				target = scannerRoutes[""]
			}
			buildProxy(target).ServeHTTP(w, r)
			return
		}

		for _, rt := range routes {
			if strings.HasPrefix(r.URL.Path, rt.prefix) {
				buildProxy(rt.target).ServeHTTP(w, r)
				return
			}
		}

		http.Error(w, `{"error":"no route"}`, http.StatusNotFound)
	})

	addr := ":" + port
	log.Printf("dev-gateway listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatal(err)
	}
}

func ping(name, target string) backendHealth {
	client := &http.Client{Timeout: 1 * time.Second}
	resp, err := client.Get(target + "/health")
	if err != nil {
		return backendHealth{Name: name, URL: target, Status: "down"}
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 500 {
		return backendHealth{Name: name, URL: target, Status: fmt.Sprintf("http_%d", resp.StatusCode)}
	}
	return backendHealth{Name: name, URL: target, Status: "ok"}
}
