package metrics

import (
	"fmt"
	"net/http"
	"time"

	"github.com/sentinel-cnapp/sentinel-cnapp/pkg/finding"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Metrics holds all prometheus metric collectors for a service.
type Metrics struct {
	RequestCount      *prometheus.CounterVec
	RequestDuration   *prometheus.HistogramVec
	RequestInFlight   prometheus.Gauge
	FindingsScanned   *prometheus.CounterVec
	FindingsBySource  *prometheus.CounterVec
	FindingsByType    *prometheus.CounterVec
	FindingsBySeverity *prometheus.CounterVec
	ScanDuration      *prometheus.HistogramVec
	ScanTotal         *prometheus.CounterVec
	ScanErrors        *prometheus.CounterVec
	AssetsDiscovered  prometheus.Gauge
	AssetsByProvider  *prometheus.GaugeVec
	AssetsByType      *prometheus.GaugeVec
	GraphNodes        prometheus.Gauge
	GraphEdges        prometheus.Gauge
	MessagesPublished *prometheus.CounterVec
	MessagesConsumed  *prometheus.CounterVec
	QueueLatency      *prometheus.HistogramVec
	OpenFindings      prometheus.Gauge
	CriticalFindings  prometheus.Gauge
	MeanTimeToRemediate prometheus.Gauge
	MemoryUsage       prometheus.Gauge
	Goroutines        prometheus.Gauge
	registry          *prometheus.Registry
}

// New creates a new Metrics collector for the given service.
func New(service string) *Metrics {
	reg := prometheus.NewRegistry()
	ns := "sentinel"

	m := &Metrics{
		RequestCount: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "http_requests_total", Help: "Total number of HTTP requests."},
			[]string{"method", "path", "status"},
		),
		RequestDuration: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{Namespace: ns, Subsystem: service, Name: "http_request_duration_seconds", Help: "HTTP request duration in seconds.", Buckets: []float64{0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10}},
			[]string{"method", "path"},
		),
		RequestInFlight: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "http_requests_in_flight", Help: "Current number of in-flight HTTP requests."},
		),
		FindingsScanned: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "findings_scanned_total", Help: "Total number of findings scanned."},
			[]string{"source"},
		),
		FindingsBySource: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "findings_by_source_total", Help: "Findings by source scanner."},
			[]string{"source"},
		),
		FindingsByType: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "findings_by_type_total", Help: "Findings by type."},
			[]string{"type"},
		),
		FindingsBySeverity: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "findings_by_severity_total", Help: "Findings by severity."},
			[]string{"severity"},
		),
		ScanDuration: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{Namespace: ns, Subsystem: service, Name: "scan_duration_seconds", Help: "Duration of scan operations.", Buckets: []float64{1, 5, 10, 30, 60, 120, 300, 600}},
			[]string{"source", "target_type"},
		),
		ScanTotal: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "scans_total", Help: "Total number of scans performed."},
			[]string{"source", "status"},
		),
		ScanErrors: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "scan_errors_total", Help: "Total number of scan errors."},
			[]string{"source", "error_type"},
		),
		AssetsDiscovered: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "assets_discovered", Help: "Current number of discovered assets."},
		),
		AssetsByProvider: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "assets_by_provider", Help: "Assets grouped by provider."},
			[]string{"provider"},
		),
		AssetsByType: promauto.With(reg).NewGaugeVec(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "assets_by_type", Help: "Assets grouped by type."},
			[]string{"type"},
		),
		GraphNodes: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "graph_nodes", Help: "Current number of nodes in the Neo4j graph."},
		),
		GraphEdges: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "graph_edges", Help: "Current number of edges in the Neo4j graph."},
		),
		MessagesPublished: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "messages_published_total", Help: "Total number of messages published."},
			[]string{"topic"},
		),
		MessagesConsumed: promauto.With(reg).NewCounterVec(
			prometheus.CounterOpts{Namespace: ns, Subsystem: service, Name: "messages_consumed_total", Help: "Total number of messages consumed."},
			[]string{"topic", "consumer_group"},
		),
		QueueLatency: promauto.With(reg).NewHistogramVec(
			prometheus.HistogramOpts{Namespace: ns, Subsystem: service, Name: "queue_latency_seconds", Help: "Message queue latency in seconds.", Buckets: []float64{0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1}},
			[]string{"topic"},
		),
		OpenFindings: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "open_findings", Help: "Current number of open findings."},
		),
		CriticalFindings: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "critical_findings", Help: "Current number of critical findings."},
		),
		MeanTimeToRemediate: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "mean_time_to_remediate_hours", Help: "Mean time to remediate in hours."},
		),
		MemoryUsage: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "memory_usage_bytes", Help: "Current memory usage in bytes."},
		),
		Goroutines: promauto.With(reg).NewGauge(
			prometheus.GaugeOpts{Namespace: ns, Subsystem: service, Name: "goroutines", Help: "Current number of goroutines."},
		),
		registry: reg,
	}

	return m
}

// Handler returns an HTTP handler for Prometheus metrics scraping.
func (m *Metrics) Handler() http.Handler {
	return promhttp.HandlerFor(m.registry, promhttp.HandlerOpts{
		EnableOpenMetrics: true,
	})
}

// ObserveRequest records an HTTP request.
func (m *Metrics) ObserveRequest(method, path string, statusCode int, duration time.Duration) {
	m.RequestCount.WithLabelValues(method, path, fmt.Sprintf("%d", statusCode)).Inc()
	m.RequestDuration.WithLabelValues(method, path).Observe(duration.Seconds())
}

// RecordFinding records a scanned finding.
func (m *Metrics) RecordFinding(source finding.Source, ftype finding.FindingType, severity finding.Severity) {
	m.FindingsScanned.WithLabelValues(string(source)).Inc()
	m.FindingsBySource.WithLabelValues(string(source)).Inc()
	m.FindingsByType.WithLabelValues(string(ftype)).Inc()
	m.FindingsBySeverity.WithLabelValues(string(severity)).Inc()
}

// RecordScan records a completed scan.
func (m *Metrics) RecordScan(source string, status string, duration time.Duration) {
	m.ScanTotal.WithLabelValues(source, status).Inc()
	m.ScanDuration.WithLabelValues(source, "").Observe(duration.Seconds())
}

// Registry returns the underlying prometheus registry.
func (m *Metrics) Registry() *prometheus.Registry {
	return m.registry
}
