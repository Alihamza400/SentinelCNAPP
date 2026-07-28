package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config provides environment-variable-based configuration
// with type-safe accessors and sensible defaults.
type Config struct {
	envs map[string]string
}

// New loads configuration from environment variables.
func New() *Config {
	return &Config{envs: loadEnvs()}
}

// NewWithMap creates a config from a map (for testing).
func NewWithMap(envs map[string]string) *Config {
	return &Config{envs: envs}
}

func loadEnvs() map[string]string {
	envs := make(map[string]string)
	for _, e := range os.Environ() {
		parts := strings.SplitN(e, "=", 2)
		if len(parts) == 2 {
			envs[parts[0]] = parts[1]
		}
	}
	return envs
}

// Get returns the raw value of an env var.
func (c *Config) Get(key string) (string, bool) {
	v, ok := c.envs[key]
	return v, ok
}

// GetDefault returns the value or a default.
func (c *Config) GetDefault(key, defaultVal string) string {
	if v, ok := c.Get(key); ok {
		return v
	}
	return defaultVal
}

// MustGet returns the value or panics.
func (c *Config) MustGet(key string) string {
	if v, ok := c.Get(key); ok {
		return v
	}
	panic(fmt.Sprintf("required config key %q is not set", key))
}

// GetInt returns the value as int with a default.
func (c *Config) GetInt(key string, defaultVal int) int {
	if v, ok := c.Get(key); ok {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// GetFloat returns the value as float64 with a default.
func (c *Config) GetFloat(key string, defaultVal float64) float64 {
	if v, ok := c.Get(key); ok {
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f
		}
	}
	return defaultVal
}

// GetBool returns the value as bool with a default.
func (c *Config) GetBool(key string, defaultVal bool) bool {
	if v, ok := c.Get(key); ok {
		if b, err := strconv.ParseBool(v); err == nil {
			return b
		}
	}
	return defaultVal
}

// GetDuration returns the value as duration with a default.
func (c *Config) GetDuration(key string, defaultVal time.Duration) time.Duration {
	if v, ok := c.Get(key); ok {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

// GetStringSlice returns the value as a comma-separated slice with a default.
func (c *Config) GetStringSlice(key string, defaultVal []string) []string {
	if v, ok := c.Get(key); ok && v != "" {
		parts := strings.Split(v, ",")
		for i := range parts {
			parts[i] = strings.TrimSpace(parts[i])
		}
		return parts
	}
	return defaultVal
}

// Common configuration keys used across services.
const (
	// Service
	ServiceName    = "SENTINEL_SERVICE_NAME"
	ServiceVersion = "SENTINEL_SERVICE_VERSION"
	ServicePort    = "SENTINEL_SERVICE_PORT"

	// Database
	PostgresURL = "SENTINEL_POSTGRES_URL"

	// Neo4j
	Neo4jURI      = "SENTINEL_NEO4J_URI"
	Neo4jUser     = "SENTINEL_NEO4J_USER"
	Neo4jPassword = "SENTINEL_NEO4J_PASSWORD"

	// NATS
	NATSURL    = "SENTINEL_NATS_URL"
	NATSToken  = "SENTINEL_NATS_TOKEN"

	// Redis
	RedisURL      = "SENTINEL_REDIS_URL"
	RedisPassword = "SENTINEL_REDIS_PASSWORD"

	// OpenSearch
	OpenSearchURL      = "SENTINEL_OPENSEARCH_URL"
	OpenSearchUsername = "SENTINEL_OPENSEARCH_USERNAME"
	OpenSearchPassword = "SENTINEL_OPENSEARCH_PASSWORD"

	// Auth
	OIDCIssuerURL = "SENTINEL_OIDC_ISSUER_URL"
	OIDCClientID  = "SENTINEL_OIDC_CLIENT_ID"

	// AWS
	AWSRegion          = "SENTINEL_AWS_REGION"
	AWSAccountID       = "SENTINEL_AWS_ACCOUNT_ID"
	AWSAssumeRoleARN   = "SENTINEL_AWS_ASSUME_ROLE_ARN"
	AWSRegions         = "SENTINEL_AWS_REGIONS"

	// Sync
	SyncInterval = "SENTINEL_SYNC_INTERVAL"

	// Falco
	FalcoWebhookPort = "SENTINEL_FALCO_WEBHOOK_PORT"
	FalcoOutputFile  = "SENTINEL_FALCO_OUTPUT_FILE"

	// LLM (AI Assistant)
	LLMApiKey    = "SENTINEL_LLM_API_KEY"
	LLMEndpoint  = "SENTINEL_LLM_ENDPOINT"

	// Webhook
	WebhookSecret = "SENTINEL_WEBHOOK_SECRET"

	// Logging
	LogLevel  = "SENTINEL_LOG_LEVEL"
	LogFormat = "SENTINEL_LOG_FORMAT"

	// Tracing
	OTLPEndpoint = "SENTINEL_OTLP_ENDPOINT"
)
